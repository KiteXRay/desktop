//go:build darwin

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

const plistLabel = "com.kite.vpn"

func getLaunchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", plistLabel+".plist"), nil
}

// IsEnabled returns true if Kite LaunchAgent plist exists.
func IsEnabled() bool {
	path, err := getLaunchAgentPath()
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// SetEnabled enables or disables autostart on macOS via LaunchAgent.
func SetEnabled(enabled bool) error {
	path, err := getLaunchAgentPath()
	if err != nil {
		return err
	}

	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove LaunchAgent plist: %w", err)
		}
		return nil
	}

	exePath, err := resolveExecutable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>--autostart</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>ProcessType</key>
	<string>Interactive</string>
</dict>
</plist>
`, plistLabel, exePath)

	if err := os.WriteFile(path, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("write LaunchAgent plist: %w", err)
	}

	return nil
}
