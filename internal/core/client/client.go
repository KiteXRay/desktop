package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/goxray/core/awg"
	"github.com/goxray/core/wireguard"
	"github.com/goxray/core/network/route"
	"github.com/goxray/core/network/tun"
	"github.com/goxray/core/pipe2socks"
	"github.com/jackpal/gateway"
	xrayproto "github.com/lilendian0x00/xray-knife/v3/pkg/protocol"
	"github.com/lilendian0x00/xray-knife/v3/pkg/xray"
	xapplog "github.com/xtls/xray-core/app/log"
	xcommlog "github.com/xtls/xray-core/common/log"
	"github.com/xjasonlyu/tun2socks/v2/dialer"
	tproxy "github.com/xjasonlyu/tun2socks/v2/proxy"
	"golang.org/x/net/proxy"
)

type TunnelMode string

const (
	TunnelModeTunnel TunnelMode = "tunnel"
	TunnelModeProxy  TunnelMode = "proxy"
	TunnelModeBridge TunnelMode = "bridge"

	// Backward compatibility
	TunnelModeSystem TunnelMode = "system"
	TunnelModePerApp TunnelMode = "per_app"

	DefaultSocksPort = 10808
	DefaultHTTPPort  = 10809

	disconnectTimeout = 30 * time.Second
)

var (
	defaultTUNAddress = &net.IPNet{IP: net.IPv4(192, 18, 0, 1), Mask: net.IPv4Mask(255, 255, 255, 0)}

	DefaultRoutesToTUN = []*route.Addr{
		route.MustParseAddr("0.0.0.0/1"),
		route.MustParseAddr("128.0.0.0/1"),
	}

	// TelegramRoutes covers all CIDR blocks assigned to Telegram data centers (AS44907, AS62041).
	TelegramRoutes = []*route.Addr{
		route.MustParseAddr("91.108.4.0/22"),
		route.MustParseAddr("91.108.8.0/22"),
		route.MustParseAddr("91.108.12.0/22"),
		route.MustParseAddr("91.108.16.0/22"),
		route.MustParseAddr("91.108.20.0/22"),
		route.MustParseAddr("91.108.56.0/22"),
		route.MustParseAddr("149.154.160.0/20"),
		route.MustParseAddr("185.76.151.0/24"),
	}
)

type Proxy struct {
	IP   net.IP
	Port int
}

