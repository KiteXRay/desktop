//go:build windows

package clean

import (
	"log/slog"
	"os/exec"
	"syscall"

	"golang.zx2c4.com/wintun"
)

func clearStuckNetworkOS() error {
	slog.Info("Cleaning up stuck Windows network routes and TUN adapters")

	// 1. Delete split routes added for VPN routing
	cmd1 := exec.Command("route", "delete", "0.0.0.0", "mask", "128.0.0.0")
	cmd1.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	_ = cmd1.Run()

	cmd2 := exec.Command("route", "delete", "128.0.0.0", "mask", "128.0.0.0")
	cmd2.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	_ = cmd2.Run()

	// 2. Delete lingering Wintun adapter instance if exists
	for _, adapterName := range []string{"kite0", "goxray0"} {
		if adapter, err := wintun.OpenAdapter(adapterName); err == nil && adapter != nil {
			slog.Info("Removing stuck wintun adapter", "name", adapterName)
			_ = adapter.Close()
		}
	}

	return nil
}
