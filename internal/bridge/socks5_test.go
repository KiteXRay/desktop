package bridge

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	M "github.com/xjasonlyu/tun2socks/v2/metadata"
	"github.com/xjasonlyu/tun2socks/v2/proxy/proto"
)

func TestIsLoopback(t *testing.T) {
	tests := []struct {
		addr     string
		expected bool
	}{
		{"127.0.0.1:10808", true},
		{"127.0.0.1", true},
		{"127.0.0.2:80", true},
		{"localhost:10808", true},
		{"localhost", true},
		{"[::1]:10808", true},
		{"::1", true},
		{"192.168.1.1:10808", false},
		{"8.8.8.8:53", false},
		{"example.com:10808", false},
		{"10.0.0.1", false},
	}

	for _, tt := range tests {
		res := isLoopback(tt.addr)
		if res != tt.expected {
			t.Errorf("isLoopback(%q) = %v, expected %v", tt.addr, res, tt.expected)
		}
	}
}

// startMockSocks5Server starts a minimal mock SOCKS5 server for TCP connect and UDP associate.
func startMockSocks5Server(t *testing.T) (socksAddr string, shutdown func()) {
	t.Helper()

	tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock socks5 tcp listener: %v", err)
	}

	udpRelay, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		tcpListener.Close()
		t.Fatalf("failed to start mock socks5 udp relay listener: %v", err)
	}

	relayUDPAddr := udpRelay.LocalAddr().(*net.UDPAddr)

	stop := make(chan struct{})

	// UDP Relay Handler: Echoes packets back with SOCKS5 encapsulation
	go func() {
		buf := make([]byte, 65535)
		for {
			select {
			case <-stop:
				return
			default:
			}
			udpRelay.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, clientAddr, err := udpRelay.ReadFrom(buf)
			if err != nil {
				continue
			}

			// Packet received from client. Echo it back to client.
			_, _ = udpRelay.WriteTo(buf[:n], clientAddr)
		}
	}()

	// TCP SOCKS5 Handler
	go func() {
		for {
			conn, err := tcpListener.Accept()
			if err != nil {
				return
			}

			go func(c net.Conn) {
				defer c.Close()

				// 1. Version identifier / method selection
				// Client: [0x05, NMETHODS, METHODS...]
				buf := make([]byte, 256)
				if _, err := io.ReadFull(c, buf[:2]); err != nil {
					return
				}
				nMethods := int(buf[1])
				if _, err := io.ReadFull(c, buf[:nMethods]); err != nil {
					return
				}
				// Reply: [0x05, 0x00] (NO AUTH)
				if _, err := c.Write([]byte{0x05, 0x00}); err != nil {
					return
				}

				// 2. SOCKS request: [0x05, CMD, RSV, ATYP, DST.ADDR, DST.PORT]
				if _, err := io.ReadFull(c, buf[:4]); err != nil {
					return
				}
				cmd := buf[1]
				atyp := buf[3]

				var dstPort uint16
				if atyp == 0x01 { // IPv4
					if _, err := io.ReadFull(c, buf[:4+2]); err != nil {
						return
					}
					dstPort = binary.BigEndian.Uint16(buf[4:6])
				} else if atyp == 0x03 { // Domain
					if _, err := io.ReadFull(c, buf[:1]); err != nil {
						return
					}
					dLen := int(buf[0])
					if _, err := io.ReadFull(c, buf[:dLen+2]); err != nil {
						return
					}
					dstPort = binary.BigEndian.Uint16(buf[dLen : dLen+2])
				} else {
					return
				}

				if cmd == 0x01 { // CONNECT
					// Reply success with BND.ADDR 0.0.0.0:0
					resp := []byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, byte(dstPort >> 8), byte(dstPort & 0xFF)}
					if _, err := c.Write(resp); err != nil {
						return
					}
					// Simple echo for TCP
					_, _ = io.Copy(c, c)
				} else if cmd == 0x03 { // UDP ASSOCIATE
					// Reply success with UDP relay address
					ip4 := relayUDPAddr.IP.To4()
					if ip4 == nil {
						ip4 = net.IPv4(127, 0, 0, 1).To4()
					}
					resp := []byte{
						0x05, 0x00, 0x00, 0x01,
						ip4[0], ip4[1], ip4[2], ip4[3],
						byte(relayUDPAddr.Port >> 8), byte(relayUDPAddr.Port & 0xFF),
					}
					if _, err := c.Write(resp); err != nil {
						return
					}
					// Keep TCP open until client disconnects
					bufDiscard := make([]byte, 128)
					for {
						if _, err := c.Read(bufDiscard); err != nil {
							break
						}
					}
				}
			}(conn)
		}
	}()

	shutdown = func() {
		close(stop)
		_ = tcpListener.Close()
		_ = udpRelay.Close()
	}

	return tcpListener.Addr().String(), shutdown
}

