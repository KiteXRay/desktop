package bridge

import (
	"context"
	"log/slog"
	"net"
	"sync/atomic"
	"syscall"
	"time"

	M "github.com/xjasonlyu/tun2socks/v2/metadata"
	"github.com/xjasonlyu/tun2socks/v2/proxy"
)

var (
	boundInterfaceIndex atomic.Int32
	boundInterfaceName  atomic.Pointer[string]
	boundInterfaceIP    atomic.Pointer[net.IP]
	boundInterfaceGW    atomic.Pointer[net.IP]
)

type bridgeDirectProxy struct {
	proxy.Proxy
}

func newBridgeDirectProxy() proxy.Proxy {
	return &bridgeDirectProxy{
		Proxy: proxy.NewDirect(),
	}
}

func (d *bridgeDirectProxy) DialContext(ctx context.Context, metadata *M.Metadata) (net.Conn, error) {
	ifaceIdx := int(boundInterfaceIndex.Load())
	var ifaceName string
	if p := boundInterfaceName.Load(); p != nil {
		ifaceName = *p
	}

	if ifaceIdx == 0 && ifaceName == "" {
		return d.Proxy.DialContext(ctx, metadata)
	}

	var localIP net.IP
	if p := boundInterfaceIP.Load(); p != nil && *p != nil {
		localIP = *p
	}
	if len(localIP) == 0 && ifaceName != "" {
		if ifc, err := net.InterfaceByName(ifaceName); err == nil {
			localIP = GetInterfaceIPv4(ifc)
		}
	}

	dst := metadata.DestinationAddress()
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			return bindRawConnToInterface(c, network, address, ifaceIdx, ifaceName)
		},
	}
	if len(localIP) > 0 && (!metadata.DstIP.IsValid() || metadata.DstIP.Is4()) {
		dialer.LocalAddr = &net.TCPAddr{IP: localIP}
	}

	network := "tcp"
	if metadata.DstIP.IsValid() {
		if metadata.DstIP.Is4() {
			network = "tcp4"
		} else if metadata.DstIP.Is6() {
			network = "tcp6"
		}
	}
	conn, err := dialer.DialContext(ctx, network, dst)
	if err != nil && dialer.LocalAddr != nil {
		dialer.LocalAddr = nil
		conn, err = dialer.DialContext(ctx, network, dst)
	}
	if err != nil {
		slog.Error("Bridge direct TCP dial failed", "network", network, "dst", dst, "iface", ifaceName, "idx", ifaceIdx, "err", err)
		return nil, err
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetKeepAlive(true)
		_ = tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	return conn, nil
}

func (d *bridgeDirectProxy) DialUDP(metadata *M.Metadata) (net.PacketConn, error) {
	ifaceIdx := int(boundInterfaceIndex.Load())
	var ifaceName string
	if p := boundInterfaceName.Load(); p != nil {
		ifaceName = *p
	}

	if ifaceIdx == 0 && ifaceName == "" {
		return d.Proxy.DialUDP(metadata)
	}

	var localIP net.IP
	if p := boundInterfaceIP.Load(); p != nil && *p != nil {
		localIP = *p
	}
	if len(localIP) == 0 && ifaceName != "" {
		if ifc, err := net.InterfaceByName(ifaceName); err == nil {
			localIP = GetInterfaceIPv4(ifc)
		}
	}

	network := "udp4"
	if metadata.DstIP.IsValid() && metadata.DstIP.Is6() {
		network = "udp6"
	}

	lc := &net.ListenConfig{
		Control: func(netw, address string, c syscall.RawConn) error {
			return bindRawConnToInterface(c, netw, address, ifaceIdx, ifaceName)
		},
	}

	listenAddr := ":0"
	if len(localIP) > 0 && (!metadata.DstIP.IsValid() || metadata.DstIP.Is4()) {
		listenAddr = net.JoinHostPort(localIP.String(), "0")
	}

	pc, err := lc.ListenPacket(context.Background(), network, listenAddr)
	if err != nil {
		pc, err = lc.ListenPacket(context.Background(), network, ":0")
		if err != nil {
			slog.Error("Bridge direct UDP listen failed", "network", network, "addr", listenAddr, "iface", ifaceName, "idx", ifaceIdx, "err", err)
			return nil, err
		}
	}

	return &bridgePacketConn{PacketConn: pc}, nil
}

type bridgePacketConn struct {
	net.PacketConn
}

func (pc *bridgePacketConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	if udpAddr, ok := addr.(*net.UDPAddr); ok {
		n, err := pc.PacketConn.WriteTo(b, udpAddr)
		if err != nil {
			slog.Debug("Bridge direct UDP WriteTo failed", "dst", addr, "err", err)
		}
		return n, err
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr.String())
	if err != nil {
		return 0, err
	}
	n, err := pc.PacketConn.WriteTo(b, udpAddr)
	if err != nil {
		slog.Debug("Bridge direct UDP WriteTo failed", "dst", addr, "err", err)
	}
	return n, err
}