func (p *Proxy) String() string {
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

type Config struct {
	GatewayIP        *net.IP
	InboundProxy     *Proxy
	TUNAddress       *net.IPNet
	RoutesToTUN      []*route.Addr
	TLSAllowInsecure bool
	Logger           *slog.Logger
	XRayLogType      xapplog.LogType
	Mode             TunnelMode
	SocksPort        int
	HTTPPort         int
	TunnelDeviceIP      string
	TunnelDNS           string
	TunnelBinaryPath    string
	BypassIPs           []string
	BridgeDialerFactory func(defaultSocksAddr string) tproxy.Dialer
	ConfigPath          string
}

type Client struct {
	cfg Config

	mode TunnelMode

	bridgeDialerFactory func(defaultSocksAddr string) tproxy.Dialer

	xInst  xrayproto.Instance
	xCfg   *xrayproto.GeneralConfig
	xSrvIP *net.IPAddr

	awgEngine     *awg.Engine
	isAWG         bool
	awgLink       string
	directSocksLn net.Listener

	tunnel      io.ReadWriteCloser
	tunnelCmd   *exec.Cmd
	tunnelStdin io.WriteCloser
	pipe        *pipe2socks.Pipe
	routes      *route.Route

	tunnelStopped chan error
	stopTunnel    func()

	// Public HTTP proxy listener
	httpLn  net.Listener
	proxyWg sync.WaitGroup

	// Active TUN interface name
	ifName string

	// Traffic counters (bytes)
	bytesRead    atomic.Int64
	bytesWritten atomic.Int64
}

func NewClient() (*Client, error) {
	gatewayIP, err := gateway.DiscoverGateway()
	if err != nil {
		gatewayIP = net.IPv4(192, 168, 1, 1)
	}

	p, err := pipe2socks.NewPipe(pipe2socks.DefaultOpts)
	if err != nil {
		return nil, fmt.Errorf("tun2socks new pipe: %w", err)
	}

	r, err := route.New()
	if err != nil {
		return nil, fmt.Errorf("route new: %w", err)
	}

	internalPort := getFreePort()

	return &Client{
		cfg: Config{
			GatewayIP: &gatewayIP,
			InboundProxy: &Proxy{
				IP:   net.IPv4(127, 0, 0, 1),
				Port: internalPort,
			},
			TUNAddress:     defaultTUNAddress,
			RoutesToTUN:    DefaultRoutesToTUN,
			Logger:         slog.New(slog.NewTextHandler(os.Stdout, nil)),
			Mode:           TunnelModeTunnel,
			SocksPort:      DefaultSocksPort,
			HTTPPort:       DefaultHTTPPort,
			TunnelDeviceIP: "192.18.0.1",
			TunnelDNS:      "8.8.8.8",
		},
		mode:          TunnelModeTunnel,
		tunnelStopped: make(chan error, 1),
		pipe:          p,
		routes:        r,
	}, nil
}

func NewClientWithOpts(cfg Config) (*Client, error) {
	c, err := NewClient()
	if err != nil {
		return nil, err
	}
	if cfg.GatewayIP != nil {
		c.cfg.GatewayIP = cfg.GatewayIP
	}
	if cfg.Logger != nil {
		c.cfg.Logger = cfg.Logger
	}
	if cfg.Mode != "" {
		c.cfg.Mode = cfg.Mode
		c.mode = cfg.Mode
	}
	if cfg.TunnelDeviceIP != "" || cfg.TunnelDNS != "" {
		c.SetTunnelSettings(cfg.TunnelDeviceIP, cfg.TunnelDNS)
	}
	if cfg.BridgeDialerFactory != nil {
		c.bridgeDialerFactory = cfg.BridgeDialerFactory
	}
	return c, nil
}

func (c *Client) Mode() TunnelMode {
	return c.mode
}

func (c *Client) SetMode(mode TunnelMode) {
	c.mode = mode
	c.cfg.Mode = mode
}

func (c *Client) SetBridgeDialerFactory(fn func(defaultSocksAddr string) tproxy.Dialer) {
	c.bridgeDialerFactory = fn
}

func (c *Client) SetConfigPath(path string) {
	c.cfg.ConfigPath = path
}

func (c *Client) SetTunnelSettings(deviceIP, dns string) {
	if deviceIP != "" {
		c.cfg.TunnelDeviceIP = deviceIP
		if parsed := net.ParseIP(deviceIP); parsed != nil {
			c.cfg.TUNAddress = &net.IPNet{
				IP:   parsed.To4(),
				Mask: net.IPv4Mask(255, 255, 255, 0),
			}
		}
	}
	if dns != "" {
		c.cfg.TunnelDNS = dns
	}
}

func (c *Client) BytesRead() int {
	awgBytes := int64(0)
	if c.awgEngine != nil {
		awgBytes = c.awgEngine.BytesRead()
	}
	return int(c.bytesRead.Load() + awgBytes)
}

func (c *Client) BytesWritten() int {
	awgBytes := int64(0)
	if c.awgEngine != nil {
		awgBytes = c.awgEngine.BytesWritten()
	}
	return int(c.bytesWritten.Load() + awgBytes)
}

func (c *Client) Connect(link string) error {
	return c.ConnectWithMode(link, c.mode)
}

func (c *Client) ConnectWithMode(link string, mode TunnelMode) error {
	c.mode = mode
	c.cfg.Mode = mode
	c.cfg.Logger.Info("Connecting client", "mode", string(mode))

	// Dynamically refresh default gateway in case network changed or resumed from sleep
	if gw, err := gateway.DiscoverGateway(); err == nil && gw != nil && !gw.IsUnspecified() {
		c.cfg.GatewayIP = &gw
	}

	// Ensure XRay / AWG listens directly on the standard SOCKS5 port (127.0.0.1:10808)
	c.cfg.InboundProxy.IP = net.IPv4(127, 0, 0, 1)
	c.cfg.InboundProxy.Port = c.cfg.SocksPort

	if wireguard.IsAWGLink(link) {
		awgCfg, _, err := wireguard.ParseLink(link)
		if err != nil {
			return fmt.Errorf("parse awg link: %w", err)
		}

		endpointHost := awgCfg.Endpoint
		if h, _, err := net.SplitHostPort(awgCfg.Endpoint); err == nil {
			endpointHost = h
		}
		ip, err := net.ResolveIPAddr("ip", endpointHost)
		if err != nil {
			return fmt.Errorf("awg endpoint not resolvable: %w", err)
		}
		c.xSrvIP = ip
		c.isAWG = true
		c.awgLink = link

		helperBin := c.cfg.TunnelBinaryPath
		if helperBin == "" {
			helperBin = findTunnelBinary()
		}
		// If running in Proxy-only mode, Bridge mode, or helper binary is unavailable, use in-process netstack engine
		if mode == TunnelModeProxy || mode == TunnelModeBridge || helperBin == "" {
			c.awgEngine = awg.NewEngine()
			if err := c.awgEngine.Start(awgCfg, c.cfg.SocksPort); err != nil {
				return fmt.Errorf("start awg engine: %w", err)
			}
		}
	} else {
		var err error
		c.xInst, c.xCfg, err = c.createXrayProxy(link)
		if err != nil {
			return fmt.Errorf("create xray core instance: %w", err)
		}

		if err = c.xInst.Start(); err != nil {
			return fmt.Errorf("start xray core instance: %w", err)
		}
	}
	time.Sleep(120 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	c.stopTunnel = cancel

	// If native AWG TUN is active (no in-process engine), start direct SOCKS5 listener
	// on 127.0.0.1:10808 to satisfy watchdog and HTTP forwarder (10809).
	if c.isAWG && c.awgEngine == nil {
		if err := c.startDirectSocksServer(ctx); err != nil {
			c.cfg.Logger.Warn("could not start direct socks server", "err", err)
		}
	}

	// Always start the public proxy listeners on 127.0.0.1:10808 and 10809
	c.startProxyForwarders(ctx)

	switch mode {
	case TunnelModeTunnel, TunnelModeSystem:
		c.cfg.RoutesToTUN = DefaultRoutesToTUN
		if err := c.setupSystemRouting(ctx); err != nil {
			_ = c.Disconnect(context.Background())
			return err
		}
	case TunnelModePerApp:
		c.cfg.RoutesToTUN = TelegramRoutes
		c.cfg.Logger.Info("Per-App Mode active: Routing Telegram subnets through TUN", "subnets", len(TelegramRoutes))
		if err := c.setupSystemRouting(ctx); err != nil {
			_ = c.Disconnect(context.Background())
			return err
		}
	case TunnelModeProxy:
		c.cfg.Logger.Info("Proxy Mode active: Listening on SOCKS5 10808 and HTTP 10809 (system proxy managed externally)")
	case TunnelModeBridge:
		c.cfg.RoutesToTUN = DefaultRoutesToTUN
		c.cfg.Logger.Info("Bridge Mode active: Setting up TUN routing with per-process rules")
		if err := c.setupSystemRouting(ctx); err != nil {
			_ = c.Disconnect(context.Background())
			return err
		}
	default:
		c.cfg.RoutesToTUN = DefaultRoutesToTUN
		if err := c.setupSystemRouting(ctx); err != nil {
			_ = c.Disconnect(context.Background())
			return err
		}
	}

	return nil
}

func findTunnelBinary() string {
	if p := os.Getenv("KITE_TUNNEL_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		p := filepath.Join(dir, "kite-tunnel")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		if runtime.GOOS == "darwin" {
			pHelper := filepath.Join(dir, "..", "Helpers", "kite-tunnel")
			if _, err := os.Stat(pHelper); err == nil {
				return filepath.Clean(pHelper)
			}
		}
	}
	for _, rel := range []string{"build/bin/kite-tunnel", "./kite-tunnel", "cmd/kite-tunnel/kite-tunnel"} {
		if _, err := os.Stat(rel); err == nil {
			if abs, err := filepath.Abs(rel); err == nil {
				return abs
			}
			return rel
		}
	}
	defaultPath := "/opt/kite/kite-tunnel"
	if runtime.GOOS == "darwin" {
		defaultPath = "/Applications/Kite.app/Contents/MacOS/kite-tunnel"
	}
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}
	if p, err := exec.LookPath("kite-tunnel"); err == nil {
		return p
	}
	return ""
}

