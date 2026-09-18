//go:build darwin && cgo

package dock

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

void SetApplicationIconFromPNG(const void* bytes, int length);
*/
import "C"
import "unsafe"

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

