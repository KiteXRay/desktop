package root

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// FindTunnelBinary searches for the kite-tunnel executable across standard and relative locations.
// It returns the canonical path if found, or the default expected install path and an error.
func FindTunnelBinary() (string, error) {
	// 1. Explicit env var override
	if custom := os.Getenv("KITE_TUNNEL_PATH"); custom != "" {
		if _, err := os.Stat(custom); err == nil {
			if real, err := filepath.EvalSymlinks(custom); err == nil {
				return real, nil
			}
			return custom, nil
		}
	}

	// 2. Alongside current executable
	exePath, err := os.Executable()
	if err == nil {
		if real, err := filepath.EvalSymlinks(exePath); err == nil {
			exePath = real
		}
		dir := filepath.Dir(exePath)
		candidate := filepath.Join(dir, "kite-tunnel")
		if _, err := os.Stat(candidate); err == nil {
			return evalPath(candidate), nil
		}
		// On macOS, if app is in Contents/MacOS/Kite, also check Contents/Helpers/kite-tunnel
		if runtime.GOOS == "darwin" {
			candidateHelper := filepath.Join(dir, "..", "Helpers", "kite-tunnel")
			if _, err := os.Stat(candidateHelper); err == nil {
				return evalPath(filepath.Clean(candidateHelper)), nil
			}
		}
	}

	// 3. In current working directory or build output (development mode)
	for _, rel := range []string{"build/bin/kite-tunnel", "./kite-tunnel", "cmd/kite-tunnel/kite-tunnel"} {
		if _, err := os.Stat(rel); err == nil {
			if abs, err := filepath.Abs(rel); err == nil {
				return evalPath(abs), nil
			}
			return evalPath(rel), nil
		}
	}

	// 4. Standard installation paths
	defaultPath := "/opt/kite/kite-tunnel"
	if runtime.GOOS == "darwin" {
		defaultPath = "/Applications/Kite.app/Contents/MacOS/kite-tunnel"
	}
	if _, err := os.Stat(defaultPath); err == nil {
		return evalPath(defaultPath), nil
	}

	// 5. Check PATH
	if p, err := exec.LookPath("kite-tunnel"); err == nil {
		return evalPath(p), nil
	}

	return defaultPath, errors.New("kite-tunnel executable not found")
}

func evalPath(p string) string {
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	return p
}