func (c *Client) setupSystemRouting(ctx context.Context) error {
	if runtime.GOOS != "windows" {
		tunnelBin := c.cfg.TunnelBinaryPath
		if tunnelBin == "" {
			tunnelBin = findTunnelBinary()
		}
		if tunnelBin != "" {
			return c.setupSystemRoutingWithHelper(ctx, tunnelBin)
		}
	}

	return c.setupSystemRoutingInProcess(ctx)
}

func (c *Client) setupSystemRoutingWithHelper(ctx context.Context, tunnelBin string) error {
	args := []string{
		"--socks5", c.cfg.InboundProxy.String(),
		"--mode", string(c.cfg.Mode),
	}
	if c.isAWG && c.awgLink != "" && c.cfg.Mode != TunnelModeBridge {
		args = append(args, "--engine", "awg", "--awg-link", c.awgLink)
	}
	if c.cfg.ConfigPath != "" {
		args = append(args, "--config", c.cfg.ConfigPath)
	}
	if c.cfg.TunnelDNS != "" {
		args = append(args, "--tun-dns", c.cfg.TunnelDNS)
	}
	if c.cfg.TUNAddress != nil {
		args = append(args, "--tun-addr", c.cfg.TUNAddress.String())
		if len(c.cfg.TUNAddress.IP) > 0 {
			args = append(args, "--tun-gw", c.cfg.TUNAddress.IP.String())
		}
	}
	if len(c.cfg.RoutesToTUN) > 0 {
		var rStrs []string
		for _, r := range c.cfg.RoutesToTUN {
			rStrs = append(rStrs, r.String())
		}
		args = append(args, "--routes", strings.Join(rStrs, ","))
	}
	if c.xSrvIP != nil {
		args = append(args, "--bypass-ip", c.xSrvIP.String())
	}
	if len(c.cfg.BypassIPs) > 0 {
		args = append(args, "--bypass-ips", strings.Join(c.cfg.BypassIPs, ","))
	}
	if c.cfg.GatewayIP != nil {
		args = append(args, "--gateway-ip", c.cfg.GatewayIP.String())
	}

	c.cfg.Logger.Info("Spawning kite-tunnel helper", "bin", tunnelBin, "args", args)
	cmd := exec.Command(tunnelBin, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("open kite-tunnel stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("open kite-tunnel stdout: %w", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("start kite-tunnel helper: %w", err)
	}

	c.tunnelCmd = cmd
	c.tunnelStdin = stdin

	readyChan := make(chan error, 1)
	scanner := bufio.NewScanner(stdout)

	go func() {
		var isReady bool
		var rawLines []string
		for scanner.Scan() {
			line := scanner.Bytes()
			var ev struct {
				Event     string `json:"event"`
				Interface string `json:"interface"`
				BytesIn   int64  `json:"bytes_in"`
				BytesOut  int64  `json:"bytes_out"`
				Message   string `json:"message"`
			}
			if err := json.Unmarshal(line, &ev); err != nil {
				if trimmed := strings.TrimSpace(string(line)); trimmed != "" {
					rawLines = append(rawLines, trimmed)
					if len(rawLines) > 10 {
						rawLines = rawLines[1:]
					}
				}
				continue
			}
			switch ev.Event {
			case "ready":
				c.ifName = ev.Interface
				if !isReady {
					isReady = true
					readyChan <- nil
				}
			case "stats":
				c.bytesRead.Store(ev.BytesIn)
				c.bytesWritten.Store(ev.BytesOut)
			case "error":
				if !isReady {
					isReady = true
					readyChan <- errors.New(ev.Message)
				}
			}
		}

		cmdErr := cmd.Wait()
		if !isReady {
			detail := strings.TrimSpace(stderrBuf.String())
			if detail == "" && len(rawLines) > 0 {
				detail = strings.Join(rawLines, " | ")
			}
			if detail != "" {
				readyChan <- fmt.Errorf("kite-tunnel exited prematurely (%w): %s", cmdErr, detail)
			} else if cmdErr != nil {
				readyChan <- fmt.Errorf("kite-tunnel exited prematurely: %w", cmdErr)
			} else {
				readyChan <- errors.New("kite-tunnel closed unexpectedly")
			}
		}
		select {
		case c.tunnelStopped <- cmdErr:
		default:
		}
	}()

	select {
	case err := <-readyChan:
		if err != nil {
			_ = c.cleanupTunnelHelper()
			return fmt.Errorf("kite-tunnel startup failed: %w", err)
		}
	case <-time.After(10 * time.Second):
		_ = c.cleanupTunnelHelper()
		return errors.New("timed out waiting for kite-tunnel ready event")
	case <-ctx.Done():
		_ = c.cleanupTunnelHelper()
		return ctx.Err()
	}

	return nil
}

func (c *Client) cleanupTunnelHelper() error {
	var errs []error
	if c.tunnelStdin != nil {
		if err := c.tunnelStdin.Close(); err != nil {
			errs = append(errs, err)
		}
		c.tunnelStdin = nil
	}
	if c.tunnelCmd != nil && c.tunnelCmd.Process != nil {
		_ = c.tunnelCmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-c.tunnelStopped:
		case <-time.After(300 * time.Millisecond):
			_ = c.tunnelCmd.Process.Kill()
		}
		c.tunnelCmd = nil
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (c *Client) setupSystemRoutingInProcess(ctx context.Context) error {
	var err error
	c.tunnel, err = c.setupTunnel()
	if err != nil {
		return fmt.Errorf("setup TUN device: %w", err)
	}

	// Meter the TUN device
	c.tunnel = newMeteredReadWriteCloser(c.tunnel, &c.bytesRead, &c.bytesWritten)

	// Set XRay remote address to be routed through the default gateway, so that we don't get a loop.
	_ = c.routes.Delete(c.xrayToGatewayRoute())
	err = c.routes.Add(c.xrayToGatewayRoute())
	if err != nil {
		c.cfg.Logger.Error("routing xray server IP to default route failed", "err", err)
		_ = c.routes.Delete(route.Opts{IfName: c.tunIfName(), Routes: c.cfg.RoutesToTUN})
		if c.tunnel != nil {
			_ = c.tunnel.Close()
			c.tunnel = nil
		}
		return fmt.Errorf("add xray server route exception: %w", err)
	}

	go func() {
		var errPipe error
		if c.cfg.Mode == TunnelModeBridge {
			c.setupBridgeBypass()
			if c.bridgeDialerFactory != nil {
				d := c.bridgeDialerFactory(c.cfg.InboundProxy.String())
				errPipe = c.pipe.CopyWithDialer(ctx, c.tunnel, d)
			} else {
				errPipe = c.pipe.Copy(ctx, c.tunnel, c.cfg.InboundProxy.String())
			}
			c.cleanupBridgeBypass()
		} else {
			errPipe = c.pipe.Copy(ctx, c.tunnel, c.cfg.InboundProxy.String())
		}
		select {
		case c.tunnelStopped <- errPipe:
		default:
		}
	}()

	return nil
}

func (c *Client) setupBridgeBypass() {
	iface, err := getPhysicalInterface()
	if err != nil {
		c.cfg.Logger.Error("failed to find physical interface for bridge bypass", "err", err)
		return
	}
	c.cfg.Logger.Info("Bridge mode binding direct traffic to physical interface", "name", iface.Name, "index", iface.Index)
	dialer.DefaultDialer.InterfaceIndex.Store(int32(iface.Index))
	dialer.DefaultDialer.InterfaceName.Store(iface.Name)

	if runtime.GOOS == "darwin" && c.cfg.GatewayIP != nil {
		gwStr := c.cfg.GatewayIP.String()
		routes := []struct {
			dest string
			mask string
		}{
			{"0.0.0.0", "128.0.0.0"},
			{"128.0.0.0", "128.0.0.0"},
		}
		for _, r := range routes {
			_ = exec.Command("/sbin/route", "-q", "delete", "-net", "-ifscope", iface.Name, r.dest, gwStr, r.mask).Run()
			_ = exec.Command("/sbin/route", "-q", "delete", "-net", "-ifscope", iface.Name, r.dest, r.mask).Run()
			_ = exec.Command("/sbin/route", "-q", "delete", "-net", "-ifscope", iface.Name, r.dest+"/1").Run()

			if out, err := exec.Command("/sbin/route", "-q", "add", "-net", "-ifscope", iface.Name, r.dest, gwStr, r.mask).CombinedOutput(); err != nil {
				cidr := r.dest + "/1"
				_ = exec.Command("/sbin/route", "-q", "add", "-net", "-ifscope", iface.Name, cidr, gwStr).Run()
				_ = out
			}
		}
		_ = exec.Command("/sbin/route", "-q", "delete", "-ifscope", iface.Name, "default", gwStr).Run()
		_ = exec.Command("/sbin/route", "-q", "delete", "-ifscope", iface.Name, "default").Run()
		_ = exec.Command("/sbin/route", "-q", "add", "-ifscope", iface.Name, "default", gwStr).Run()
	}
}

func (c *Client) cleanupBridgeBypass() {
	if runtime.GOOS == "darwin" {
		name := dialer.DefaultDialer.InterfaceName.Load()
		if name != "" {
			var gwStr string
			if c.cfg.GatewayIP != nil {
				gwStr = c.cfg.GatewayIP.String()
			}
			for _, dest := range []string{"0.0.0.0", "128.0.0.0"} {
				if gwStr != "" {
					_ = exec.Command("/sbin/route", "-q", "delete", "-net", "-ifscope", name, dest, gwStr, "128.0.0.0").Run()
				}
				_ = exec.Command("/sbin/route", "-q", "delete", "-net", "-ifscope", name, dest, "128.0.0.0").Run()
				_ = exec.Command("/sbin/route", "-q", "delete", "-net", "-ifscope", name, dest+"/1").Run()
			}
			if gwStr != "" {
				_ = exec.Command("/sbin/route", "-q", "delete", "-ifscope", name, "default", gwStr).Run()
			}
			_ = exec.Command("/sbin/route", "-q", "delete", "-ifscope", name, "default").Run()
		}
	}
	dialer.DefaultDialer.InterfaceIndex.Store(0)
	dialer.DefaultDialer.InterfaceName.Store("")
}

func getPhysicalInterface() (*net.Interface, error) {
	ifIP, err := gateway.DiscoverInterface()
	if err == nil && ifIP != nil && !ifIP.IsUnspecified() {
		ifaces, _ := net.Interfaces()
		for _, ifc := range ifaces {
			addrs, _ := ifc.Addrs()
			for _, addr := range addrs {
				if ipNet, ok := addr.(*net.IPNet); ok {
					if ipNet.IP.Equal(ifIP) {
						return &ifc, nil
					}
				}
			}
		}
	}

	gwIP, err := gateway.DiscoverGateway()
	if err == nil && gwIP != nil && !gwIP.IsUnspecified() {
		ifaces, _ := net.Interfaces()
		for _, ifc := range ifaces {
			if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
				continue
			}
			name := strings.ToLower(ifc.Name)
			if strings.Contains(name, "tun") || strings.Contains(name, "tap") || strings.Contains(name, "wintun") || strings.Contains(name, "kite") {
				continue
			}
			addrs, _ := ifc.Addrs()
			for _, addr := range addrs {
				if ipNet, ok := addr.(*net.IPNet); ok {
					if ipNet.Contains(gwIP) {
						return &ifc, nil
					}
				}
			}
		}
	}

	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp != 0 && ifc.Flags&net.FlagBroadcast != 0 && ifc.Flags&net.FlagLoopback == 0 {
			name := strings.ToLower(ifc.Name)
			if strings.Contains(name, "tun") || strings.Contains(name, "tap") || strings.Contains(name, "wintun") || strings.Contains(name, "kite") {
				continue
			}
			return &ifc, nil
		}
	}

	return nil, errors.New("no physical network interface found")
}

