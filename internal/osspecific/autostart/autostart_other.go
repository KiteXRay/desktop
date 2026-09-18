//go:build !linux && !windows && !darwin

package autostart

func IsEnabled() bool {
	return false
}

func SetEnabled(enabled bool) error {
	return nil
}
