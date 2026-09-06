//go:build !linux && !windows

package bridge

import (
	"os/exec"
	"strings"
)

// GetRunningProcesses returns the names of running processes using ps on other platforms.
func GetRunningProcesses() ([]string, error) {
	out, err := exec.Command("ps", "-A", "-o", "comm=").Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")
	var procs []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			procs = append(procs, trimmed)
		}
	}
	return procs, nil
}