func (c *Client) tunIfName() string {
	if c.ifName != "" {
		return c.ifName
	}
	if runtime.GOOS == "windows" {
		return "kite0"
	}
	return "tun0"
}

func (c *Client) Disconnect(ctx context.Context) error {
	c.cleanupBridgeBypass()

	if c.stopTunnel != nil {
		c.stopTunnel()
		c.stopTunnel = nil
	}

	var errs []error

	// 1. Remove tunnel & routes FIRST so system network is immediately restored
	if c.tunnelStdin != nil || c.tunnelCmd != nil {
		if err := c.cleanupTunnelHelper(); err != nil {
			errs = append(errs, err)
		}
	}

	if c.tunnel != nil {
		if err := c.tunnel.Close(); err != nil {
			errs = append(errs, err)
		}
		c.tunnel = nil
		if c.xSrvIP != nil && c.cfg.GatewayIP != nil {
			_ = c.routes.Delete(c.xrayToGatewayRoute())
		}
		_ = c.routes.Delete(route.Opts{IfName: c.tunIfName(), Routes: c.cfg.RoutesToTUN})

		ctxTimeout, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		select {
		case tunErr := <-c.tunnelStopped:
			if tunErr != nil && !errors.Is(tunErr, context.Canceled) {
				errs = append(errs, tunErr)
			}
		case <-ctxTimeout.Done():
		}
	}

	// 2. Shut down proxy forwarders & XRay core AFTER tunnel routes are removed
	if c.directSocksLn != nil {
		_ = c.directSocksLn.Close()
		c.directSocksLn = nil
	}
	if c.httpLn != nil {
		_ = c.httpLn.Close()
		c.httpLn = nil
	}
	c.proxyWg.Wait()

	if c.xInst != nil {
		if err := c.xInst.Close(); err != nil {
			errs = append(errs, err)
		}
		c.xInst = nil
	}

	if c.awgEngine != nil {
		if err := c.awgEngine.Close(); err != nil {
			errs = append(errs, err)
		}
		c.awgEngine = nil
	}
	c.isAWG = false
	c.awgLink = ""

	c.cfg.Logger.Info("Client disconnected successfully")

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (c *Client) xrayToGatewayRoute() route.Opts {
	var gw net.IP
	if c.cfg.GatewayIP != nil {
		gw = *c.cfg.GatewayIP
	}
	var routes []*route.Addr
	if c.xSrvIP != nil {
		routes = []*route.Addr{route.MustParseAddr(c.xSrvIP.String() + "/32")}
	}
	return route.Opts{
		Gateway: gw,
		Routes:  routes,
	}
}

func (c *Client) createXrayProxy(link string) (xrayproto.Instance, *xrayproto.GeneralConfig, error) {
	inbound := &xray.Socks{
		Remark:  "Kite-XRay-Inbound",
		Address: c.cfg.InboundProxy.IP.String(),
		Port:    strconv.Itoa(c.cfg.InboundProxy.Port),
	}

	svc := xray.NewXrayService(true,
		c.cfg.TLSAllowInsecure,
		xray.WithCustomLogLevel(c.cfg.XRayLogType, xRayLogLevel(c.cfg.Logger.Handler())),
		xray.WithInbound(inbound),
	)

	link = strings.TrimSpace(link)
	protocol, err := svc.CreateProtocol(link)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid config: protocol create: %w", err)
	}

	if err := protocol.Parse(); err != nil {
		return nil, nil, fmt.Errorf("invalid config: parse: %w", err)
	}

	cfg := protocol.ConvertToGeneralConfig()

	inst, err := svc.MakeInstance(protocol)
	if err != nil {
		return nil, nil, fmt.Errorf("make instance: %w", err)
	}

	host := cfg.Address
	if h, _, err := net.SplitHostPort(cfg.Address); err == nil {
		host = h
	}
	ip, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return nil, nil, fmt.Errorf("xray address not resolvable: %w", err)
	}
	c.xSrvIP = ip

	return inst, &cfg, nil
}

