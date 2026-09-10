package bridge

import (
	"errors"
	"log/slog"
	"net"
	"os/exec"
	"runtime"
	"strings"

	"github.com/jackpal/gateway"
	"github.com/xjasonlyu/tun2socks/v2/dialer"
)

func isExcludedInterface(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "tun") ||
		strings.Contains(lower, "tap") ||
		strings.Contains(lower, "wintun") ||
		strings.Contains(lower, "kite") ||
		strings.HasPrefix(lower, "awdl") ||
		strings.HasPrefix(lower, "llw") ||
		strings.HasPrefix(lower, "bridge") ||
		strings.HasPrefix(lower, "gif") ||
		strings.HasPrefix(lower, "stf") ||
		strings.HasPrefix(lower, "anpi")
}

func hasValidIPv4(ifc *net.Interface) bool {
	addrs, err := ifc.Addrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			ip4 := ipNet.IP.To4()
			if ip4 != nil && !ip4.IsLoopback() && !ip4.IsUnspecified() && !ip4.IsLinkLocalUnicast() {
				return true
			}
		}
	}
	return false
}

// GetPhysicalInterface finds the active physical network interface used for internet access,
// skipping virtual interfaces (tun, tap, wintun, kite, awdl, bridge).
func GetPhysicalInterface(customGW ...net.IP) (*net.Interface, error) {
	var gw net.IP
	if len(customGW) > 0 && customGW[0] != nil && !customGW[0].IsUnspecified() {
		gw = customGW[0]
	}

	ifIP, err := gateway.DiscoverInterface()
	if err == nil && ifIP != nil && !ifIP.IsUnspecified() {
		ifaces, _ := net.Interfaces()
		for _, ifc := range ifaces {
			if isExcludedInterface(ifc.Name) {
				continue
			}
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
			if isExcludedInterface(ifc.Name) {
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
			if isExcludedInterface(ifc.Name) || !hasValidIPv4(&ifc) {
				continue
			}
			return &ifc, nil
		}
	}

	return nil, errors.New("no physical network interface found")
}

// GetInterfaceIPv4 returns the first valid non-loopback IPv4 address for the interface.
func GetInterfaceIPv4(ifc *net.Interface) net.IP {
	if ifc == nil {
		return nil
	}
	addrs, err := ifc.Addrs()
	if err != nil {
		return nil
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			if ip4 := ipNet.IP.To4(); ip4 != nil && !ip4.IsLoopback() && !ip4.IsUnspecified() && !ip4.IsLinkLocalUnicast() {
				return ip4
			}
		}
	}
	return nil
}

// SetupBridgeBypass binds direct traffic to the physical network interface so that non-matching
// traffic exits directly through the physical gateway without looping back into the TUN device.
func SetupBridgeBypass(customGW ...net.IP) (func(), error) {
	iface, err := GetPhysicalInterface(customGW...)
	if err != nil {
		return func() {}, err
	}
	slog.Info("Bridge mode binding direct traffic to physical interface", "name", iface.Name, "index", iface.Index)
	boundInterfaceIndex.Store(int32(iface.Index))
	name := iface.Name
	boundInterfaceName.Store(&name)

	if ip := GetInterfaceIPv4(iface); ip != nil {
		boundInterfaceIP.Store(&ip)
		slog.Info("Bridge mode bound interface IP", "ip", ip.String())
	}

	dialer.DefaultDialer.InterfaceIndex.Store(int32(iface.Index))
	dialer.DefaultDialer.InterfaceName.Store(iface.Name)

	var gw net.IP
	if len(customGW) > 0 && customGW[0] != nil && !customGW[0].IsUnspecified() {
		gw = customGW[0]
	}
	if gw == nil {
		if g, err := gateway.DiscoverGateway(); err == nil && g != nil && !g.IsUnspecified() {
			gw = g
		}
	}
	if gw != nil {
		boundInterfaceGW.Store(&gw)
		addDarwinScopedRoutes(iface.Name, gw)
	}

	return CleanupBridgeBypass, nil
}

