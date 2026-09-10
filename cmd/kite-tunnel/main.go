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
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/KiteXRay/desktop/internal/bridge"
	"github.com/KiteXRay/desktop/internal/osspecific/clean"
	"github.com/KiteXRay/desktop/internal/osspecific/root"
	"github.com/amnezia-vpn/amneziawg-go/conn"
	"github.com/amnezia-vpn/amneziawg-go/device"
	awgtun "github.com/amnezia-vpn/amneziawg-go/tun"
	"github.com/goxray/core/awg"
	"github.com/goxray/core/network/route"
	"github.com/goxray/core/network/tun"
	"github.com/goxray/core/pipe2socks"
	"github.com/goxray/core/wireguard"
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
		_ = os.Stdout.Sync()
	}
	if ev.Event == "error" {
		fmt.Fprintf(os.Stderr, "kite-tunnel error: %s\n", ev.Message)
		_ = os.Stderr.Sync()
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

type meteredAWGTun struct {
	awgtun.Device
	read    *atomic.Int64
	written *atomic.Int64
}

func (m *meteredAWGTun) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	n, err := m.Device.Read(bufs, sizes, offset)
	if n > 0 {
		var total int64
		for i := 0; i < n && i < len(sizes); i++ {
			total += int64(sizes[i])
		}
		m.read.Add(total)
	}
	return n, err
}

func (m *meteredAWGTun) Write(bufs [][]byte, offset int) (int, error) {
	n, err := m.Device.Write(bufs, offset)
	if n > 0 {
		m.written.Add(int64(n))
	}
	return n, err
}

func parseBypassIPs(bypassIP string, bypassIPs string) []string {
	seen := make(map[string]bool)
	var result []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		ipStr := s
		if idx := strings.Index(s, "/"); idx != -1 {
			ipStr = s[:idx]
		}
		if ip := net.ParseIP(ipStr); ip != nil {
			if ip4 := ip.To4(); ip4 != nil {
				canonical := ip4.String()
				if !seen[canonical] {
					seen[canonical] = true
					result = append(result, canonical)
				}
			}
		}
	}
	if bypassIP != "" {
		add(bypassIP)
	}
	if bypassIPs != "" {
		for _, p := range strings.Split(bypassIPs, ",") {
			add(p)
		}
	}
	return result
}

func setupBypassRoutes(routeManager *route.Route, ips []string, gw net.IP) []*route.Opts {
	var opts []*route.Opts
	if gw == nil || gw.To4() == nil || len(ips) == 0 {
		return opts
	}
	for _, ipStr := range ips {
		addr, err := route.ParseAddr(ipStr)
		if err != nil {
			slog.Warn("skipping invalid bypass ip", "ip", ipStr, "err", err)
			continue
		}
		bOpts := &route.Opts{
			Gateway: gw,
			Routes:  []*route.Addr{addr},
		}
		_ = routeManager.Delete(*bOpts)
		if err := routeManager.Add(*bOpts); err != nil {
			slog.Error("failed to add bypass route", "ip", ipStr, "gw", gw, "err", err)
		} else {
			opts = append(opts, bOpts)
		}
	}
	return opts
}

func cleanupBypassRoutes(routeManager *route.Route, opts []*route.Opts) {
	for _, bOpts := range opts {
		if bOpts != nil {
			_ = routeManager.Delete(*bOpts)
		}
	}
}

