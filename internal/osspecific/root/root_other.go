//go:build !linux && !darwin && !windows

package root

import "log/slog"

func PromptRootAccess() {
	slog.Warn("PromptRootAccess not implemented on this platform, run the program as root manually")
}