func (c *Client) setupTunnel() (*tun.Interface, error) {
	ifc, err := tun.New("", 1500)
	if err != nil {
		return nil, fmt.Errorf("create tun: %w", err)
	}
	c.ifName = ifc.Name()

	tunAddr := c.cfg.TUNAddress
	if tunAddr == nil {
		tunAddr = defaultTUNAddress
	}

	if err = ifc.Up(tunAddr, tunAddr.IP); err != nil {
		return nil, fmt.Errorf("setup interface: %w", err)
	}

	if c.cfg.TunnelDNS != "" {
		if err := ifc.SetDNS(c.cfg.TunnelDNS); err != nil {
			c.cfg.Logger.Warn("failed to set TUN DNS", "dns", c.cfg.TunnelDNS, "err", err)
		}
	}

	if err = c.routes.Add(route.Opts{IfName: ifc.Name(), Routes: c.cfg.RoutesToTUN}); err != nil {
		return nil, fmt.Errorf("add route: %w", err)
	}

	return ifc, nil
}

// startProxyForwarders starts the HTTP CONNECT proxy (10809) which forwards through XRay SOCKS5 (10808).
func (c *Client) startProxyForwarders(ctx context.Context) {
	internalSocks := c.cfg.InboundProxy.String()

	// HTTP CONNECT Forwarder on 127.0.0.1:10809
	httpAddr := fmt.Sprintf("127.0.0.1:%d", c.cfg.HTTPPort)
	if ln, err := net.Listen("tcp", httpAddr); err == nil {
		c.httpLn = ln
		c.proxyWg.Add(1)
		go func() {
			defer c.proxyWg.Done()
			c.serveHTTPProxy(ctx, ln, internalSocks)
		}()
		c.cfg.Logger.Info("HTTP proxy listening", "addr", httpAddr)
	} else {
		c.cfg.Logger.Warn("Could not bind standard HTTP port", "addr", httpAddr, "err", err)
	}
}

