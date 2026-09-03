//go:build !darwin && !linux && !windows

package root

import "log/slog"

func PromptRootAccess() {
	slog.Warn("PromptRootAccess not implemented on this platform, run the program as root manually")
}

func HasNetworkPrivileges() (bool, error) {
	return true, nil
}

func GetPrivilegeFixCommand() (string, string) {
	return "", ""
}

func GrantPrivilegesViaPkexec() error {
	return nil
}

func GrantPrivilegesAndRestart() error {
	return nil
}
