//go:build darwin && cgo

package dock

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

static id savedDelegate = nil;

void SaveAppDelegate(void) {
    savedDelegate = [[NSApplication sharedApplication] delegate];
}

void RestoreAppDelegate(void) {
    if (savedDelegate != nil) {
        [[NSApplication sharedApplication] setDelegate:savedDelegate];
    }
}
*/
import "C"

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

