package root

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func PromptRootAccess() {}

// HasNetworkPrivileges checks whether the process has necessary rights to create TUN devices
// and manage routing rules (root or CAP_NET_ADMIN / CAP_NET_RAW / CAP_NET_BIND_SERVICE).
func HasNetworkPrivileges() (bool, error) {
	if os.Geteuid() == 0 {
		return true, nil
	}

	// 1. Test opening /dev/net/tun
	f, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		return false, fmt.Errorf("cannot access /dev/net/tun: %w", err)
	}
	_ = f.Close()

	// 2. Test raw socket capability (CAP_NET_RAW / CAP_NET_ADMIN)
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

// GetPrivilegeFixCommand returns the canonical executable path and the recommended shell command.
func GetPrivilegeFixCommand() (string, string) {
	exePath, err := os.Executable()
	if err == nil {
		if realPath, err := filepath.EvalSymlinks(exePath); err == nil {
			exePath = realPath
		}
	} else {
		exePath = "/opt/kite/kite"
	}
	cmd := fmt.Sprintf("killall kite 2>/dev/null; sudo setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip %s && %s &", exePath, exePath)
	return exePath, cmd
}

// GrantPrivilegesAndRestart launches a detached background script that waits for the current
// process to exit (to avoid 'Text file busy'), runs pkexec setcap, and relaunches Kite.
func GrantPrivilegesAndRestart() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	if realPath, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = realPath
	}

	pid := os.Getpid()

	script := fmt.Sprintf(`
target_pid=%d
target_exe=%q

# Wait up to 5s for the app to exit so the binary is not busy
for i in $(seq 1 50); do
    if ! kill -0 "$target_pid" 2>/dev/null; then
        break
    fi
    sleep 0.1
done

# Ensure binary is not locked by another process
fuser -k -TERM "$target_exe" 2>/dev/null || true
pkill -TERM -f "$target_exe" 2>/dev/null || true
sleep 0.2

# Grant capabilities using graphical Polkit prompt (pkexec)
if command -v pkexec >/dev/null 2>&1; then
    pkexec setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip "$target_exe"
elif command -v sudo >/dev/null 2>&1; then
    sudo setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip "$target_exe"
fi

# Relaunch the application
nohup "$target_exe" >/dev/null 2>&1 &
`, pid, exePath)

	cmd := exec.Command("bash", "-c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	return cmd.Start()
}

// GrantPrivilegesViaPkexec is maintained for backward compatibility.
func GrantPrivilegesViaPkexec() error {
	return GrantPrivilegesAndRestart()
}
