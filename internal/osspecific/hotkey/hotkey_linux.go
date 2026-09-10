//go:build linux

package hotkey

/*
#cgo LDFLAGS: -lX11
#include <X11/Xlib.h>
#include <X11/keysym.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

static int ignoreXErrors(Display *d, XErrorEvent *e) {
    (void)d;
    (void)e;
    return 0;
}

static void initThreads() {
    XInitThreads();
    XSetErrorHandler(ignoreXErrors);
}

static int openDisplayTest() {
    Display *d = XOpenDisplay(NULL);
    if (!d) return 0;
    XCloseDisplay(d);
    return 1;
}

static int getEventType(XEvent *ev) {
    return ev->type;
}

static unsigned int getEventState(XKeyEvent *ev) {
    return ev->state;
}

static unsigned int getEventKeycode(XKeyEvent *ev) {
    return ev->keycode;
}

static void sendWakeup(Display *d, Window w) {
    XClientMessageEvent ev;
    memset(&ev, 0, sizeof(ev));
    ev.type = ClientMessage;
    ev.window = w;
    ev.format = 32;
    XSendEvent(d, w, False, 0, (XEvent *)&ev);
    XFlush(d);
}
*/
import "C"

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"unsafe"
)

func init() {
	C.initThreads()
}

type linuxManager struct {
	mu          sync.RWMutex
	display     *C.Display
	root        C.Window
	dummyWindow C.Window
	callbacks   map[string]func()
	keyCodes    map[string]C.KeyCode
	modMasks    map[string]C.uint
	doneChan    chan struct{}
	closed      bool
}

func NewManager() Manager {
	if os.Getenv("DISPLAY") == "" {
		slog.Info("Global hotkeys: DISPLAY environment variable not set, hotkey listener disabled")
		return &noOpManager{}
	}

	if C.openDisplayTest() == 0 {
		slog.Warn("Global hotkeys: failed to open X11 display, global hotkeys unavailable")
		return &noOpManager{}
	}

	cName := C.CString(os.Getenv("DISPLAY"))
	defer C.free(unsafe.Pointer(cName))
	display := C.XOpenDisplay(cName)
	if display == nil {
		slog.Warn("Global hotkeys: cannot connect to X11 server")
		return &noOpManager{}
	}

	root := C.XDefaultRootWindow(display)
	// Create an unmapped dummy window used solely for thread wakeup
	dummy := C.XCreateSimpleWindow(display, root, 0, 0, 1, 1, 0, 0, 0)
	C.XSelectInput(display, dummy, C.StructureNotifyMask)

	mgr := &linuxManager{
		display:     display,
		root:        root,
		dummyWindow: dummy,
		callbacks:   make(map[string]func()),
		keyCodes:    make(map[string]C.KeyCode),
		modMasks:    make(map[string]C.uint),
		doneChan:    make(chan struct{}),
	}

	go mgr.eventLoop()
	return mgr
}

func (m *linuxManager) eventLoop() {
	defer close(m.doneChan)
	var event C.XEvent

	for {
		m.mu.RLock()
		disp := m.display
		closed := m.closed
		m.mu.RUnlock()

		if disp == nil || closed {
			return
		}

		C.XNextEvent(disp, &event)

		eventType := C.getEventType(&event)
		if eventType == C.ClientMessage {
			// Wakeup message received, exit cleanly
			return
		}

		if eventType == C.KeyPress {
			keyEvent := (*C.XKeyEvent)(unsafe.Pointer(&event))
			kc := C.KeyCode(C.getEventKeycode(keyEvent))
			cleanState := C.getEventState(keyEvent) & (C.ControlMask | C.Mod1Mask | C.ShiftMask | C.Mod4Mask)

			m.mu.RLock()
			for shortcut, cb := range m.callbacks {
				regKc := m.keyCodes[shortcut]
				regMod := m.modMasks[shortcut]
				if kc == regKc && cleanState == regMod {
					slog.Info("Global hotkey triggered", "shortcut", shortcut)
					if cb != nil {
						go cb()
					}
					break
				}
			}
			m.mu.RUnlock()
		}
	}
}

