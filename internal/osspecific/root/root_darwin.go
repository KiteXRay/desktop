package root

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func PromptRootAccess() {}

func CheckProcessCapabilities() (bool, error) {
	if os.Geteuid() == 0 {
		return true, nil
	}
	return false, errors.New("administrative privileges required for TUN mode on macOS")
}

func HasNetworkPrivileges() (bool, error) {
	if os.Geteuid() == 0 {
		return true, nil
	}

	exePath, err := os.Executable()
	if err == nil {
		if realPath, err := filepath.EvalSymlinks(exePath); err == nil {
			exePath = realPath
		}
	}
	if filepath.Base(exePath) == "kite-tunnel" {
		return CheckProcessCapabilities()
	}

	if tunnelBin, err := FindTunnelBinary(); err == nil && tunnelBin != exePath {
		cmd := exec.Command(tunnelBin, "--check")
		if err := cmd.Run(); err == nil {
			return true, nil
		}
	}

	return false, errors.New("administrative privileges required for TUN mode on macOS")
}

func GetPrivilegeFixCommand() (string, string) {
	if tunnelBin, err := FindTunnelBinary(); err == nil && tunnelBin != "" {
		cmd := fmt.Sprintf("sudo chown root:wheel %q && sudo chmod 4755 %q", tunnelBin, tunnelBin)
		return tunnelBin, cmd
	}

	exePath, err := os.Executable()
	if err == nil {
		if realPath, err := filepath.EvalSymlinks(exePath); err == nil {
			exePath = realPath
		}
	} else {
		exePath = "/Applications/Kite.app/Contents/MacOS/Kite"
	}
	cmd := fmt.Sprintf("sudo %q", exePath)
	return exePath, cmd
}

func GrantPrivilegesViaPkexec() error {
	return GrantPrivilegesAndRestart()
}

func GrantPrivilegesAndRestart() error {
	if tunnelBin, err := FindTunnelBinary(); err == nil && tunnelBin != "" {
		homeDir, _ := os.UserHomeDir()
		cfgDir := filepath.Join(homeDir, "Library", "Application Support", "kite")
		asScript := `on run argv
    set targetBin to item 1 of argv
    set cfgDir to item 2 of argv
    set scriptText to "chown root:wheel " & quoted form of targetBin & " && chmod 4755 " & quoted form of targetBin
    if cfgDir is not "" then
        set scriptText to scriptText & " && if [ -d " & quoted form of cfgDir & " ]; then chown -R " & (do shell script "id -u") & ":" & (do shell script "id -g") & " " & quoted form of cfgDir & "; fi"
    end if
    do shell script scriptText with administrator privileges
end run`
		cmd := exec.Command("osascript", "-e", asScript, tunnelBin, cfgDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			outStr := strings.TrimSpace(string(out))
			if outStr == "" {
				return fmt.Errorf("authentication cancelled or failed: %w", err)
			}
			return fmt.Errorf("elevation failed: %s (%w)", outStr, err)
		}

		if has, _ := HasNetworkPrivileges(); has {
			return nil
		}
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	if realPath, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = realPath
	}

	pid := os.Getpid()
	homeDir, _ := os.UserHomeDir()

	// Wait for current process to exit, then exec binary with root privileges.
	// Preserving HOME ensures user settings/subscriptions in ~/Library/Application Support remain intact.
	var argsBuilder strings.Builder
	for _, a := range os.Args[1:] {
		argsBuilder.WriteString(fmt.Sprintf(" %q", a))
	}

	relaunchScript := fmt.Sprintf(
		`pid=%d
count=0
while kill -0 "$pid" 2>/dev/null; do
    sleep 0.05
    count=$((count + 1))
    if [ "$count" -ge 100 ]; then
        kill -9 "$pid" 2>/dev/null || true
        break
    fi
done
sleep 0.1
export HOME=%q
exec %q%s`,
		pid,
		homeDir,
		exePath,
		argsBuilder.String(),
	)

	// AppleScript: authenticate with administrator privileges and launch relaunchScript detached as root
	asScript := `on run argv
    set scriptText to item 1 of argv
    do shell script "/bin/sh -c " & quoted form of scriptText & " >/dev/null 2>&1 &" with administrator privileges
end run`

	cmd := exec.Command("osascript", "-e", asScript, relaunchScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(out))
		if outStr == "" {
			return fmt.Errorf("authentication cancelled or failed: %w", err)
		}
		return fmt.Errorf("elevation failed: %s (%w)", outStr, err)
	}

	go func() {
		time.Sleep(150 * time.Millisecond)
		os.Exit(0)
	}()

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
