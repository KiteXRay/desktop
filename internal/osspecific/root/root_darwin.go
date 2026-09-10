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
		if real, err := filepath.EvalSymlinks(tunnelBin); err == nil {
			tunnelBin = real
		}

		asScript := `on run argv
    set targetBin to item 1 of argv
    set scriptText to "/usr/bin/xattr -d com.apple.quarantine " & quoted form of targetBin & " 2>/dev/null || true; /usr/sbin/chown root:wheel " & quoted form of targetBin & " && /bin/chmod 4755 " & quoted form of targetBin
    do shell script scriptText with administrator privileges
end run`
		cmd := exec.Command("osascript", "-e", asScript, tunnelBin)
		out, err := cmd.CombinedOutput()
		if err == nil {
			if has, _ := HasNetworkPrivileges(); has {
				return nil
			}
		}

		// Fallback: If "do shell script with administrator privileges" was blocked by TCC/SIP,
		// launch AppleScript instructing Terminal.app to run the sudo command directly.
		termScript := `on run argv
    set targetBin to item 1 of argv
    set scriptText to "sudo /usr/bin/xattr -d com.apple.quarantine " & quoted form of targetBin & " 2>/dev/null || true; sudo /usr/sbin/chown root:wheel " & quoted form of targetBin & " && sudo /bin/chmod 4755 " & quoted form of targetBin
    tell application "Terminal"
        activate
        do script scriptText
    end tell
end run`
		termCmd := exec.Command("osascript", "-e", termScript, tunnelBin)
		if termErr := termCmd.Run(); termErr == nil {
			for i := 0; i < 6; i++ {
				time.Sleep(500 * time.Millisecond)
				if has, _ := HasNetworkPrivileges(); has {
					return nil
				}
			}
			return errors.New("opened Terminal to grant privileges. Please enter your password in Terminal and click 'Check Again'")
		}

		outStr := strings.TrimSpace(string(out))
		if outStr == "" {
			return fmt.Errorf("authentication cancelled or failed: %w", err)
		}
		return fmt.Errorf("elevation failed: %s (%w). Run in terminal: sudo chown root:wheel %q && sudo chmod 4755 %q", outStr, err, tunnelBin, tunnelBin)
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
