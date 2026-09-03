//go:build linux

package networkready

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/jackpal/gateway"
	"github.com/vishvananda/netlink"
)

func waitUntilReadyOS(ctx context.Context) bool {
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	var nmObj dbus.BusObject
	if sysBus, err := dbus.SystemBus(); err == nil {
		defer sysBus.Close()
		nmObj = sysBus.Object("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager")
	}

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			// 1. If NetworkManager is present, wait until it is awake and has acquired DHCP (State >= 70)
			if nmObj != nil {
				if variant, err := nmObj.GetProperty("org.freedesktop.NetworkManager.State"); err == nil {
					if state, ok := variant.Value().(uint32); ok && state < 70 {
						// NM_STATE_ASLEEP = 20, DISCONNECTED = 30, CONNECTING = 50, CONNECTED_LOCAL = 60
						continue
					}
				}
			}

			gw, err := gateway.DiscoverGateway()
			if err != nil || gw == nil || gw.IsUnspecified() {
				continue
			}

			// 2. Check if kernel has a valid route to gateway
			rList, err := netlink.RouteGet(gw)
			if err != nil || len(rList) == 0 {
				continue
			}

			// 3. Check that the egress device is a physical/active network link (not tun, not docker, not lo)
			link, err := netlink.LinkByIndex(rList[0].LinkIndex)
			if err != nil {
				continue
			}
			linkName := link.Attrs().Name
			if strings.HasPrefix(linkName, "tun") || strings.HasPrefix(linkName, "docker") || strings.HasPrefix(linkName, "lo") {
				continue
			}
			operState := link.Attrs().OperState
			flags := link.Attrs().Flags
			// Must be UP or UNKNOWN (some ethernet drivers report UNKNOWN when operational)
			if operState == netlink.OperDown || flags&net.FlagUp == 0 {
				continue
			}

			// 4. Verify socket connectivity through the physical interface
			d := net.Dialer{Timeout: 700 * time.Millisecond}
			if conn, err := d.DialContext(ctx, "udp", "1.1.1.1:53"); err == nil {
				_ = conn.Close()
				// Settle briefly for NetworkManager DHCP policy rules and routing to stabilize
				time.Sleep(1 * time.Second)
				slog.Info("Network is ready via physical link", "device", linkName, "gateway", gw)
				return true
			}
			if conn, err := d.DialContext(ctx, "tcp", "8.8.8.8:53"); err == nil {
				_ = conn.Close()
				time.Sleep(1 * time.Second)
				slog.Info("Network is ready via physical link (8.8.8.8)", "device", linkName, "gateway", gw)
				return true
			}
		}
	}
}