func main() {
	checkFlag := flag.Bool("check", false, "check network privileges and exit (0 = ok, 1 = missing)")
	cleanFlag := flag.Bool("clean", false, "clean stuck TUN devices and routes and exit")
	fixPerms := flag.String("fix-perms", "", "recursively fix ownership of path to calling user UID/GID")
	engine := flag.String("engine", "pipe2socks", "tunnel engine: pipe2socks, awg")
	awgLink := flag.String("awg-link", "", "WireGuard / AmneziaWG configuration URI link")
	socks5 := flag.String("socks5", "127.0.0.1:10808", "local SOCKS5 proxy address")
	tunName := flag.String("tun-name", "", "virtual TUN interface name (empty for OS default)")
	tunAddr := flag.String("tun-addr", "192.18.0.1/24", "TUN interface IPv4 CIDR")
	tunGw := flag.String("tun-gw", "192.18.0.1", "TUN interface peer gateway IPv4")
	tunDNS := flag.String("tun-dns", "8.8.8.8", "TUN interface DNS")
	routesStr := flag.String("routes", "0.0.0.0/1,128.0.0.0/1", "comma-separated CIDR routes to route into TUN")
	bypassIP := flag.String("bypass-ip", "", "remote server IP to route directly through physical gateway")
	bypassIPs := flag.String("bypass-ips", "", "comma-separated remote server IPs to route directly through physical gateway")
	gatewayIP := flag.String("gateway-ip", "", "default gateway IP for bypass route")
	mode := flag.String("mode", "tunnel", "tunnel mode (tunnel, system, bridge, per_app)")
	configPath := flag.String("config", "", "path to connections.json config file for bridge mode")
	flag.Parse()

	// Fix user permissions mode (runs as setuid root to repair root-owned config files)
	if *fixPerms != "" {
		uid := os.Getuid()
		gid := os.Getgid()
		if uid != 0 {
			_ = filepath.Walk(*fixPerms, func(path string, info os.FileInfo, err error) error {
				if err == nil {
					_ = os.Chown(path, uid, gid)
					if info.IsDir() {
						_ = os.Chmod(path, 0755)
					} else {
						_ = os.Chmod(path, 0644)
					}
				}
				return nil
			})
		}
		os.Exit(0)
	}

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

	if *engine == "awg" {
		if *mode == "bridge" {
			emit(Event{Event: "error", Message: "bridge mode is not supported with native awg engine"})
			os.Exit(1)
		}
		runAWG(ctx, *tunName, *tunAddr, *tunGw, *tunDNS, *routesStr, *bypassIP, *bypassIPs, *gatewayIP, *awgLink)
		return
	}

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

	var gw net.IP
	if *gatewayIP != "" {
		gw = net.ParseIP(*gatewayIP)
	}
	if gw == nil {
		if g, err := gateway.DiscoverGateway(); err == nil && g != nil && !g.IsUnspecified() {
			gw = g
		}
	}

	allBypassIPs := parseBypassIPs(*bypassIP, *bypassIPs)
	bypassOptsList := setupBypassRoutes(routeManager, allBypassIPs, gw)

	var cleanupBypass func()
	if *mode == "bridge" {
		var err error
		cleanupBypass, err = bridge.SetupBridgeBypass(gw)
		if err != nil {
			emit(Event{Event: "error", Message: fmt.Sprintf("setup bridge bypass: %v", err)})
			os.Exit(1)
		}
		defer cleanupBypass()
	}

	// Parse routes to TUN
	var routesToTUN []*route.Addr
	for _, r := range strings.Split(*routesStr, ",") {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		addr, err := route.ParseAddr(r)
		if err != nil {
			slog.Warn("skipping invalid tun route", "route", r, "err", err)
			continue
		}
		if addr != nil {
			routesToTUN = append(routesToTUN, addr)
		}
	}

	tunOpts := route.Opts{
		IfName: ifc.Name(),
		Routes: routesToTUN,
	}

	if err := routeManager.Add(tunOpts); err != nil {
		cleanupBypassRoutes(routeManager, bypassOptsList)
		emit(Event{Event: "error", Message: fmt.Sprintf("add TUN routes: %v", err)})
		os.Exit(1)
	}

	// Immediate route cleanup on context cancellation
	var cleanOnce sync.Once
	cleanup := func() {
		cleanOnce.Do(func() {
			if cleanupBypass != nil {
				cleanupBypass()
			}
			_ = routeManager.Delete(tunOpts)
			cleanupBypassRoutes(routeManager, bypassOptsList)
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

	var errPipe error
	if *mode == "bridge" {
		watcher := bridge.NewConfigWatcher(*configPath)
		bd := bridge.NewBridgeDialer(*socks5, watcher.GetRules, slog.Default(), watcher.GetGroups)
		errPipe = pipe.CopyWithDialer(ctx, metered, bd)
	} else {
		errPipe = pipe.Copy(ctx, metered, *socks5)
	}
	if errPipe != nil && errPipe != context.Canceled {
		slog.Error("pipe2socks stopped with error", "err", errPipe)
	}

	emit(Event{
		Event:    "stopped",
		BytesIn:  bytesRead.Load(),
		BytesOut: bytesWritten.Load(),
	})
}

func runAWG(ctx context.Context, tunName string, tunAddr string, tunGw string, tunDNS string, routesStr string, bypassIP string, bypassIPs string, gatewayIP string, awgLink string) {
	awgCfg, _, err := wireguard.ParseLink(awgLink)
	if err != nil {
		emit(Event{Event: "error", Message: fmt.Sprintf("parse awg link: %v", err)})
		os.Exit(1)
	}

	mtu := awgCfg.MTU
	if mtu <= 0 {
		mtu = 1420
	}

	if tunName == "" {
		tunName = "kite0"
	}

	tunDev, err := awgtun.CreateTUN(tunName, mtu)
	if err != nil {
		emit(Event{Event: "error", Message: fmt.Sprintf("create native TUN device %s: %v", tunName, err)})
		os.Exit(1)
	}

	actualName, err := tunDev.Name()
	if err != nil || actualName == "" {
		actualName = tunName
	}

	addrCIDR := tunAddr
	if awgCfg.Address != "" {
		for _, part := range strings.Split(awgCfg.Address, ",") {
			part = strings.TrimSpace(part)
			if !strings.Contains(part, ":") && part != "" {
				addrCIDR = part
				break
			}
		}
	}
	if !strings.Contains(addrCIDR, "/") {
		addrCIDR += "/32"
	}

	tunIP, tunNet, err := net.ParseCIDR(addrCIDR)
	if err != nil {
		_ = tunDev.Close()
		emit(Event{Event: "error", Message: fmt.Sprintf("invalid tun address %q: %v", addrCIDR, err)})
		os.Exit(1)
	}
	tunNet.IP = tunIP

	peerGw := net.ParseIP(tunGw)
	if peerGw == nil || awgCfg.Address != "" {
		peerGw = tunIP
	}

	if err := tun.UpInterface(actualName, tunNet, peerGw); err != nil {
		_ = tunDev.Close()
		emit(Event{Event: "error", Message: fmt.Sprintf("bring up interface %s: %v", actualName, err)})
		os.Exit(1)
	}

	routeManager, err := route.New()
	if err != nil {
		_ = tunDev.Close()
		emit(Event{Event: "error", Message: fmt.Sprintf("init route manager: %v", err)})
		os.Exit(1)
	}

	endpointIP := ""
	endpointHost := awgCfg.Endpoint
	if h, _, err := net.SplitHostPort(awgCfg.Endpoint); err == nil {
		endpointHost = h
	}
	if ip, err := net.ResolveIPAddr("ip", endpointHost); err == nil && ip.IP.To4() != nil {
		endpointIP = ip.IP.To4().String()
	}

	allBypassIPs := parseBypassIPs(bypassIP, bypassIPs)
	if endpointIP != "" {
		var found bool
		for _, ip := range allBypassIPs {
			if ip == endpointIP {
				found = true
				break
			}
		}
		if !found {
			allBypassIPs = append(allBypassIPs, endpointIP)
		}
	}

	var gw net.IP
	if gatewayIP != "" {
		gw = net.ParseIP(gatewayIP)
	}
	if gw == nil {
		if g, err := gateway.DiscoverGateway(); err == nil && g != nil && !g.IsUnspecified() {
			gw = g
		}
	}

	bypassOptsList := setupBypassRoutes(routeManager, allBypassIPs, gw)

	var routesToTUN []*route.Addr
	for _, r := range strings.Split(routesStr, ",") {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		addr, err := route.ParseAddr(r)
		if err != nil {
			slog.Warn("skipping invalid awg tun route", "route", r, "err", err)
			continue
		}
		if addr != nil {
			routesToTUN = append(routesToTUN, addr)
		}
	}

	tunOpts := route.Opts{
		IfName: actualName,
		Routes: routesToTUN,
	}

	if err := routeManager.Add(tunOpts); err != nil {
		cleanupBypassRoutes(routeManager, bypassOptsList)
		_ = tunDev.Close()
		emit(Event{Event: "error", Message: fmt.Sprintf("add TUN routes: %v", err)})
		os.Exit(1)
	}

	var bytesRead, bytesWritten atomic.Int64
	metered := &meteredAWGTun{
		Device:  tunDev,
		read:    &bytesRead,
		written: &bytesWritten,
	}

	dev := device.NewDevice(metered, conn.NewDefaultBind(), device.NewLogger(device.LogLevelVerbose, "awg: "))

	ipcStr, err := awg.BuildIPCConfig(awgCfg)
	if err != nil {
		cleanupBypassRoutes(routeManager, bypassOptsList)
		_ = routeManager.Delete(tunOpts)
		dev.Close()
		emit(Event{Event: "error", Message: fmt.Sprintf("build awg ipc config: %v", err)})
		os.Exit(1)
	}

	if err := dev.IpcSet(ipcStr); err != nil {
		cleanupBypassRoutes(routeManager, bypassOptsList)
		_ = routeManager.Delete(tunOpts)
		dev.Close()
		emit(Event{Event: "error", Message: fmt.Sprintf("set awg ipc: %v", err)})
		os.Exit(1)
	}

	if err := dev.Up(); err != nil {
		cleanupBypassRoutes(routeManager, bypassOptsList)
		_ = routeManager.Delete(tunOpts)
		dev.Close()
		emit(Event{Event: "error", Message: fmt.Sprintf("awg dev up: %v", err)})
		os.Exit(1)
	}

	var cleanOnce sync.Once
	cleanup := func() {
		cleanOnce.Do(func() {
			_ = routeManager.Delete(tunOpts)
			cleanupBypassRoutes(routeManager, bypassOptsList)
			dev.Close()
		})
	}
	defer cleanup()

	go func() {
		<-ctx.Done()
		cleanup()
	}()

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

	emit(Event{
		Event:     "ready",
		Interface: actualName,
	})

	<-ctx.Done()

	emit(Event{
		Event:    "stopped",
		BytesIn:  bytesRead.Load(),
		BytesOut: bytesWritten.Load(),
	})
}