func (c *Client) serveHTTPProxy(ctx context.Context, ln net.Listener, targetSocks string) {
	dialer, err := proxy.SOCKS5("tcp", targetSocks, nil, proxy.Direct)
	if err != nil {
		c.cfg.Logger.Error("create socks5 dialer for http proxy", "err", err)
		return
	}

	for {
		clientConn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				return
			}
		}

		go c.handleHTTPProxyConn(clientConn, dialer)
	}
}

func (c *Client) handleHTTPProxyConn(clientConn net.Conn, dialer proxy.Dialer) {
	defer clientConn.Close()

	reader := bufio.NewReader(clientConn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		return
	}

	if req.Method == http.MethodConnect {
		// HTTPS tunneling via CONNECT
		targetConn, err := dialer.Dial("tcp", req.Host)
		if err != nil {
			_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			return
		}
		defer targetConn.Close()

		_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		if err != nil {
			return
		}

		c.relay(clientConn, targetConn)
		return
	}

	// Plain HTTP proxy
	host := req.Host
	if !strings.Contains(host, ":") {
		host = host + ":80"
	}

	targetConn, err := dialer.Dial("tcp", host)
	if err != nil {
		_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer targetConn.Close()

	// Count bytes written (upload from client)
	var buf bytes.Buffer
	if err := req.Write(&buf); err == nil {
		c.bytesRead.Add(int64(buf.Len()))
		_, _ = targetConn.Write(buf.Bytes())
	}

	c.relay(clientConn, targetConn)
}

func (c *Client) relay(clientConn, targetConn net.Conn) {
	var once sync.Once
	closeBoth := func() {
		_ = clientConn.Close()
		_ = targetConn.Close()
	}

	// Client -> Target (Upload)
	go func() {
		buf := make([]byte, 32*1024)
		for {
			nr, err := clientConn.Read(buf)
			if nr > 0 {
				c.bytesRead.Add(int64(nr))
				_, werr := targetConn.Write(buf[:nr])
				if werr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		once.Do(closeBoth)
	}()

	// Target -> Client (Download)
	buf := make([]byte, 32*1024)
	for {
		nr, err := targetConn.Read(buf)
		if nr > 0 {
			c.bytesWritten.Add(int64(nr))
			_, werr := clientConn.Write(buf[:nr])
			if werr != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}
	once.Do(closeBoth)
}

type meteredReadWriteCloser struct {
	io.ReadWriteCloser
	readCounter    *atomic.Int64
	writtenCounter *atomic.Int64
}

func newMeteredReadWriteCloser(rwc io.ReadWriteCloser, read, written *atomic.Int64) *meteredReadWriteCloser {
	return &meteredReadWriteCloser{
		ReadWriteCloser: rwc,
		readCounter:    read,
		writtenCounter: written,
	}
}

func (m *meteredReadWriteCloser) Read(p []byte) (int, error) {
	n, err := m.ReadWriteCloser.Read(p)
	if n > 0 && m.readCounter != nil {
		m.readCounter.Add(int64(n))
	}
	return n, err
}

func (m *meteredReadWriteCloser) Write(p []byte) (int, error) {
	n, err := m.ReadWriteCloser.Write(p)
	if n > 0 && m.writtenCounter != nil {
		m.writtenCounter.Add(int64(n))
	}
	return n, err
}

func xRayLogLevel(h slog.Handler) xcommlog.Severity {
	ctx := context.Background()
	switch {
	case h.Enabled(ctx, slog.LevelDebug):
		return xcommlog.Severity_Debug
	case h.Enabled(ctx, slog.LevelInfo):
		return xcommlog.Severity_Info
	case h.Enabled(ctx, slog.LevelWarn):
		return xcommlog.Severity_Warning
	case h.Enabled(ctx, slog.LevelError):
		return xcommlog.Severity_Error
	}
	return xcommlog.Severity_Unknown
}

func getFreePort() int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 10810
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func (c *Client) startDirectSocksServer(ctx context.Context) error {
	addr := fmt.Sprintf("127.0.0.1:%d", c.cfg.SocksPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen direct socks5 %s: %w", addr, err)
	}
	c.directSocksLn = ln

	c.proxyWg.Add(1)
	go func() {
		defer c.proxyWg.Done()
		<-ctx.Done()
		_ = ln.Close()
	}()

	c.proxyWg.Add(1)
	go func() {
		defer c.proxyWg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go c.handleDirectSocksConn(ctx, conn)
		}
	}()

	c.cfg.Logger.Info("Direct SOCKS5 proxy listening", "addr", addr)
	return nil
}

func (c *Client) handleDirectSocksConn(ctx context.Context, clientConn net.Conn) {
	defer clientConn.Close()

	reader := bufio.NewReader(clientConn)

	// 1. SOCKS5 Greeting
	ver, err := reader.ReadByte()
	if err != nil || ver != 0x05 {
		return
	}
	nMethods, err := reader.ReadByte()
	if err != nil || nMethods == 0 {
		return
	}
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(reader, methods); err != nil {
		return
	}
	// Select NO AUTH (0x00)
	if _, err := clientConn.Write([]byte{0x05, 0x00}); err != nil {
		return
	}

	// 2. SOCKS5 Request
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return
	}
	if header[0] != 0x05 || header[1] != 0x01 { // 0x01 = CONNECT
		_, _ = clientConn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	var host string
	switch header[3] { // ATYP
	case 0x01: // IPv4
		ipv4 := make([]byte, 4)
		if _, err := io.ReadFull(reader, ipv4); err != nil {
			return
		}
		host = net.IP(ipv4).String()
	case 0x03: // Domain
		domainLen, err := reader.ReadByte()
		if err != nil || domainLen == 0 {
			return
		}
		domain := make([]byte, domainLen)
		if _, err := io.ReadFull(reader, domain); err != nil {
			return
		}
		host = string(domain)
	case 0x04: // IPv6
		ipv6 := make([]byte, 16)
		if _, err := io.ReadFull(reader, ipv6); err != nil {
			return
		}
		host = net.IP(ipv6).String()
	default:
		_, _ = clientConn.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(reader, portBuf); err != nil {
		return
	}
	port := int(portBuf[0])<<8 | int(portBuf[1])
	targetAddr := fmt.Sprintf("%s:%d", host, port)

	dialer := &net.Dialer{Timeout: 7 * time.Second}
	targetConn, err := dialer.DialContext(ctx, "tcp", targetAddr)
	if err != nil {
		_, _ = clientConn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer targetConn.Close()

	if _, err := clientConn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(targetConn, reader)
		if tc, ok := targetConn.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(clientConn, targetConn)
		if cc, ok := clientConn.(*net.TCPConn); ok {
			_ = cc.CloseWrite()
		}
	}()
	wg.Wait()
}

// SetBypassIPs configures list of remote server IPs that bypass the VPN tunnel.
func (c *Client) SetBypassIPs(ips []string) {
	c.cfg.BypassIPs = ips
}

