//go:build windows

package autostart

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	appName    = "Kite"
)

// IsEnabled returns true if Kite is configured in Windows Run registry key.
func IsEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	val, _, err := k.GetStringValue(appName)
	return err == nil && strings.TrimSpace(val) != ""
}

// SetEnabled enables or disables autostart in Windows Run registry key.
func SetEnabled(enabled bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry run key: %w", err)
	}
	defer k.Close()

	if !enabled {
		err := k.DeleteValue(appName)
		if err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("delete registry value: %w", err)
		}
		return nil
	}

	exePath, err := resolveExecutable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	cmd := fmt.Sprintf(`"%s" --autostart`, exePath)
	if err := k.SetStringValue(appName, cmd); err != nil {
		return fmt.Errorf("set registry value: %w", err)
	}

	return nil
}
