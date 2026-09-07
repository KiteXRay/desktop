//go:build darwin

package clean

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/KiteXRay/desktop/internal/osspecific/root"
)

func clearStuckNetworkOS() error {
	exePath, err := os.Executable()
	if err == nil {
		if realPath, err := filepath.EvalSymlinks(exePath); err == nil {
			exePath = realPath
		}
	}

	if filepath.Base(exePath) != "kite-tunnel" {
		if tunnelBin, err := root.FindTunnelBinary(); err == nil && tunnelBin != exePath {
			cmd := exec.Command(tunnelBin, "--clean")
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
	}

	_ = exec.Command("route", "-n", "delete", "0.0.0.0/1").Run()
	_ = exec.Command("route", "-n", "delete", "128.0.0.0/1").Run()
	return nil
}
