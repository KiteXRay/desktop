//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

func getAutostartFilePath() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		var err error
		configDir, err = os.UserConfigDir()
		if err != nil || configDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			configDir = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(configDir, "autostart", "kite.desktop"), nil
}

// IsEnabled returns true if Kite is configured to run at system startup on Linux.
func IsEnabled() bool {
	path, err := getAutostartFilePath()
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// SetEnabled enables or disables autostart on Linux via XDG autostart .desktop file.
func SetEnabled(enabled bool) error {
	path, err := getAutostartFilePath()
	if err != nil {
		return err
	}

	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove autostart desktop file: %w", err)
		}
		return nil
	}

	exePath, err := resolveExecutable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create autostart dir: %w", err)
	}

	home, _ := os.UserHomeDir()
	iconPath := "kite"
	if home != "" {
		userIcon := filepath.Join(home, ".local", "share", "icons", "hicolor", "512x512", "apps", "kite.png")
		if _, err := os.Stat(userIcon); err == nil {
			iconPath = userIcon
		} else if _, err := os.Stat("/opt/kite/kite.png"); err == nil {
			iconPath = "/opt/kite/kite.png"
		}
	}

	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Kite
Comment=Fast & Minimal Desktop VPN Client
Exec=%s --autostart
Icon=%s
Terminal=false
Categories=Network;VPN;
StartupWMClass=kite
X-GNOME-Autostart-enabled=true
`, exePath, iconPath)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write autostart desktop file: %w", err)
	}

	return nil
}
