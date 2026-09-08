package awg

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/amnezia-vpn/amneziawg-go/conn"
	"github.com/amnezia-vpn/amneziawg-go/device"
	"github.com/amnezia-vpn/amneziawg-go/tun"
	"github.com/amnezia-vpn/amneziawg-go/tun/netstack"
	"github.com/xjasonlyu/tun2socks/v2/transport/socks5"
	"github.com/goxray/core/wireguard"
)

// Engine manages an in-process AmneziaWG userspace tunnel and SOCKS5 proxy server.
type Engine struct {
	dev    *device.Device
	tunDev tun.Device
	tnet   *netstack.Net

	tcpLn net.Listener
	udpLn *net.UDPConn

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	bytesRead    atomic.Int64
	bytesWritten atomic.Int64
	closeOnce    sync.Once
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) BytesRead() int64 {
	return e.bytesRead.Load()
}

func (e *Engine) BytesWritten() int64 {
	return e.bytesWritten.Load()
}

func (e *Engine) Addr() string {
	if e.tcpLn != nil {
		return e.tcpLn.Addr().String()
	}
	return ""
}

// Start brings up the AmneziaWG tunnel and starts a SOCKS5 proxy on 127.0.0.1:port.
func (e *Engine) Start(cfg *wireguard.Config, socksPort int) error {
	e.ctx, e.cancel = context.WithCancel(context.Background())

	// 1. Parse local addresses
	localAddrs, err := parseIPs(cfg.Address)
	if err != nil || len(localAddrs) == 0 {
		localAddrs = []netip.Addr{netip.MustParseAddr("10.0.0.2")}
	}

	// 2. Parse DNS servers
	dnsAddrs, err := parseIPs(cfg.DNS)
	if err != nil || len(dnsAddrs) == 0 {
		dnsAddrs = []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("8.8.8.8")}
	}

	mtu := cfg.MTU
	if mtu <= 0 {
		mtu = 1420
	}

	// 3. Create virtual TUN backed by gVisor netstack
	tunDev, tnet, err := netstack.CreateNetTUN(localAddrs, dnsAddrs, mtu)
	if err != nil {
		e.cancel()
		return fmt.Errorf("create awg netstack tun: %w", err)
	}
	e.tunDev = tunDev
	e.tnet = tnet

	// 4. Create AmneziaWG device
	dev := device.NewDevice(tunDev, conn.NewDefaultBind(), device.NewLogger(device.LogLevelVerbose, "awg: "))
	e.dev = dev

	// 4. Set IPC configuration
	ipcStr, err := BuildIPCConfig(cfg)
	if err != nil {
		e.Close()
		return fmt.Errorf("build awg ipc config: %w", err)
	}

	if err := dev.IpcSet(ipcStr); err != nil {
		e.Close()
		return fmt.Errorf("set awg ipc: %w", err)
	}

	if err := dev.Up(); err != nil {
		e.Close()
		return fmt.Errorf("awg dev up: %w", err)
	}

	// 6. Start local SOCKS5 server on 127.0.0.1:socksPort
	addr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	tcpLn, err := net.Listen("tcp", addr)
	if err != nil {
		e.Close()
		return fmt.Errorf("listen socks5 tcp %s: %w", addr, err)
	}
	e.tcpLn = tcpLn

	// UDP listener for SOCKS5 UDP Associate
	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err == nil {
		if udpConn, err := net.ListenUDP("udp", udpAddr); err == nil {
			e.udpLn = udpConn
			e.wg.Add(1)
			go e.serveUDP(udpConn)
		}
	}

	e.wg.Add(1)
	go e.serveTCP(tcpLn)

	return nil
}

