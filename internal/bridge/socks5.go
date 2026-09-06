package bridge

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/xjasonlyu/tun2socks/v2/dialer"
	M "github.com/xjasonlyu/tun2socks/v2/metadata"
	"github.com/xjasonlyu/tun2socks/v2/proxy"
	"github.com/xjasonlyu/tun2socks/v2/proxy/proto"
	"github.com/xjasonlyu/tun2socks/v2/transport/socks5"
)

var _ proxy.Proxy = (*socks5Proxy)(nil)

type socks5Proxy struct {
	addr    string
	user    string
	pass    string
	isLocal bool
}

func newSocks5Proxy(addr, user, pass string) (proxy.Proxy, error) {
	return &socks5Proxy{
		addr:    addr,
		user:    user,
		pass:    pass,
		isLocal: isLoopback(addr),
	}, nil
}

func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *socks5Proxy) Addr() string {
	return s.addr
}

func (s *socks5Proxy) Proto() proto.Proto {
	return proto.Socks5
}

func (s *socks5Proxy) DialContext(ctx context.Context, metadata *M.Metadata) (net.Conn, error) {
	var c net.Conn
	var err error

	if s.isLocal {
		var d net.Dialer
		c, err = d.DialContext(ctx, "tcp", s.addr)
	} else {
		c, err = dialer.DialContext(ctx, "tcp", s.addr)
	}
	if err != nil {
		return nil, fmt.Errorf("connect to socks5 %s: %w", s.addr, err)
	}

	setKeepAlive(c)

	var user *socks5.User
	if s.user != "" {
		user = &socks5.User{
			Username: s.user,
			Password: s.pass,
		}
	}

	targetAddr := serializeSocksAddr(metadata)
	_, err = socks5.ClientHandshake(c, targetAddr, socks5.CmdConnect, user)
	if err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("socks5 handshake to %s: %w", s.addr, err)
	}

	return c, nil
}

func (s *socks5Proxy) DialUDP(*M.Metadata) (net.PacketConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var c net.Conn
	var err error
	if s.isLocal {
		var d net.Dialer
		c, err = d.DialContext(ctx, "tcp", s.addr)
	} else {
		c, err = dialer.DialContext(ctx, "tcp", s.addr)
	}
	if err != nil {
		return nil, fmt.Errorf("connect to socks5 %s for udp associate: %w", s.addr, err)
	}
	setKeepAlive(c)

	var user *socks5.User
	if s.user != "" {
		user = &socks5.User{
			Username: s.user,
			Password: s.pass,
		}
	}

	targetAddr := socks5.Addr([]byte{socks5.AtypIPv4, 0, 0, 0, 0, 0, 0})
	addr, err := socks5.ClientHandshake(c, targetAddr, socks5.CmdUDPAssociate, user)
	if err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("socks5 udp associate handshake: %w", err)
	}

	var pc net.PacketConn
	if s.isLocal {
		// Crucial fix: For local/loopback SOCKS5 proxy, listen directly on a pure loopback
		// UDP socket without binding to the physical network interface (DefaultDialer).
		// This prevents OS routing from dropping packets destined for 127.0.0.1.
		pc, err = net.ListenPacket("udp", "127.0.0.1:0")
	} else {
		pc, err = dialer.ListenPacket("udp", "")
	}
	if err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("listen udp packet conn: %w", err)
	}

	go func() {
		// RFC 1928: A UDP association terminates when the TCP connection closes.
		_, _ = io.Copy(io.Discard, c)
		_ = c.Close()
		_ = pc.Close()
	}()

	bindAddr := addr.UDPAddr()
	if bindAddr == nil {
		_ = c.Close()
		_ = pc.Close()
		return nil, fmt.Errorf("invalid UDP binding address from socks5 server: %#v", addr)
	}

	if bindAddr.IP.IsUnspecified() {
		udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
		if err != nil {
			_ = c.Close()
			_ = pc.Close()
			return nil, fmt.Errorf("resolve udp address %s: %w", s.addr, err)
		}
		bindAddr.IP = udpAddr.IP
	}

	return &socksPacketConn{PacketConn: pc, rAddr: bindAddr, tcpConn: c}, nil
}

type socksPacketConn struct {
	net.PacketConn
	rAddr   net.Addr
	tcpConn net.Conn
}

func (pc *socksPacketConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	var packet []byte
	var err error
	if ma, ok := addr.(*M.Addr); ok {
		packet, err = socks5.EncodeUDPPacket(serializeSocksAddr(ma.Metadata()), b)
	} else {
		packet, err = socks5.EncodeUDPPacket(socks5.ParseAddr(addr), b)
	}
	if err != nil {
		return 0, err
	}
	return pc.PacketConn.WriteTo(packet, pc.rAddr)
}

func (pc *socksPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, _, err := pc.PacketConn.ReadFrom(b)
	if err != nil {
		return 0, nil, err
	}

	addr, payload, err := socks5.DecodeUDPPacket(b[:n])
	if err != nil {
		return 0, nil, err
	}

	udpAddr := addr.UDPAddr()
	if udpAddr == nil {
		return 0, nil, fmt.Errorf("convert %s to UDPAddr is nil", addr)
	}

	copy(b, payload)
	return len(payload), udpAddr, nil
}

func (pc *socksPacketConn) Close() error {
	_ = pc.tcpConn.Close()
	return pc.PacketConn.Close()
}

func serializeSocksAddr(m *M.Metadata) socks5.Addr {
	return socks5.SerializeAddr("", m.DstIP, m.DstPort)
}

func setKeepAlive(c net.Conn) {
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.SetKeepAlive(true)
		_ = tc.SetKeepAlivePeriod(30 * time.Second)
	}
}