func (m *linuxManager) Register(shortcut string, callback func()) error {
	sc, err := ParseShortcut(shortcut)
	if err != nil {
		return err
	}

	canonical := sc.Normalize()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed || m.display == nil {
		return ErrUnsupportedPlatform
	}

	var mask C.uint
	if sc.Modifiers&ModCtrl != 0 {
		mask |= C.ControlMask
	}
	if sc.Modifiers&ModAlt != 0 {
		mask |= C.Mod1Mask
	}
	if sc.Modifiers&ModShift != 0 {
		mask |= C.ShiftMask
	}
	if sc.Modifiers&ModMeta != 0 {
		mask |= C.Mod4Mask
	}

	cKey := C.CString(sc.Key)
	sym := C.XStringToKeysym(cKey)
	C.free(unsafe.Pointer(cKey))

	if sym == C.NoSymbol {
		cKeyLower := C.CString(string(sc.Key[0] + 32))
		sym = C.XStringToKeysym(cKeyLower)
		C.free(unsafe.Pointer(cKeyLower))
	}

	if sym == C.NoSymbol {
		return fmt.Errorf("unknown X11 keysym for key %s", sc.Key)
	}

	kc := C.XKeysymToKeycode(m.display, sym)
	if kc == 0 {
		return fmt.Errorf("no X11 keycode mapping for key %s", sc.Key)
	}

	numLockMask := C.uint(C.Mod2Mask)
	capsLockMask := C.uint(C.LockMask)

	C.XGrabKey(m.display, C.int(kc), mask, m.root, C.True, C.GrabModeAsync, C.GrabModeAsync)
	C.XGrabKey(m.display, C.int(kc), mask|numLockMask, m.root, C.True, C.GrabModeAsync, C.GrabModeAsync)
	C.XGrabKey(m.display, C.int(kc), mask|capsLockMask, m.root, C.True, C.GrabModeAsync, C.GrabModeAsync)
	C.XGrabKey(m.display, C.int(kc), mask|numLockMask|capsLockMask, m.root, C.True, C.GrabModeAsync, C.GrabModeAsync)
	C.XFlush(m.display)

	m.callbacks[canonical] = callback
	m.keyCodes[canonical] = kc
	m.modMasks[canonical] = mask

	slog.Debug("Registered global X11 hotkey", "shortcut", canonical)
	return nil
}

func (m *linuxManager) Unregister(shortcut string) error {
	sc, err := ParseShortcut(shortcut)
	if err != nil {
		return err
	}
	canonical := sc.Normalize()

	m.mu.Lock()
	defer m.mu.Unlock()

	kc, exists := m.keyCodes[canonical]
	if !exists || m.display == nil {
		return nil
	}
	mask := m.modMasks[canonical]

	numLockMask := C.uint(C.Mod2Mask)
	capsLockMask := C.uint(C.LockMask)

	C.XUngrabKey(m.display, C.int(kc), mask, m.root)
	C.XUngrabKey(m.display, C.int(kc), mask|numLockMask, m.root)
	C.XUngrabKey(m.display, C.int(kc), mask|capsLockMask, m.root)
	C.XUngrabKey(m.display, C.int(kc), mask|numLockMask|capsLockMask, m.root)
	C.XFlush(m.display)

	delete(m.callbacks, canonical)
	delete(m.keyCodes, canonical)
	delete(m.modMasks, canonical)
	return nil
}

func (m *linuxManager) UnregisterAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.display == nil {
		return
	}

	for _, kc := range m.keyCodes {
		C.XUngrabKey(m.display, C.int(kc), C.AnyModifier, m.root)
	}
	C.XFlush(m.display)

	m.callbacks = make(map[string]func())
	m.keyCodes = make(map[string]C.KeyCode)
	m.modMasks = make(map[string]C.uint)
}

func (m *linuxManager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true

	if m.display != nil {
		for _, kc := range m.keyCodes {
			C.XUngrabKey(m.display, C.int(kc), C.AnyModifier, m.root)
		}
		C.XFlush(m.display)

		// Wake up event loop cleanly
		C.sendWakeup(m.display, m.dummyWindow)
	}
	m.mu.Unlock()

	// Wait for event loop to exit before closing display
	<-m.doneChan

	m.mu.Lock()
	if m.display != nil {
		C.XDestroyWindow(m.display, m.dummyWindow)
		C.XCloseDisplay(m.display)
		m.display = nil
	}
	m.mu.Unlock()
	return nil
}