func (e *Engine) Close() error {
	e.closeOnce.Do(func() {
		if e.cancel != nil {
			e.cancel()
		}
		if e.tcpLn != nil {
			_ = e.tcpLn.Close()
		}
		if e.udpLn != nil {
			_ = e.udpLn.Close()
		}
		e.wg.Wait()

		if e.dev != nil {
			e.dev.Close() // dev.Close() already closes the underlying tunDev
			e.dev = nil
			e.tunDev = nil
		} else if e.tunDev != nil {
			_ = e.tunDev.Close()
			e.tunDev = nil
		}
	})
	return nil
}

func (e *Engine) serveTCP(ln net.Listener) {
	defer e.wg.Done()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-e.ctx.Done():
				return
			default:
				return
			}
		}

		go e.handleTCPConn(conn)
	}
}

func (e *Engine) handleTCPConn(clientConn net.Conn) {
	defer clientConn.Close()

	// SOCKS5 greeting handshake
	header := make([]byte, 2)
	if _, err := io.ReadFull(clientConn, header); err != nil {
		return
	}
	if header[0] != 0x05 {
		return
	}
	nmethods := int(header[1])
	methods := make([]byte, nmethods)
	if _, err := io.ReadFull(clientConn, methods); err != nil {
		return
	}

	// Respond: no authentication required
	if _, err := clientConn.Write([]byte{0x05, 0x00}); err != nil {
		return
	}

	// SOCKS5 request header
	cmdBuf := make([]byte, 3)
	if _, err := io.ReadFull(clientConn, cmdBuf); err != nil {
		return
	}
	if cmdBuf[0] != 0x05 {
		return
	}
	cmd := cmdBuf[1]

	buf := make([]byte, 512)
	targetAddr, err := socks5.ReadAddr(clientConn, buf)
	if err != nil {
		slog.Debug("awg socks5: read addr failed", "err", err)
		return
	}

	switch cmd {
	case byte(socks5.CmdConnect):
		dialCtx, dialCancel := context.WithTimeout(e.ctx, 7*time.Second)
		targetConn, err := e.tnet.DialContext(dialCtx, "tcp", targetAddr.String())
		dialCancel()
		if err != nil {
			slog.Debug("awg socks5: dial failed", "target", targetAddr.String(), "err", err)
			_, _ = clientConn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			return
		}
		defer targetConn.Close()

		// Success response
		if _, err := clientConn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
			return
		}

		e.relay(clientConn, targetConn)

	case 0x03: // UDP Associate
		if e.udpLn == nil {
			_, _ = clientConn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			return
		}
		udpLocal := e.udpLn.LocalAddr().(*net.UDPAddr)
		resp := []byte{0x05, 0x00, 0x00, 0x01}
		resp = append(resp, udpLocal.IP.To4()...)
		resp = append(resp, byte(udpLocal.Port>>8), byte(udpLocal.Port&0xFF))
		if _, err := clientConn.Write(resp); err != nil {
			return
		}

		// Keep connection open until client disconnects
		dummy := make([]byte, 1)
		for {
			if _, err := clientConn.Read(dummy); err != nil {
				return
			}
		}

	default:
		_, _ = clientConn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	}
}

