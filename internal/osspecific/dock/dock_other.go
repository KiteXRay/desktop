//go:build (!linux && !darwin) || !cgo

package dock

import "log/slog"

func HideIconInDock() {
	slog.Warn("hiding dock icon not implemented on this platform")

	return
}

func SetWindowIconFromPNG(pngBytes []byte) {}

