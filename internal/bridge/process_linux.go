//go:build linux

package bridge

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GetRunningProcesses returns the names of all currently running processes on Linux.
func GetRunningProcesses() ([]string, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	var procs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// PID directories are numbers
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}

		pidDir := filepath.Join("/proc", entry.Name())
		commBytes, err := os.ReadFile(filepath.Join(pidDir, "comm"))
		if err == nil {
			name := strings.TrimSpace(string(commBytes))
			if name != "" {
				procs = append(procs, name)
			}
		}

		// Also check exe symlink if accessible
		if target, err := os.Readlink(filepath.Join(pidDir, "exe")); err == nil && target != "" {
			procs = append(procs, target)
		}
	}

	return procs, nil
}
