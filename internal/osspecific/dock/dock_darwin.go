//go:build darwin && cgo

package dock

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

int
SetActivationPolicy(void) {
    [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
    return 0;
}

*/
import "C"

func HideIconInDock() {
	// Do not hide the app icon from the macOS Dock
}

func SetWindowIconFromPNG(pngBytes []byte) {}

