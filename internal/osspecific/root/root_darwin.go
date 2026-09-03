package root

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

func PromptRootAccess() {
	if !hasPermissions() {
		runItselfAsRoot()
	}
}

func runItselfAsRoot() {
	exe, err := os.Executable()
	if err != nil {
		panic(fmt.Errorf("could not get executable path: %v", err))
	}
	args := strings.Join(append(os.Args[1:], appendFlag), " ")
	prompt := "Kite requires administrative privileges to configure VPN network interfaces."
	script := fmt.Sprintf("do shell script \"%s %s\" with prompt \"%s\" with administrator privileges", exe, args, prompt)
	cmd := exec.Command("osascript", "-e", script)
	err = cmd.Run()
	if err != nil {
		slog.Error("failed to elevate on darwin", "error", err)
		return
	}
	os.Exit(0)
}

func HasNetworkPrivileges() (bool, error) {
	if os.Geteuid() == 0 {
		return true, nil
	}
	return false, errors.New("administrative privileges required on macOS")
}

func GetPrivilegeFixCommand() (string, string) {
	exePath, _ := os.Executable()
	return exePath, "sudo " + exePath
}

func GrantPrivilegesViaPkexec() error {
	return nil
}

func GrantPrivilegesAndRestart() error {
	return nil
}
