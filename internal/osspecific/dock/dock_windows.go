//go:build windows

package dock

import (
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/w32"
)

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procLoadImage                = user32.NewProc("LoadImageW")
	procGetModuleHandle          = kernel32.NewProc("GetModuleHandleW")
)

func HideIconInDock() {}

// SetWindowIconFromPNG sets both small (titlebar) and large (taskbar) icons for the application window.
func SetWindowIconFromPNG(pngBytes []byte) {
	go func() {
		currentPID := uint32(os.Getpid())
		var foundHwnds []w32.HWND

		// Wait up to 5 seconds for the window to be created and mapped
		for i := 0; i < 20; i++ {
			time.Sleep(250 * time.Millisecond)
			foundHwnds = foundHwnds[:0]
			cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
				var pid uint32
				procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
				if pid == currentPID {
					foundHwnds = append(foundHwnds, w32.HWND(hwnd))
				}
				return 1
			})
			procEnumWindows.Call(cb, 0)
			if len(foundHwnds) > 0 {
				break
			}
		}

		if len(foundHwnds) == 0 {
			return
		}

		// Try loading from embedded PE resources first (ID 3, then ID 1)
		hModule, _, _ := procGetModuleHandle.Call(0)
		var hSmall, hBig w32.HICON
		if hModule != 0 {
			// IMAGE_ICON = 1, LR_SHARED = 0x8000
			rSmall, _, _ := procLoadImage.Call(hModule, 3, 1, 16, 16, 0x8000)
			if rSmall == 0 {
				rSmall, _, _ = procLoadImage.Call(hModule, 1, 1, 16, 16, 0x8000)
			}
			hSmall = w32.HICON(rSmall)

			rBig, _, _ := procLoadImage.Call(hModule, 3, 1, 32, 32, 0x8000)
			if rBig == 0 {
				rBig, _, _ = procLoadImage.Call(hModule, 1, 1, 32, 32, 0x8000)
			}
			hBig = w32.HICON(rBig)
		}

		// Fallback to creating HICON directly from PNG bytes
		if hSmall == 0 && len(pngBytes) > 0 {
			hSmall, _ = w32.CreateSmallHIconFromImage(pngBytes)
		}
		if hBig == 0 && len(pngBytes) > 0 {
			hBig, _ = w32.CreateLargeHIconFromImage(pngBytes)
		}

		for _, hwnd := range foundHwnds {
			if hSmall != 0 {
				w32.SendMessage(hwnd, w32.WM_SETICON, w32.ICON_SMALL, uintptr(hSmall))
			}
			if hBig != 0 {
				w32.SendMessage(hwnd, w32.WM_SETICON, w32.ICON_BIG, uintptr(hBig))
			}
		}
	}()
}
