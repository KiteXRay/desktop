//go:build linux

package bridge

import (
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func bindRawConnToInterface(c syscall.RawConn, network, address string, ifaceIdx int, ifaceName string) error {
	if ifaceName == "" && ifaceIdx != 0 {
		if ifc, err := net.InterfaceByIndex(ifaceIdx); err == nil {
			ifaceName = ifc.Name
		}
	}
	if ifaceName == "" {
		return nil
	}

	var innerErr error
	err := c.Control(func(fd uintptr) {
		innerErr = unix.BindToDevice(int(fd), ifaceName)
	})
	if innerErr != nil {
		return innerErr
	}
	return err
}
