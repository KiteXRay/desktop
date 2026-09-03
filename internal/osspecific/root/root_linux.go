package root

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
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

// GetPrivilegeFixCommand returns the executable path and the recommended shell command to set all needed capabilities.
func GetPrivilegeFixCommand() (string, string) {
	exePath, err := os.Executable()
	if err != nil {
		exePath = "/opt/kite/kite"
	}
	cmd := fmt.Sprintf("sudo setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip %s", exePath)
	return exePath, cmd
}

// GrantPrivilegesViaPkexec invokes polkit pkexec to grant capabilities to the current executable.
func GrantPrivilegesViaPkexec() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	cmd := exec.Command("pkexec", "setcap", "cap_net_raw,cap_net_admin,cap_net_bind_service+eip", exePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pkexec failed: %s (%w)", string(out), err)
	}
	return nil
}
