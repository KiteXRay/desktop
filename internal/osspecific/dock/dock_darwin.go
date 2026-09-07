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

extern void runMainThreadCallback(void* ctx);

static void dispatchOnMainThread(void* ctx) {
    if ([NSThread isMainThread]) {
        runMainThreadCallback(ctx);
    } else {
        dispatch_sync(dispatch_get_main_queue(), ^{
            runMainThreadCallback(ctx);
        });
    }
}
*/
import "C"
import "unsafe"

//export runMainThreadCallback
func runMainThreadCallback(ctx unsafe.Pointer) {
	if ctx == nil {
		return
	}
	fn := *(*func())(ctx)
	if fn != nil {
		fn()
	}
}

// RunOnMainThread executes the given function synchronously on Cocoa's main dispatch queue.
func RunOnMainThread(fn func()) {
	if fn == nil {
		return
	}
	C.dispatchOnMainThread(unsafe.Pointer(&fn))
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

