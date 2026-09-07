package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/KiteXRay/desktop/internal/osspecific/clean"
	"github.com/KiteXRay/desktop/internal/osspecific/root"
	"github.com/goxray/core/network/route"
	"github.com/goxray/core/network/tun"
	"github.com/goxray/core/pipe2socks"
	"github.com/jackpal/gateway"
)

type Event struct {
	Event     string `json:"event"`
	Interface string `json:"interface,omitempty"`
	BytesIn   int64  `json:"bytes_in,omitempty"`
	BytesOut  int64  `json:"bytes_out,omitempty"`
	Message   string `json:"message,omitempty"`
}

func emit(ev Event) {
	data, err := json.Marshal(ev)
	if err == nil {
		fmt.Fprintf(os.Stdout, "%s\n", string(data))
	}
}

type meteredTunnel struct {
	io.ReadWriteCloser
	read    *atomic.Int64
	written *atomic.Int64
}

func (m *meteredTunnel) Read(p []byte) (int, error) {
	n, err := m.ReadWriteCloser.Read(p)
	if n > 0 {
		m.read.Add(int64(n))
	}
	return n, err
}

func (m *meteredTunnel) Write(p []byte) (int, error) {
	n, err := m.ReadWriteCloser.Write(p)
	if n > 0 {
		m.written.Add(int64(n))
	}
	return n, err
}