func (e *Engine) serveUDP(udpConn *net.UDPConn) {
	defer e.wg.Done()
	buf := make([]byte, 65535)

	type udpSession struct {
		targetConn net.Conn
		lastActive time.Time
	}
	sessions := make(map[string]*udpSession)
	var mu sync.Mutex

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-e.ctx.Done():
				return
			case <-ticker.C:
				mu.Lock()
				now := time.Now()
				for k, s := range sessions {
					if now.Sub(s.lastActive) > 60*time.Second {
						_ = s.targetConn.Close()
						delete(sessions, k)
					}
				}
				mu.Unlock()
			}
		}
	}()

	for {
		n, clientAddr, err := udpConn.ReadFrom(buf)
		if err != nil {
			select {
			case <-e.ctx.Done():
				return
			default:
				return
			}
		}

		if n < 4 {
			continue
		}

		rawPkt := make([]byte, n)
		copy(rawPkt, buf[:n])

		addr, payload, err := socks5.DecodeUDPPacket(rawPkt)
		if err != nil {
			continue
		}

		e.bytesRead.Add(int64(len(payload)))

		targetAddrCopy := append(socks5.Addr(nil), addr...)
		clientUDPAddr := &net.UDPAddr{
			IP:   append([]byte(nil), clientAddr.(*net.UDPAddr).IP...),
			Port: clientAddr.(*net.UDPAddr).Port,
			Zone: clientAddr.(*net.UDPAddr).Zone,
		}

		sessKey := fmt.Sprintf("%s->%s", clientUDPAddr.String(), targetAddrCopy.String())
		mu.Lock()
		sess, ok := sessions[sessKey]
		if !ok {
			targetConn, err := e.tnet.DialContext(e.ctx, "udp", targetAddrCopy.String())
			if err != nil {
				mu.Unlock()
				continue
			}
			sess = &udpSession{targetConn: targetConn, lastActive: time.Now()}
			sessions[sessKey] = sess

			// Goroutine to read back UDP reply from target
			go func(key string, c net.Conn, returnAddr net.Addr, replyTargetAddr socks5.Addr) {
				defer func() {
					_ = c.Close()
					mu.Lock()
					delete(sessions, key)
					mu.Unlock()
				}()

				backBuf := make([]byte, 65535)
				for {
					nr, err := c.Read(backBuf)
					if err != nil {
						return
					}
					pkt, err := socks5.EncodeUDPPacket(replyTargetAddr, backBuf[:nr])
					if err == nil {
						e.bytesWritten.Add(int64(nr))
						_, _ = udpConn.WriteTo(pkt, returnAddr)
					}
				}
			}(sessKey, targetConn, clientUDPAddr, targetAddrCopy)
		}
		sess.lastActive = time.Now()
		mu.Unlock()

		_, _ = sess.targetConn.Write(payload)
	}
}

func (e *Engine) relay(left, right net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			nr, er := left.Read(buf)
			if nr > 0 {
				e.bytesRead.Add(int64(nr))
				nw, ew := right.Write(buf[:nr])
				if ew != nil || nr != nw {
					break
				}
			}
			if er != nil {
				break
			}
		}
		_ = right.Close()
	}()

	go func() {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			nr, er := right.Read(buf)
			if nr > 0 {
				e.bytesWritten.Add(int64(nr))
				nw, ew := left.Write(buf[:nr])
				if ew != nil || nr != nw {
					break
				}
			}
			if er != nil {
				break
			}
		}
		_ = left.Close()
	}()

	wg.Wait()
}

