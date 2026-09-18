//go:build windows

package autostart

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	appName    = "Kite"
	taskName   = "KiteAutostart"
)

func isTaskEnabled() bool {
	cmd := exec.Command("schtasks.exe", "/Query", "/TN", taskName)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run() == nil
}

func isRegistryEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	val, _, err := k.GetStringValue(appName)
	return err == nil && strings.TrimSpace(val) != ""
}

// IsEnabled returns true if Kite is configured in Task Scheduler or Windows Run registry key.
func IsEnabled() bool {
	return isTaskEnabled() || isRegistryEnabled()
}

// SetEnabled enables or disables autostart via Windows Task Scheduler (elevated) and Run registry key.
func SetEnabled(enabled bool) error {
	if !enabled {
		// 1. Remove from Task Scheduler
		delCmd := exec.Command("schtasks.exe", "/Delete", "/TN", taskName, "/F")
		delCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = delCmd.Run()

		// 2. Remove from Registry
		k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
		if err == nil {
			_ = k.DeleteValue(appName)
			k.Close()
		}
		return nil
	}

	exePath, err := resolveExecutable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	// 1. Task Scheduler with HighestAvailable (required because Kite requires Administrator privileges)
	taskCmd := fmt.Sprintf(`"%s" --autostart`, exePath)
	schedCmd := exec.Command("schtasks.exe", "/Create", "/TN", taskName, "/TR", taskCmd, "/SC", "ONLOGON", "/RL", "HIGHEST", "/F")
	schedCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	schedErr := schedCmd.Run()

	// 2. Also register in HKCU Run key as secondary fallback
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err == nil {
		_ = k.SetStringValue(appName, taskCmd)
		k.Close()
	}

	if schedErr != nil && err != nil {
		return fmt.Errorf("failed to configure autostart in task scheduler (%v) and registry (%v)", schedErr, err)
	}

	return nil
}
