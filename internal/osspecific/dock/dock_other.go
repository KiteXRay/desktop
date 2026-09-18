//go:build (!linux && !darwin && !windows) || (!cgo && !windows)

package dock

import "log/slog"

func HideIconInDock() {
	slog.Warn("hiding dock icon not implemented on this platform")

	return
}

func SetWindowIconFromPNG(pngBytes []byte) {}


