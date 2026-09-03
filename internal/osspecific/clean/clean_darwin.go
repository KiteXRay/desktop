//go:build darwin

package clean

import (
	"os/exec"
)

func clearStuckNetworkOS() error {
	_ = exec.Command("route", "-n", "delete", "0.0.0.0/1").Run()
	_ = exec.Command("route", "-n", "delete", "128.0.0.0/1").Run()
	return nil
}
