package bridge

import (
	"errors"
	"log/slog"
	"net"
	"strings"

	"github.com/jackpal/gateway"
	"github.com/xjasonlyu/tun2socks/v2/dialer"
)

// GetPhysicalInterface finds the active physical network interface used for internet access,
// skipping virtual interfaces (tun, tap, wintun, kite).
func GetPhysicalInterface(customGW ...net.IP) (*net.Interface, error) {
	var gw net.IP
	if len(customGW) > 0 && customGW[0] != nil && !customGW[0].IsUnspecified() {
		gw = customGW[0]
	}

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

	if gw == nil {
		gwIP, err := gateway.DiscoverGateway()
		if err == nil && gwIP != nil && !gwIP.IsUnspecified() {
			gw = gwIP
		}
	}

	if gw != nil {
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
					if ipNet.Contains(gw) {
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

// SetupBridgeBypass binds direct traffic to the physical network interface so that non-matching
// traffic exits directly through the physical gateway without looping back into the TUN device.
func SetupBridgeBypass(customGW ...net.IP) (func(), error) {
	iface, err := GetPhysicalInterface(customGW...)
	if err != nil {
		return func() {}, err
	}
	slog.Info("Bridge mode binding direct traffic to physical interface", "name", iface.Name, "index", iface.Index)
	dialer.DefaultDialer.InterfaceIndex.Store(int32(iface.Index))
	dialer.DefaultDialer.InterfaceName.Store(iface.Name)

	return CleanupBridgeBypass, nil
}

// CleanupBridgeBypass resets the dialer interface binding.
func CleanupBridgeBypass() {
	dialer.DefaultDialer.InterfaceIndex.Store(0)
	dialer.DefaultDialer.InterfaceName.Store("")
}