func TestSocks5Proxy_DialContext(t *testing.T) {
	socksAddr, shutdown := startMockSocks5Server(t)
	defer shutdown()

	px, err := newSocks5Proxy(socksAddr, "", "")
	if err != nil {
		t.Fatalf("newSocks5Proxy failed: %v", err)
	}

	meta := &M.Metadata{
		Network: M.TCP,
		DstIP:   netip.MustParseAddr("127.0.0.1"),
		DstPort: 9999,
	}

	conn, err := px.DialContext(context.Background(), meta)
	if err != nil {
		t.Fatalf("DialContext failed: %v", err)
	}
	defer conn.Close()

	msg := []byte("hello tcp")
	if _, err := conn.Write(msg); err != nil {
		t.Fatalf("conn.Write failed: %v", err)
	}

	reply := make([]byte, len(msg))
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("io.ReadFull failed: %v", err)
	}

	if string(reply) != string(msg) {
		t.Fatalf("expected echo %q, got %q", string(msg), string(reply))
	}
}

func TestSocks5Proxy_DialUDP(t *testing.T) {
	socksAddr, shutdown := startMockSocks5Server(t)
	defer shutdown()

	px, err := newSocks5Proxy(socksAddr, "", "")
	if err != nil {
		t.Fatalf("newSocks5Proxy failed: %v", err)
	}

	targetIP := netip.MustParseAddr("198.51.100.1")
	targetPort := uint16(27015)
	meta := &M.Metadata{
		Network: M.UDP,
		SrcPort: 54321,
		DstIP:   targetIP,
		DstPort: targetPort,
	}

	pc, err := px.DialUDP(meta)
	if err != nil {
		t.Fatalf("DialUDP failed: %v", err)
	}
	defer pc.Close()

	payload := []byte("discovery game match packet")
	targetAddr := &net.UDPAddr{IP: targetIP.AsSlice(), Port: int(targetPort)}

	// Send UDP packet via SOCKS5 proxy
	n, err := pc.WriteTo(payload, targetAddr)
	if err != nil {
		t.Fatalf("pc.WriteTo failed: %v", err)
	}
	if n == 0 {
		t.Fatalf("pc.WriteTo wrote 0 bytes")
	}

	// Read echoed response
	recvBuf := make([]byte, 1024)
	pc.SetReadDeadline(time.Now().Add(2 * time.Second))
	nRecv, fromAddr, err := pc.ReadFrom(recvBuf)
	if err != nil {
		t.Fatalf("pc.ReadFrom failed: %v", err)
	}

	if string(recvBuf[:nRecv]) != string(payload) {
		t.Fatalf("expected echoed payload %q, got %q", string(payload), string(recvBuf[:nRecv]))
	}

	udpFrom, ok := fromAddr.(*net.UDPAddr)
	if !ok {
		t.Fatalf("expected *net.UDPAddr fromAddr, got %T", fromAddr)
	}
	if !udpFrom.IP.Equal(targetIP.AsSlice()) || udpFrom.Port != int(targetPort) {
		t.Fatalf("expected from %s:%d, got %v", targetIP, targetPort, fromAddr)
	}
}

func TestBridgeDialer_Routing(t *testing.T) {
	socksAddr, shutdown := startMockSocks5Server(t)
	defer shutdown()

	rules := []BridgeRule{
		{
			ID:          "rule-the-finals",
			Pattern:     "Discovery*.exe",
			ProxyTarget: "socks5://" + socksAddr,
			Enabled:     true,
		},
	}

	bd := NewBridgeDialer(socksAddr, func() []BridgeRule { return rules }, nil)

	// Test rule matching
	matched := bd.matchRule("Discovery.exe")
	if matched == nil || matched.ID != "rule-the-finals" {
		t.Fatalf("expected Discovery.exe to match rule-the-finals")
	}

	matchedUnrelated := bd.matchRule("chrome.exe")
	if matchedUnrelated != nil {
		t.Fatalf("expected chrome.exe not to match rule-the-finals")
	}

	// Test proxy retrieval
	px := bd.getProxyForRule(matched)
	if px == nil {
		t.Fatalf("expected non-nil proxy for matched rule")
	}
	if px.Proto() != proto.Socks5 {
		t.Fatalf("expected socks5 proto, got %v", px.Proto())
	}
}
