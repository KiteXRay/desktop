package client

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goxray/core/network/route"
	"github.com/goxray/core/network/tun"
	"github.com/goxray/core/pipe2socks"
	"github.com/jackpal/gateway"
	xrayproto "github.com/lilendian0x00/xray-knife/v3/pkg/protocol"
	"github.com/lilendian0x00/xray-knife/v3/pkg/xray"
	xapplog "github.com/xtls/xray-core/app/log"
	xcommlog "github.com/xtls/xray-core/common/log"
	"golang.org/x/net/proxy"
)

type TunnelMode string

const (
	TunnelModeSystem TunnelMode = "system"
	TunnelModePerApp TunnelMode = "per_app"

	DefaultSocksPort = 10808
	DefaultHTTPPort  = 10809

	disconnectTimeout = 30 * time.Second
)

var (
	defaultTUNAddress = &net.IPNet{IP: net.IPv4(192, 18, 0, 1), Mask: net.IPv4Mask(255, 255, 255, 255)}

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
}

type Client struct {
	cfg Config

	mode TunnelMode

	xInst  xrayproto.Instance
	xCfg   *xrayproto.GeneralConfig
	xSrvIP *net.IPAddr

	tunnel io.ReadWriteCloser
	pipe   *pipe2socks.Pipe
	routes *route.Route

	tunnelStopped chan error
	stopTunnel    func()

	// Public HTTP proxy listener
	httpLn  net.Listener
	proxyWg sync.WaitGroup

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
			TUNAddress:  defaultTUNAddress,
			RoutesToTUN: DefaultRoutesToTUN,
			Logger:      slog.New(slog.NewTextHandler(os.Stdout, nil)),
			Mode:        TunnelModeSystem,
			SocksPort:   DefaultSocksPort,
			HTTPPort:    DefaultHTTPPort,
		},
		mode:          TunnelModeSystem,
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
	return c, nil
}

func (c *Client) Mode() TunnelMode {
	return c.mode
}

func (c *Client) SetMode(mode TunnelMode) {
	c.mode = mode
	c.cfg.Mode = mode
}

func (c *Client) BytesRead() int {
	return int(c.bytesRead.Load())
}

func (c *Client) BytesWritten() int {
	return int(c.bytesWritten.Load())
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

	// Ensure XRay listens directly on the standard SOCKS5 port (127.0.0.1:10808)
	c.cfg.InboundProxy.IP = net.IPv4(127, 0, 0, 1)
	c.cfg.InboundProxy.Port = c.cfg.SocksPort

	var err error
	c.xInst, c.xCfg, err = c.createXrayProxy(link)
	if err != nil {
		return fmt.Errorf("create xray core instance: %w", err)
	}

	if err = c.xInst.Start(); err != nil {
		return fmt.Errorf("start xray core instance: %w", err)
	}
	time.Sleep(120 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	c.stopTunnel = cancel

	// Always start the public proxy listeners on 127.0.0.1:10808 and 10809
	c.startProxyForwarders(ctx)

	if mode == TunnelModeSystem {
		c.cfg.RoutesToTUN = DefaultRoutesToTUN
	} else {
		// In Per-App mode, route Telegram subnets through TUN automatically, leaving the rest of OS untouched
		c.cfg.RoutesToTUN = TelegramRoutes
		c.cfg.Logger.Info("Per-App Mode active: Routing Telegram subnets through TUN", "subnets", len(TelegramRoutes))
	}

	if err := c.setupSystemRouting(ctx); err != nil {
		_ = c.Disconnect(context.Background())
		return err
	}

	return nil
}

func (c *Client) setupSystemRouting(ctx context.Context) error {
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
		_ = c.routes.Delete(route.Opts{IfName: "tun0", Routes: c.cfg.RoutesToTUN})
		if c.tunnel != nil {
			_ = c.tunnel.Close()
			c.tunnel = nil
		}
		return fmt.Errorf("add xray server route exception: %w", err)
	}

	go func() {
		errPipe := c.pipe.Copy(ctx, c.tunnel, c.cfg.InboundProxy.String())
		select {
		case c.tunnelStopped <- errPipe:
			default:
		}
	}()

	return nil
}

func (c *Client) Disconnect(ctx context.Context) error {
	if c.stopTunnel != nil {
		c.stopTunnel()
		c.stopTunnel = nil
	}

	var errs []error

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

	if c.tunnel != nil {
		if err := c.tunnel.Close(); err != nil {
			errs = append(errs, err)
		}
		c.tunnel = nil
		if c.xSrvIP != nil && c.cfg.GatewayIP != nil {
			_ = c.routes.Delete(c.xrayToGatewayRoute())
		}
		_ = c.routes.Delete(route.Opts{IfName: "tun0", Routes: c.cfg.RoutesToTUN})

		ctxTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		select {
		case tunErr := <-c.tunnelStopped:
			if tunErr != nil && !errors.Is(tunErr, context.Canceled) {
				errs = append(errs, tunErr)
			}
		case <-ctxTimeout.Done():
		}
	}

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

	ip, err := net.ResolveIPAddr("ip", cfg.Address)
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

	if err = ifc.Up(c.cfg.TUNAddress, c.cfg.TUNAddress.IP); err != nil {
		return nil, fmt.Errorf("setup interface: %w", err)
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
