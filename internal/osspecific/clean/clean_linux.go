//go:build linux

package clean

import (
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/KiteXRay/desktop/internal/osspecific/root"
	"github.com/vishvananda/netlink"
)

func clearStuckNetworkOS() error {
	exePath, err := os.Executable()
	if err == nil {
		if realPath, err := filepath.EvalSymlinks(exePath); err == nil {
			exePath = realPath
		}
	}

	// If running inside GUI process, delegate to kite-tunnel helper to use its CAP_NET_ADMIN capabilities
	if filepath.Base(exePath) != "kite-tunnel" {
		if tunnelBin, err := root.FindTunnelBinary(); err == nil && tunnelBin != exePath {
			cmd := exec.Command(tunnelBin, "--clean")
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
	}

	var errs []error

	// 1. Delete split routes (0.0.0.0/1 and 128.0.0.0/1) added for TUN routing
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err == nil {
		for _, r := range routes {
			if r.Dst != nil {
				dstStr := r.Dst.String()
				isAppRoute := dstStr == "0.0.0.0/1" || dstStr == "128.0.0.0/1"
				if !isAppRoute && r.Priority == 1 && r.Dst.Mask != nil {
					ones, bits := r.Dst.Mask.Size()
					if ones == 32 && bits == 32 {
						isAppRoute = true
					}
				}
				if isAppRoute {
					slog.Info("Removing stuck route", "dst", dstStr)
					if err := netlink.RouteDel(&r); err != nil {
						errs = append(errs, err)
					}
				}
			}
		}
	}

	// 2. Delete app TUN interfaces (tun0, kite0, goxray0)
	links, err := netlink.LinkList()
	if err == nil {
		for _, l := range links {
			name := l.Attrs().Name
			if name == "tun0" || name == "kite0" || name == "goxray0" {
				slog.Info("Removing stuck TUN interface", "name", name)
				_ = netlink.LinkSetDown(l)
				if err := netlink.LinkDel(l); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	// 3. Command-line fallback if ip tool is present
	_ = exec.Command("ip", "route", "del", "0.0.0.0/1").Run()
	_ = exec.Command("ip", "route", "del", "128.0.0.0/1").Run()
	_ = exec.Command("ip", "link", "delete", "kite0").Run()
	_ = exec.Command("ip", "link", "delete", "goxray0").Run()
	_ = exec.Command("ip", "link", "delete", "tun0").Run()

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
