//go:build darwin && cgo

package dock

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

void SaveAppDelegate(void);
void RestoreAppDelegate(void);
void SafeStartSystray(void);
*/
import "C"

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

func SetWindowIconFromPNG(pngBytes []byte) {}

