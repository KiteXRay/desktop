//go:build windows

package hotkey

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

const (
	MOD_ALT     = 0x0001
	MOD_CONTROL = 0x0002
	MOD_SHIFT   = 0x0004
	MOD_WIN     = 0x0008
	WM_HOTKEY   = 0x0312
	WM_QUIT     = 0x0012
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procRegisterHotKey   = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey = user32.NewProc("UnregisterHotKey")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procPostThreadMsgW   = user32.NewProc("PostThreadMessageW")
	procGetCurrentThread = kernel32.NewProc("GetCurrentThreadId")
)

type MSG struct {
	Hwnd    uintptr
	Message uint32
	Wparam  uintptr
	Lparam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type windowsManager struct {
	mu        sync.RWMutex
	threadID  uint32
	readyChan chan struct{}
	stopChan  chan struct{}
	nextID    int
	callbacks map[int]func()
	shortcuts map[string]int
	closed    bool
}

func NewManager() Manager {
	mgr := &windowsManager{
		readyChan: make(chan struct{}),
		stopChan:  make(chan struct{}),
		nextID:    1,
		callbacks: make(map[int]func()),
		shortcuts: make(map[string]int),
	}

	go mgr.messageLoop()
	<-mgr.readyChan
	return mgr
}

func (m *windowsManager) messageLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	tid, _, _ := procGetCurrentThread.Call()
	m.threadID = uint32(tid)
	close(m.readyChan)

	var msg MSG
	for {
		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0,
			0,
			0,
		)

		if int32(ret) <= 0 || msg.Message == WM_QUIT {
			return
		}

		if msg.Message == WM_HOTKEY {
			hkID := int(msg.Wparam)
			m.mu.RLock()
			cb, ok := m.callbacks[hkID]
			m.mu.RUnlock()

			if ok && cb != nil {
				go cb()
			}
		}
	}
}

func (m *windowsManager) Register(shortcut string, callback func()) error {
	sc, err := ParseShortcut(shortcut)
	if err != nil {
		return err
	}
	canonical := sc.Normalize()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrUnsupportedPlatform
	}

	// Unregister previous if exists
	if oldID, exists := m.shortcuts[canonical]; exists {
		procUnregisterHotKey.Call(0, uintptr(oldID))
		delete(m.callbacks, oldID)
		delete(m.shortcuts, canonical)
	}

	var mod uint32
	if sc.Modifiers&ModCtrl != 0 {
		mod |= MOD_CONTROL
	}
	if sc.Modifiers&ModAlt != 0 {
		mod |= MOD_ALT
	}
	if sc.Modifiers&ModShift != 0 {
		mod |= MOD_SHIFT
	}
	if sc.Modifiers&ModMeta != 0 {
		mod |= MOD_WIN
	}

	vk := uint32(sc.Key[0])
	if len(sc.Key) > 1 && sc.Key[0] == 'F' {
		var fNum int
		if _, err := fmt.Sscanf(sc.Key, "F%d", &fNum); err == nil && fNum >= 1 && fNum <= 24 {
			vk = 0x6F + uint32(fNum)
		}
	}

	id := m.nextID
	m.nextID++

	ret, _, errCode := procRegisterHotKey.Call(
		0,
		uintptr(id),
		uintptr(mod),
		uintptr(vk),
	)

	if ret == 0 {
		return fmt.Errorf("RegisterHotKey failed: %v", errCode)
	}

	m.callbacks[id] = callback
	m.shortcuts[canonical] = id
	slog.Debug("Registered global Windows hotkey", "shortcut", canonical, "id", id)
	return nil
}

func (m *windowsManager) Unregister(shortcut string) error {
	sc, err := ParseShortcut(shortcut)
	if err != nil {
		return err
	}
	canonical := sc.Normalize()

	m.mu.Lock()
	defer m.mu.Unlock()

	id, exists := m.shortcuts[canonical]
	if !exists {
		return nil
	}

	procUnregisterHotKey.Call(0, uintptr(id))
	delete(m.callbacks, id)
	delete(m.shortcuts, canonical)
	return nil
}

func (m *windowsManager) UnregisterAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range m.shortcuts {
		procUnregisterHotKey.Call(0, uintptr(id))
	}
	m.callbacks = make(map[int]func())
	m.shortcuts = make(map[string]int)
}

func (m *windowsManager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true

	for _, id := range m.shortcuts {
		procUnregisterHotKey.Call(0, uintptr(id))
	}

	if m.threadID != 0 {
		procPostThreadMsgW.Call(uintptr(m.threadID), WM_QUIT, 0, 0)
	}
	m.mu.Unlock()
	return nil
}
