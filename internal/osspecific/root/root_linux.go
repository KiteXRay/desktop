package root

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func PromptRootAccess() {}

// CheckProcessCapabilities tests whether the current process itself has necessary rights
// to open /dev/net/tun and create raw ICMP sockets.
func CheckProcessCapabilities() (bool, error) {
	if os.Geteuid() == 0 {
		return true, nil
	}

	f, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		return false, fmt.Errorf("cannot access /dev/net/tun: %w", err)
	}
	_ = f.Close()

	conn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		var sysErr syscall.Errno
		if errors.As(err, &sysErr) && (sysErr == syscall.EPERM || sysErr == syscall.EACCES) {
			return false, fmt.Errorf("missing network capabilities (CAP_NET_ADMIN / CAP_NET_RAW): %w", err)
		}
	} else {
		_ = conn.Close()
	}

	return true, nil
}

// HasNetworkPrivileges checks whether the process (or kite-tunnel helper) has necessary rights
// to create TUN devices and manage routing rules.
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
	// If current process is already kite-tunnel, test capabilities directly (no exec/recursion)
	if filepath.Base(exePath) == "kite-tunnel" {
		return CheckProcessCapabilities()
	}

	// In main Kite app: check if standalone kite-tunnel helper has capabilities
	if tunnelBin, err := FindTunnelBinary(); err == nil && tunnelBin != exePath {
		cmd := exec.Command(tunnelBin, "--check")
		out, err := cmd.CombinedOutput()
		if err == nil {
			return true, nil
		}
		errMsg := strings.TrimSpace(string(out))
		if errMsg == "" {
			errMsg = err.Error()
		}
		return false, fmt.Errorf("kite-tunnel missing network capabilities (%s)", errMsg)
	}

	return CheckProcessCapabilities()
}

// GetPrivilegeFixCommand returns the canonical executable path and the recommended shell command.
func GetPrivilegeFixCommand() (string, string) {
	targetBin, _ := FindTunnelBinary()
	if targetBin == "" {
		exePath, err := os.Executable()
		if err == nil {
			if realPath, err := filepath.EvalSymlinks(exePath); err == nil {
				targetBin = realPath
			}
		} else {
			targetBin = "/opt/kite/kite"
		}
	}
	cmd := fmt.Sprintf("sudo setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip %s", targetBin)
	return targetBin, cmd
}

// GrantPrivilegesViaPkexec invokes polkit pkexec to grant capabilities to kite-tunnel (or current executable).
func GrantPrivilegesViaPkexec() error {
	targetBin, _ := FindTunnelBinary()
	if targetBin == "" {
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("get executable path: %w", err)
		}
		if realPath, err := filepath.EvalSymlinks(exePath); err == nil {
			targetBin = realPath
		}
	}

	setcapPath, err := exec.LookPath("setcap")
	if err != nil {
		for _, p := range []string{"/usr/sbin/setcap", "/sbin/setcap", "/usr/bin/setcap"} {
			if _, statErr := os.Stat(p); statErr == nil {
				setcapPath = p
				break
			}
		}
	}
	if setcapPath == "" {
		return errors.New("setcap utility not found on system (please install libcap2-bin or libcap)")
	}

	pkexecPath, err := exec.LookPath("pkexec")
	if err != nil {
		return errors.New("polkit (pkexec) not found. Please run the command manually in terminal")
	}

	cmd := exec.Command(pkexecPath, setcapPath, "cap_net_raw,cap_net_admin,cap_net_bind_service+eip", targetBin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(out))
		if outStr == "" {
			return fmt.Errorf("authentication cancelled or failed (%w)", err)
		}
		return fmt.Errorf("pkexec failed: %s (%w)", outStr, err)
	}
	return nil
}

// RelaunchApp waits for the current process to exit before executing targetExe,
// avoiding conflicts with Wails SingleInstanceLock (D-Bus session bus name).
func RelaunchApp(targetExe string, args ...string) error {
	if _, err := os.Stat(targetExe); err != nil {
		if _, errOpt := os.Stat("/opt/kite/kite"); errOpt == nil {
			targetExe = "/opt/kite/kite"
		} else {
			return fmt.Errorf("target binary not found: %w", err)
		}
	}

	pid := os.Getpid()
	script := `
target_pid="$1"
shift
count=0
while kill -0 "$target_pid" 2>/dev/null; do
    sleep 0.05
    count=$((count + 1))
    if [ "$count" -ge 100 ]; then
        kill -9 "$target_pid" 2>/dev/null || true
        break
    fi
done
sleep 0.1
exec "$@"
`
	bashArgs := []string{"-c", script, "kite-relaunch", strconv.Itoa(pid), targetExe}
	bashArgs = append(bashArgs, args...)

	cmd := exec.Command("bash", bashArgs...)
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start relaunch process: %w", err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

// GrantPrivilegesAndRestart runs GrantPrivilegesViaPkexec. If kite-tunnel was targeted,
// capabilities are active immediately for future child processes without restarting Kite.
func GrantPrivilegesAndRestart() error {
	if err := GrantPrivilegesViaPkexec(); err != nil {
		return err
	}

	// If kite-tunnel is present and has privileges, avoid app restart
	if has, _ := HasNetworkPrivileges(); has {
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	if realPath, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = realPath
	}

	return RelaunchApp(exePath, os.Args[1:]...)
}
