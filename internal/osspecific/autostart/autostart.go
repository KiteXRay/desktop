package autostart

import (
	"os"
	"path/filepath"
	"runtime"
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
	if strings.Contains(exe, "/tmp/") || strings.Contains(exe, "\\Temp\\") || strings.Contains(exe, "\\temp\\") {
		// Fallback to standard install paths if running in temp
		if runtime.GOOS == "windows" {
			progFiles := os.Getenv("ProgramFiles")
			if progFiles != "" {
				cand := filepath.Join(progFiles, "Kite", "Kite.exe")
				if _, err := os.Stat(cand); err == nil {
					return cand, nil
				}
				cand2 := filepath.Join(progFiles, "Kite", "kite.exe")
				if _, err := os.Stat(cand2); err == nil {
					return cand2, nil
				}
			}
		}
		if _, err := os.Stat("/opt/kite/kite"); err == nil {
			return "/opt/kite/kite", nil
		}
		if _, err := os.Stat("/usr/local/bin/kite"); err == nil {
			return "/usr/local/bin/kite", nil
		}
	}
	return exe, nil
}