// BuildIPCConfig generates the UAPI IPC configuration string for AmneziaWG device.
func BuildIPCConfig(cfg *wireguard.Config) (string, error) {
	skHex, err := keyToHex(cfg.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}
	pkHex, err := keyToHex(cfg.PublicKey)
	if err != nil {
		return "", fmt.Errorf("invalid public key: %w", err)
	}

	var ipc strings.Builder
	ipc.WriteString(fmt.Sprintf("private_key=%s\n", skHex))

	// Obfuscation headers for AmneziaWG
	if cfg.Jc > 0 {
		ipc.WriteString(fmt.Sprintf("jc=%d\n", cfg.Jc))
	}
	if cfg.Jmin > 0 {
		ipc.WriteString(fmt.Sprintf("jmin=%d\n", cfg.Jmin))
	}
	if cfg.Jmax > 0 {
		ipc.WriteString(fmt.Sprintf("jmax=%d\n", cfg.Jmax))
	}
	if cfg.S1 > 0 {
		ipc.WriteString(fmt.Sprintf("s1=%d\n", cfg.S1))
	}
	if cfg.S2 > 0 {
		ipc.WriteString(fmt.Sprintf("s2=%d\n", cfg.S2))
	}
	if cfg.H1 > 0 {
		ipc.WriteString(fmt.Sprintf("h1=%d\n", cfg.H1))
	}
	if cfg.H2 > 0 {
		ipc.WriteString(fmt.Sprintf("h2=%d\n", cfg.H2))
	}
	if cfg.H3 > 0 {
		ipc.WriteString(fmt.Sprintf("h3=%d\n", cfg.H3))
	}
	if cfg.H4 > 0 {
		ipc.WriteString(fmt.Sprintf("h4=%d\n", cfg.H4))
	}

	ipc.WriteString(fmt.Sprintf("public_key=%s\n", pkHex))
	if cfg.PreSharedKey != "" {
		pskHex, err := keyToHex(cfg.PreSharedKey)
		if err == nil {
			ipc.WriteString(fmt.Sprintf("preshared_key=%s\n", pskHex))
		}
	}

	ipc.WriteString(fmt.Sprintf("endpoint=%s\n", cfg.Endpoint))
	if cfg.AllowedIPs != "" {
		for _, a := range strings.Split(cfg.AllowedIPs, ",") {
			a = strings.TrimSpace(a)
			if a != "" {
				ipc.WriteString(fmt.Sprintf("allowed_ip=%s\n", a))
			}
		}
	} else {
		ipc.WriteString("allowed_ip=0.0.0.0/0\nallowed_ip=::/0\n")
	}

	keepalive := cfg.PersistentKeepalive
	if keepalive <= 0 {
		keepalive = 25
	}
	ipc.WriteString(fmt.Sprintf("persistent_keepalive_interval=%d\n", keepalive))

	return ipc.String(), nil
}

func keyToHex(key string) (string, error) {
	k := strings.TrimSpace(key)
	if len(k) == 64 {
		if _, err := hex.DecodeString(k); err == nil {
			return k, nil
		}
	}
	b, err := base64.StdEncoding.DecodeString(k)
	if err != nil {
		return "", err
	}
	if len(b) != 32 {
		return "", errors.New("key must be 32 bytes")
	}
	return hex.EncodeToString(b), nil
}

func parseIPs(s string) ([]netip.Addr, error) {
	var res []netip.Addr
	parts := strings.Split(s, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(p); err == nil {
			res = append(res, prefix.Addr())
			continue
		}
		if addr, err := netip.ParseAddr(p); err == nil {
			res = append(res, addr)
		}
	}
	return res, nil
}

// Ping measures round-trip latency through AmneziaWG tunnel.
func Ping(cfg *wireguard.Config, timeout time.Duration) int64 {
	localAddrs, err := parseIPs(cfg.Address)
	if err != nil || len(localAddrs) == 0 {
		localAddrs = []netip.Addr{netip.MustParseAddr("10.0.0.2")}
	}
	dnsAddrs, _ := parseIPs(cfg.DNS)
	if len(dnsAddrs) == 0 {
		dnsAddrs = []netip.Addr{netip.MustParseAddr("1.1.1.1")}
	}

	mtu := cfg.MTU
	if mtu <= 0 {
		mtu = 1420
	}

	tunDev, tnet, err := netstack.CreateNetTUN(localAddrs, dnsAddrs, mtu)
	if err != nil {
		return -1
	}

	dev := device.NewDevice(tunDev, conn.NewDefaultBind(), device.NewLogger(device.LogLevelError, "awg-ping: "))
	defer dev.Close()

	ipcStr, err := BuildIPCConfig(cfg)
	if err != nil {
		return -1
	}
	if err := dev.IpcSet(ipcStr); err != nil {
		return -1
	}
	if err := dev.Up(); err != nil {
		return -1
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	conn, err := tnet.DialContext(ctx, "tcp", "1.1.1.1:443")
	if err != nil {
		conn, err = tnet.DialContext(ctx, "tcp", "cp.cloudflare.com:80")
		if err != nil {
			return -1
		}
	}
	_ = conn.Close()

	ms := time.Since(start).Milliseconds()
	if ms <= 0 {
		return 1
	}
	return ms
}
