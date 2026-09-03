//go:build windows

package root

import (
	"log/slog"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

func PromptRootAccess() {
	token := windows.GetCurrentProcessToken()
	if token.IsElevated() {
		return
	}

	slog.Info("Process is not elevated, attempting UAC elevation via runas...")

	exePath, err := os.Executable()
	if err != nil {
		slog.Warn("Failed to determine executable path for elevation", "err", err)
		return
	}

	args := strings.Join(os.Args[1:], " ")

	verbPtr, _ := windows.UTF16PtrFromString("runas")
	exePtr, _ := windows.UTF16PtrFromString(exePath)
	argsPtr, _ := windows.UTF16PtrFromString(args)

	err = windows.ShellExecute(0, verbPtr, exePtr, argsPtr, nil, windows.SW_SHOWNORMAL)
	if err == nil {
		os.Exit(0)
	}

	slog.Warn("UAC elevation failed or was cancelled by user", "err", err)
}
