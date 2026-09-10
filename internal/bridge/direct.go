package bridge

import (
	"context"
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

	dst := metadata.DestinationAddress()
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			return bindRawConnToInterface(c, network, address, ifaceIdx, ifaceName)
		},
	}

	network := "tcp"
	if metadata.Network == M.TCP {
		network = "tcp"
	}
	conn, err := dialer.DialContext(ctx, network, dst)
	if err != nil {
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

	lc := &net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return bindRawConnToInterface(c, network, address, ifaceIdx, ifaceName)
		},
	}

	pc, err := lc.ListenPacket(context.Background(), "udp4", ":0")
	if err != nil {
		pc, err = lc.ListenPacket(context.Background(), "udp", ":0")
		if err != nil {
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
		return pc.PacketConn.WriteTo(b, udpAddr)
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr.String())
	if err != nil {
		return 0, err
	}
	return pc.PacketConn.WriteTo(b, udpAddr)
}
