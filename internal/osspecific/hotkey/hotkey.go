package hotkey

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrEmptyShortcut       = errors.New("shortcut cannot be empty")
	ErrInvalidModifier     = errors.New("invalid modifier key")
	ErrMissingKey          = errors.New("shortcut must contain a main key")
	ErrUnsupportedPlatform = errors.New("global hotkeys are not supported on this platform/display server")
)

type Modifier uint32

const (
	ModCtrl Modifier = 1 << iota
	ModAlt
	ModShift
	ModMeta // Windows key / Command key / Super key
)

// Shortcut represents a parsed key combination.
type Shortcut struct {
	Raw       string
	Modifiers Modifier
	Key       string // Uppercase single character or key name (e.g. "K", "C", "F1")
}

// Manager defines the lifecycle and registration of global shortcuts.
type Manager interface {
	Register(shortcut string, callback func()) error
	Unregister(shortcut string) error
	UnregisterAll()
	Close() error
}

// ParseShortcut parses strings like "Ctrl+Shift+K", "Cmd+Shift+C", "Alt+F4".
func ParseShortcut(s string) (*Shortcut, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, ErrEmptyShortcut
	}

	parts := strings.Split(s, "+")
	var mods Modifier
	var mainKey string

	for i, p := range parts {
		part := strings.TrimSpace(p)
		if part == "" {
			continue
		}

		lower := strings.ToLower(part)
		isLast := i == len(parts)-1

		switch lower {
		case "ctrl", "control":
			mods |= ModCtrl
		case "alt", "option", "opt":
			mods |= ModAlt
		case "shift":
			mods |= ModShift
		case "cmd", "command", "super", "win", "windows", "meta":
			mods |= ModMeta
		default:
			if isLast {
				mainKey = strings.ToUpper(part)
			} else {
				return nil, fmt.Errorf("%w: %s", ErrInvalidModifier, part)
			}
		}
	}

	if mainKey == "" {
		return nil, ErrMissingKey
	}

	return &Shortcut{
		Raw:       s,
		Modifiers: mods,
		Key:       mainKey,
	}, nil
}

// NormalizeShortcut returns a canonical string format like "Ctrl+Shift+K".
func (sc *Shortcut) Normalize() string {
	var parts []string
	if sc.Modifiers&ModCtrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if sc.Modifiers&ModAlt != 0 {
		parts = append(parts, "Alt")
	}
	if sc.Modifiers&ModShift != 0 {
		parts = append(parts, "Shift")
	}
	if sc.Modifiers&ModMeta != 0 {
		parts = append(parts, "Super")
	}
	parts = append(parts, sc.Key)
	return strings.Join(parts, "+")
}

type noOpManager struct{}

func (n *noOpManager) Register(string, func()) error { return nil }
func (n *noOpManager) Unregister(string) error       { return nil }
func (n *noOpManager) UnregisterAll()                {}
func (n *noOpManager) Close() error                  { return nil }

