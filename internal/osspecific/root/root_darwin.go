package root

import (
	"errors"
	"os"
	"os/exec"
)

func PromptRootAccess() {}

func HasNetworkPrivileges() (bool, error) {
	if os.Geteuid() == 0 {
		return true, nil
	}
	return false, errors.New("administrative privileges required for TUN mode on macOS")
}

func GetPrivilegeFixCommand() (string, string) {
	exePath, _ := os.Executable()
	return exePath, "Switch to 'Proxy' mode in Settings/Tray menu, or run with administrative privileges"
}

func GrantPrivilegesViaPkexec() error {
	return nil
}

func GrantPrivilegesAndRestart() error {
	return nil
}

func RelaunchApp(targetExe string, args ...string) error {
	cmd := exec.Command(targetExe, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil
}
