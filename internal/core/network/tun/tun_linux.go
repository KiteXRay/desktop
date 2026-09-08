//go:build linux

package tun

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

func UpInterface(name string, local *net.IPNet, gw net.IP) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("failed to detect %s interface: %s", name, err)
	}

	ipv4Addr := &netlink.Addr{
		IPNet: local,
		Peer:  &net.IPNet{IP: gw, Mask: []byte{0, 0, 0, 0}},
	}
	err = netlink.AddrReplace(link, ipv4Addr)
	if err != nil {
		return fmt.Errorf("failed to set peer address on %s interface: %s", name, err)
	}

	err = netlink.LinkSetUp(link)
	if err != nil {
		return fmt.Errorf("failed to set %s interface up: %s", name, err)
	}

	return nil
}

func (i *Interface) up(local *net.IPNet, gw net.IP) error {
	return UpInterface(i.Name(), local, gw)
}
