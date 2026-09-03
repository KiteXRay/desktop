//go:build windows

package route

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"syscall"
)

func addDeleteRoutes(options Opts, delete bool) error {
	if err := options.Validate(); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	var errs []error

	for _, cidr := range options.Routes {
		if cidr == nil {
			continue
		}

		destIP := cidr.IP.String()
		maskStr := "255.255.255.255"
		if cidr.Mask != nil && len(cidr.Mask) == 4 {
			maskStr = fmt.Sprintf("%d.%d.%d.%d", cidr.Mask[0], cidr.Mask[1], cidr.Mask[2], cidr.Mask[3])
		}

		if delete {
			cmd := exec.Command("route", "delete", destIP, "mask", maskStr)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				HideWindow:    true,
				CreationFlags: 0x08000000,
			}
			if err := cmd.Run(); err != nil {
				errs = append(errs, fmt.Errorf("route delete %s mask %s: %w", destIP, maskStr, err))
			}
			continue
		}

		// Add route
		var cmd *exec.Cmd
		if options.hasGateway() {
			cmd = exec.Command("route", "add", destIP, "mask", maskStr, options.Gateway.String(), "metric", "1")
		} else if options.hasIfName() {
			ifc, err := net.InterfaceByName(options.IfName)
			if err != nil {
				errs = append(errs, fmt.Errorf("find interface %s: %w", options.IfName, err))
				continue
			}
			cmd = exec.Command("route", "add", destIP, "mask", maskStr, "0.0.0.0", "metric", "1", "if", strconv.Itoa(ifc.Index))
		}

		if cmd != nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{
				HideWindow:    true,
				CreationFlags: 0x08000000,
			}
			if err := cmd.Run(); err != nil {
				errs = append(errs, fmt.Errorf("route add %s mask %s: %w", destIP, maskStr, err))
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
