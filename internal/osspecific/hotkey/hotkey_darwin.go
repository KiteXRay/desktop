//go:build darwin

package hotkey

import (
	"log/slog"
	"sync"
)

type darwinManager struct {
	mu        sync.RWMutex
	callbacks map[string]func()
}

func NewManager() Manager {
	slog.Info("Global hotkeys initialized for macOS")
	return &darwinManager{
		callbacks: make(map[string]func()),
	}
}

func (m *darwinManager) Register(shortcut string, callback func()) error {
	sc, err := ParseShortcut(shortcut)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks[sc.Normalize()] = callback
	return nil
}

func (m *darwinManager) Unregister(shortcut string) error {
	sc, err := ParseShortcut(shortcut)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.callbacks, sc.Normalize())
	return nil
}

func (m *darwinManager) UnregisterAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = make(map[string]func())
}

func (m *darwinManager) Close() error {
	m.UnregisterAll()
	return nil
}
