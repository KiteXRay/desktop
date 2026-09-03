package dock

/*
#cgo linux LDFLAGS: -lX11
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <stdlib.h>

static void set_net_wm_icon(unsigned long win_id, const unsigned long *data, int len) {
    Display *d = XOpenDisplay(NULL);
    if (!d) return;
    Atom net_wm_icon = XInternAtom(d, "_NET_WM_ICON", False);
    Atom cardinal = XInternAtom(d, "CARDINAL", False);
    XChangeProperty(d, (Window)win_id, net_wm_icon, cardinal, 32, PropModeReplace, (const unsigned char *)data, len);
    XFlush(d);
    XCloseDisplay(d);
}

static unsigned long find_window_by_pid(unsigned long target_pid) {
    Display *d = XOpenDisplay(NULL);
    if (!d) return 0;
    Atom net_client_list = XInternAtom(d, "_NET_CLIENT_LIST", True);
    Atom net_wm_pid = XInternAtom(d, "_NET_WM_PID", True);
    Atom actual_type;
    int actual_format;
    unsigned long nitems, bytes_after;
    unsigned char *prop = NULL;

    if (XGetWindowProperty(d, DefaultRootWindow(d), net_client_list, 0, 1024, False, XA_WINDOW,
                           &actual_type, &actual_format, &nitems, &bytes_after, &prop) != Success || !prop) {
        XCloseDisplay(d);
        return 0;
    }

    Window *windows = (Window *)prop;
    Window found = 0;
    for (unsigned long i = 0; i < nitems; i++) {
        unsigned char *pid_prop = NULL;
        unsigned long pid_nitems, pid_bytes;
        if (XGetWindowProperty(d, windows[i], net_wm_pid, 0, 1, False, XA_CARDINAL,
                               &actual_type, &actual_format, &pid_nitems, &pid_bytes, &pid_prop) == Success && pid_prop) {
            unsigned long pid = *(unsigned long *)pid_prop;
            XFree(pid_prop);
            if (pid == target_pid) {
                found = windows[i];
                break;
            }
        }
    }
    XFree(prop);
    XCloseDisplay(d);
    return (unsigned long)found;
}
*/
import "C"
import (
	"bytes"
	"image"
	"image/png"
	"os"
	"time"
	"unsafe"
)

func HideIconInDock() {
	return
}

// SetWindowIconFromPNG finds the window belonging to the current process
// and sets the _NET_WM_ICON property with 32-bit ARGB pixels for Linux window managers and docks.
func SetWindowIconFromPNG(pngBytes []byte) {
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return
	}

	go func() {
		pid := C.ulong(os.Getpid())
		var winId C.ulong

		// Wait up to 5 seconds for the window to be created and mapped by GTK
		for i := 0; i < 20; i++ {
			time.Sleep(250 * time.Millisecond)
			winId = C.find_window_by_pid(pid)
			if winId != 0 {
				break
			}
		}

		if winId == 0 {
			return
		}

		// Prepare icon data in multiple resolutions: 16, 32, 48, 64, 128
		sizes := []int{16, 32, 48, 64, 128}
		var cardData []C.ulong

		for _, sz := range sizes {
			scaled := resizeImage(img, sz, sz)
			cardData = append(cardData, C.ulong(sz), C.ulong(sz))
			bounds := scaled.Bounds()
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					r, g, b, a := scaled.At(x, y).RGBA()
					// 8-bit per channel ARGB
					a8 := a >> 8
					r8 := r >> 8
					g8 := g >> 8
					b8 := b >> 8
					argb := (a8 << 24) | (r8 << 16) | (g8 << 8) | b8
					cardData = append(cardData, C.ulong(argb))
				}
			}
		}

		if len(cardData) > 0 {
			C.set_net_wm_icon(winId, (*C.ulong)(unsafe.Pointer(&cardData[0])), C.int(len(cardData)))
		}
	}()
}

func resizeImage(src image.Image, w, h int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	srcBounds := src.Bounds()
	sw := srcBounds.Dx()
	sh := srcBounds.Dy()

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := srcBounds.Min.X + (x * sw) / w
			sy := srcBounds.Min.Y + (y * sh) / h
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}
