//go:build linux

package route

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

func addDeleteRoutes(options Opts, delete bool) error {
	for _, route := range options.Routes {
		if err := addDeleteRoute(options.IfName, &options.Gateway, route, delete); err != nil {
			return fmt.Errorf("add route: %w", err)
		}
	}

	return nil
}

func addDeleteRoute(ifname string, gw *net.IP, dst *Addr, delete bool) error {
	route := netlink.Route{
		Dst:      (*net.IPNet)(dst),
		Priority: 1,
	}
	if ifname == "" {
		route.Gw = *gw
		if rList, err := netlink.RouteGet(*gw); err == nil && len(rList) > 0 {
			route.LinkIndex = rList[0].LinkIndex
		}
	} else {
		ifc, err := net.InterfaceByName(ifname)
		if err != nil {
			return err
		}
		route.LinkIndex = ifc.Index
	}

	if delete {
		_ = netlink.RouteDel(&route)
		return nil
	}

	if err := netlink.RouteAdd(&route); err != nil {
		return fmt.Errorf("failed to update %s route to (%q)-%s: %s", dst, route.LinkIndex, gw, err)
	}
	return nil
}
