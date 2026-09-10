//go:build !linux && !windows && !darwin

package hotkey

func NewManager() Manager {
	return &noOpManager{}
}
