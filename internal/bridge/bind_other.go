//go:build !darwin && !linux

package bridge

import (
	"syscall"
)

func bindRawConnToInterface(c syscall.RawConn, network, address string, ifaceIdx int, ifaceName string) error {
	return nil
}
