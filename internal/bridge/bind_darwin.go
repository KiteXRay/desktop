//go:build darwin

package bridge

import (
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func bindRawConnToInterface(c syscall.RawConn, network, address string, ifaceIdx int, ifaceName string) error {
	if ifaceIdx == 0 && ifaceName != "" {
		if ifc, err := net.InterfaceByName(ifaceName); err == nil {
			ifaceIdx = ifc.Index
		}
	}
	if ifaceIdx == 0 {
		return nil
	}

	var innerErr error
	err := c.Control(func(fd uintptr) {
		switch network {
		case "tcp4", "udp4", "ip4":
			innerErr = unix.SetsockoptInt(int(fd), syscall.IPPROTO_IP, unix.IP_BOUND_IF, ifaceIdx)
		case "tcp6", "udp6", "ip6":
			innerErr = unix.SetsockoptInt(int(fd), syscall.IPPROTO_IPV6, unix.IPV6_BOUND_IF, ifaceIdx)
		default:
			err4 := unix.SetsockoptInt(int(fd), syscall.IPPROTO_IP, unix.IP_BOUND_IF, ifaceIdx)
			err6 := unix.SetsockoptInt(int(fd), syscall.IPPROTO_IPV6, unix.IPV6_BOUND_IF, ifaceIdx)
			if err4 != nil && err6 != nil {
				innerErr = err4
			}
		}
	})
	if innerErr != nil {
		return innerErr
	}
	return err
}
