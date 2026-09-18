package autostart

import (
	"os"
	"path/filepath"
	"strings"
)

// resolveExecutable returns a clean path to the current executable.
// It avoids temporary build paths if possible.
func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return os.Executable()
	}
	if strings.Contains(exe, "/tmp/") || strings.Contains(exe, "\\Temp\\") {
		// Fallback to standard install paths if running in temp
		if _, err := os.Stat("/opt/kite/kite"); err == nil {
			return "/opt/kite/kite", nil
		}
		if _, err := os.Stat("/usr/local/bin/kite"); err == nil {
			return "/usr/local/bin/kite", nil
		}
	}
	return exe, nil
}