func main() {
	checkFlag := flag.Bool("check", false, "check network privileges and exit (0 = ok, 1 = missing)")
	cleanFlag := flag.Bool("clean", false, "clean stuck TUN devices and routes and exit")
	socks5 := flag.String("socks5", "127.0.0.1:10808", "local SOCKS5 proxy address")
	tunName := flag.String("tun-name", "", "virtual TUN interface name (empty for OS default)")
	tunAddr := flag.String("tun-addr", "192.18.0.1/24", "TUN interface IPv4 CIDR")
	tunGw := flag.String("tun-gw", "192.18.0.1", "TUN interface peer gateway IPv4")
	tunDNS := flag.String("tun-dns", "8.8.8.8", "TUN interface DNS")
	routesStr := flag.String("routes", "0.0.0.0/1,128.0.0.0/1", "comma-separated CIDR routes to route into TUN")
	bypassIP := flag.String("bypass-ip", "", "remote server IP to route directly through physical gateway")
	gatewayIP := flag.String("gateway-ip", "", "default gateway IP for bypass route")
	mode := flag.String("mode", "tunnel", "tunnel mode (tunnel, system, bridge, per_app)")
	flag.Parse()

	// Privilege check mode
	if *checkFlag {
		hasPrivs, err := root.CheckProcessCapabilities()
		if hasPrivs {
			os.Exit(0)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "privilege check failed: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "privilege check failed: missing network capabilities\n")
		}
		os.Exit(1)
	}

	// Clean stuck routes / interfaces mode
	if *cleanFlag {
		if err := clean.ClearStuckNetwork(); err != nil {
			fmt.Fprintf(os.Stderr, "clean failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals (SIGINT, SIGTERM)
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Monitor stdin for EOF (if parent closes pipe or terminates)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			_, err := reader.ReadByte()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// 1. Create TUN interface
	ifc, err := tun.New(*tunName, 1500)
	if err != nil {
		emit(Event{Event: "error", Message: fmt.Sprintf("create TUN device: %v", err)})
		os.Exit(1)
	}
	defer ifc.Close()

	// 2. Configure TUN interface IP
	tunIP, tunNet, err := net.ParseCIDR(*tunAddr)
	if err != nil {
		emit(Event{Event: "error", Message: fmt.Sprintf("invalid tun-addr CIDR %q: %v", *tunAddr, err)})
		os.Exit(1)
	}
	tunNet.IP = tunIP

	peerGw := net.ParseIP(*tunGw)
	if peerGw == nil {
		peerGw = tunIP
	}

	if err := ifc.Up(tunNet, peerGw); err != nil {
		emit(Event{Event: "error", Message: fmt.Sprintf("bring up TUN interface %s: %v", ifc.Name(), err)})
		os.Exit(1)
	}

	if *tunDNS != "" {
		if err := ifc.SetDNS(*tunDNS); err != nil {
			slog.Warn("failed to set TUN DNS", "dns", *tunDNS, "err", err)
		}
	}

	// 3. Configure routing table
	routeManager, err := route.New()
	if err != nil {
		emit(Event{Event: "error", Message: fmt.Sprintf("init route manager: %v", err)})
		os.Exit(1)
	}

	// Add bypass route for XRay remote server IP so traffic doesn't loop
	var bypassOpts *route.Opts
	if *bypassIP != "" {
		var gw net.IP
		if *gatewayIP != "" {
			gw = net.ParseIP(*gatewayIP)
		}
		if gw == nil {
			if g, err := gateway.DiscoverGateway(); err == nil && g != nil && !g.IsUnspecified() {
				gw = g
			}
		}
		if gw != nil {
			bOpts := route.Opts{
				Gateway: gw,
				Routes:  []*route.Addr{route.MustParseAddr(*bypassIP + "/32")},
			}
			_ = routeManager.Delete(bOpts)
			if err := routeManager.Add(bOpts); err != nil {
				slog.Error("failed to add bypass route for xray server", "ip", *bypassIP, "gw", gw, "err", err)
			} else {
				bypassOpts = &bOpts
			}
		}
	}

	// Parse routes to TUN
	var routesToTUN []*route.Addr
	for _, r := range strings.Split(*routesStr, ",") {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		addr := route.MustParseAddr(r)
		if addr != nil {
			routesToTUN = append(routesToTUN, addr)
		}
	}

	tunOpts := route.Opts{
		IfName: ifc.Name(),
		Routes: routesToTUN,
	}

	if err := routeManager.Add(tunOpts); err != nil {
		if bypassOpts != nil {
			_ = routeManager.Delete(*bypassOpts)
		}
		emit(Event{Event: "error", Message: fmt.Sprintf("add TUN routes: %v", err)})
		os.Exit(1)
	}

	// Immediate route cleanup on context cancellation
	var cleanOnce sync.Once
	cleanup := func() {
		cleanOnce.Do(func() {
			_ = routeManager.Delete(tunOpts)
			if bypassOpts != nil {
				_ = routeManager.Delete(*bypassOpts)
			}
			_ = ifc.Close()
		})
	}
	defer cleanup()

	go func() {
		<-ctx.Done()
		cleanup()
	}()

	// 4. Wrap with traffic metering
	var bytesRead, bytesWritten atomic.Int64
	metered := &meteredTunnel{
		ReadWriteCloser: ifc,
		read:            &bytesRead,
		written:         &bytesWritten,
	}

	// Periodic stats reporter
	statsTicker := time.NewTicker(1 * time.Second)
	defer statsTicker.Stop()
	go func() {
		for {
			select {
			case <-statsTicker.C:
				emit(Event{
					Event:    "stats",
					BytesIn:  bytesRead.Load(),
					BytesOut: bytesWritten.Load(),
				})
			case <-ctx.Done():
				return
			}
		}
	}()

	// Emit ready event
	emit(Event{
		Event:     "ready",
		Interface: ifc.Name(),
	})

	// 5. Run pipe2socks
	pipe, err := pipe2socks.NewPipe(pipe2socks.DefaultOpts)
	if err != nil {
		emit(Event{Event: "error", Message: fmt.Sprintf("init pipe2socks: %v", err)})
		os.Exit(1)
	}

	_ = mode // reserved for future per-app or bridge customizations
	errPipe := pipe.Copy(ctx, metered, *socks5)
	if errPipe != nil && errPipe != context.Canceled {
		slog.Error("pipe2socks stopped with error", "err", errPipe)
	}

	emit(Event{
		Event:    "stopped",
		BytesIn:  bytesRead.Load(),
		BytesOut: bytesWritten.Load(),
	})
}
