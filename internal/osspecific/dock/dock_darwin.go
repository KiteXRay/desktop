//go:build darwin && cgo

package dock

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

void SaveAppDelegate(void);
void RestoreAppDelegate(void);
void SafeStartSystray(void);
void SetApplicationIconFromPNG(const void* bytes, int length);
*/
import "C"
import "unsafe"

// SafeStartSystray runs systray nativeStart on Cocoa's main thread and preserves Wails's AppDelegate.
func SafeStartSystray() {
	C.SafeStartSystray()
}

// SaveAppDelegate stores Wails's NSApplicationDelegate before third-party libraries (like systray) run.
func SaveAppDelegate() {
	C.SaveAppDelegate()
}

// RestoreAppDelegate restores Wails's NSApplicationDelegate so application lifecycle events,
// window management, dock interactions, and single instance lock remain fully functional.
func RestoreAppDelegate() {
	C.RestoreAppDelegate()
}

func HideIconInDock() {
	// Do not hide the app icon from the macOS Dock
}

// SetWindowIconFromPNG sets the application icon in macOS Dock and Cmd-Tab app switcher.
func SetWindowIconFromPNG(pngBytes []byte) {
	if len(pngBytes) == 0 {
		return
	}
	C.SetApplicationIconFromPNG(unsafe.Pointer(&pngBytes[0]), C.int(len(pngBytes)))
}