// CleanupBridgeBypass resets the dialer interface binding.
func CleanupBridgeBypass() {
	if pName := boundInterfaceName.Load(); pName != nil {
		var gw net.IP
		if pGW := boundInterfaceGW.Load(); pGW != nil && *pGW != nil {
			gw = *pGW
		}
		deleteDarwinScopedRoutes(*pName, gw)
	}

	boundInterfaceIndex.Store(0)
	boundInterfaceName.Store(nil)
	boundInterfaceIP.Store(nil)
	boundInterfaceGW.Store(nil)

	dialer.DefaultDialer.InterfaceIndex.Store(0)
	dialer.DefaultDialer.InterfaceName.Store("")
}

func addDarwinScopedRoutes(ifaceName string, gw net.IP) {
	if runtime.GOOS != "darwin" || ifaceName == "" || gw == nil {
		return
	}
	gwStr := gw.String()
	slog.Info("Adding macOS scoped routes for bridge direct traffic", "iface", ifaceName, "gw", gwStr)
	// Add scoped routes on physical interface
	// so sockets with IP_BOUND_IF route directly through physical gateway without hitting global TUN routes.
	routes := []struct {
		dest string
		mask string
	}{
		{"0.0.0.0", "128.0.0.0"},
		{"128.0.0.0", "128.0.0.0"},
	}
	for _, r := range routes {
		_ = exec.Command("/sbin/route", "-q", "delete", "-net", "-ifscope", ifaceName, r.dest, gwStr, r.mask).Run()
		_ = exec.Command("/sbin/route", "-q", "delete", "-net", "-ifscope", ifaceName, r.dest, r.mask).Run()
		_ = exec.Command("/sbin/route", "-q", "delete", "-net", "-ifscope", ifaceName, r.dest+"/1").Run()

		out, err := exec.Command("/sbin/route", "-q", "add", "-net", "-ifscope", ifaceName, r.dest, gwStr, r.mask).CombinedOutput()
		if err != nil {
			slog.Warn("add darwin scoped route failed, trying cidr syntax", "dest", r.dest, "err", err, "out", strings.TrimSpace(string(out)))
			cidr := r.dest + "/1"
			out2, err2 := exec.Command("/sbin/route", "-q", "add", "-net", "-ifscope", ifaceName, cidr, gwStr).CombinedOutput()
			if err2 != nil {
				slog.Warn("add darwin scoped route with cidr failed too", "cidr", cidr, "err", err2, "out", strings.TrimSpace(string(out2)))
			}
		}
	}
	_ = exec.Command("/sbin/route", "-q", "delete", "-ifscope", ifaceName, "default", gwStr).Run()
	_ = exec.Command("/sbin/route", "-q", "delete", "-ifscope", ifaceName, "default").Run()
	out, err := exec.Command("/sbin/route", "-q", "add", "-ifscope", ifaceName, "default", gwStr).CombinedOutput()
	if err != nil {
		slog.Warn("add darwin scoped default route failed", "iface", ifaceName, "err", err, "out", strings.TrimSpace(string(out)))
	}
}

func deleteDarwinScopedRoutes(ifaceName string, gw net.IP) {
	if runtime.GOOS != "darwin" || ifaceName == "" {
		return
	}
	gwStr := ""
	if gw != nil {
		gwStr = gw.String()
	}
	slog.Info("Cleaning up macOS scoped routes for bridge direct traffic", "iface", ifaceName)
	for _, dest := range []string{"0.0.0.0", "128.0.0.0"} {
		if gwStr != "" {
			_ = exec.Command("/sbin/route", "-q", "delete", "-net", "-ifscope", ifaceName, dest, gwStr, "128.0.0.0").Run()
		}
		_ = exec.Command("/sbin/route", "-q", "delete", "-net", "-ifscope", ifaceName, dest, "128.0.0.0").Run()
		_ = exec.Command("/sbin/route", "-q", "delete", "-net", "-ifscope", ifaceName, dest+"/1").Run()
	}
	if gwStr != "" {
		_ = exec.Command("/sbin/route", "-q", "delete", "-ifscope", ifaceName, "default", gwStr).Run()
	}
	_ = exec.Command("/sbin/route", "-q", "delete", "-ifscope", ifaceName, "default").Run()
}

