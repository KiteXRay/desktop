package root

import (
	"log/slog"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func PromptRootAccess() {
	if !hasPermissions() {
		verb := "runas"
		exe, _ := os.Executable()
		cwd, _ := os.Getwd()
		args := strings.Join(append(os.Args[1:], appendFlag), " ")

		verbPtr, _ := syscall.UTF16PtrFromString(verb)
		exePtr, _ := syscall.UTF16PtrFromString(exe)
		cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
		argPtr, _ := syscall.UTF16PtrFromString(args)

		var showCmd int32 = 1 // SW_NORMAL

		err := windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, showCmd)
		if err != nil {
			slog.Error("failed to elevate on windows", "error", err)
			return
		}
		os.Exit(0)
	}
}

func HasNetworkPrivileges() (bool, error) {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return false, err
	}
	defer token.Close()
	return token.IsElevated(), nil
}

func GetPrivilegeFixCommand() (string, string) {
	exePath, _ := os.Executable()
	return exePath, "Right click Kite and select 'Run as administrator'"
}

func GrantPrivilegesViaPkexec() error {
	return nil
}
