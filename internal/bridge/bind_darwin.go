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
		// Set IP_BOUND_IF on IPv4
		err4 := unix.SetsockoptInt(int(fd), syscall.IPPROTO_IP, unix.IP_BOUND_IF, ifaceIdx)
		// Set IPV6_BOUND_IF on IPv6
		err6 := unix.SetsockoptInt(int(fd), syscall.IPPROTO_IPV6, unix.IPV6_BOUND_IF, ifaceIdx)

		// Record error only if both failed (a pure IPv4 socket will return ENOPROTOOPT on IPPROTO_IPV6, which is normal)
		if err4 != nil && err6 != nil {
			innerErr = err4
		}
	})
	if innerErr != nil {
		return innerErr
	}
	return err
}
