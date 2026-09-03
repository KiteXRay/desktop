//go:build windows

package appscan

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	modShell32 = syscall.NewLazyDLL("shell32.dll")
	modUser32  = syscall.NewLazyDLL("user32.dll")
	modGdiplus = syscall.NewLazyDLL("gdiplus.dll")

	procSHGetFileInfoW            = modShell32.NewProc("SHGetFileInfoW")
	procDestroyIcon               = modUser32.NewProc("DestroyIcon")
	procPrivateExtractIconsW      = modUser32.NewProc("PrivateExtractIconsW")
	procGdiplusStartup            = modGdiplus.NewProc("GdiplusStartup")
	procGdiplusShutdown           = modGdiplus.NewProc("GdiplusShutdown")
	procGdipCreateBitmapFromHICON = modGdiplus.NewProc("GdipCreateBitmapFromHICON")
	procGdipSaveImageToFile       = modGdiplus.NewProc("GdipSaveImageToFile")
	procGdipDisposeImage          = modGdiplus.NewProc("GdipDisposeImage")

	gdiplusOnce  sync.Once
	gdiplusToken uintptr
)

type shFileInfoW struct {
	HIcon         uintptr
	IIcon         int32
	DwAttributes  uint32
	SzDisplayName [260]uint16
	SzTypeName    [80]uint16
}

type gdiplusStartupInput struct {
	GdiplusVersion           uint32
	DebugEventCallback       uintptr
	SuppressBackgroundThread int32
	SuppressExternalCodecs   int32
}

type winGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// 557cf406-1a04-11d3-9a73-0000f81ef32e (GDI+ PNG Encoder CLSID)
var clsidPNG = winGUID{
	Data1: 0x557cf406,
	Data2: 0x1a04,
	Data3: 0x11d3,
	Data4: [8]byte{0x9a, 0x73, 0x00, 0x00, 0xf8, 0x1e, 0xf3, 0x2e},
}

func initGdiplus() {
	gdiplusOnce.Do(func() {
		input := gdiplusStartupInput{GdiplusVersion: 1}
		_, _, _ = procGdiplusStartup.Call(
			uintptr(unsafe.Pointer(&gdiplusToken)),
			uintptr(unsafe.Pointer(&input)),
			0,
		)
	})
}

func findWindowsIcon(dispIcon, exePath, instLoc string) string {
	initGdiplus()

	// 1. If dispIcon points directly to a .ico or .png file, read directly
	if dispIcon != "" {
		cleaned := cleanPath(dispIcon)
		ext := strings.ToLower(filepath.Ext(cleaned))
		if ext == ".ico" || ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			if data, err := os.ReadFile(cleaned); err == nil && len(data) > 0 && len(data) <= 512*1024 {
				return encodeDataURI(cleaned, data)
			}
		}
	}

	// 2. Extract authentic icon from executable or shortcut using Windows API (GDI+ and Shell32)
	candidates := []string{cleanPath(dispIcon), cleanPath(exePath)}
	for _, target := range candidates {
		if target == "" {
			continue
		}
		if dataURI := extractIconFromWinFile(target); dataURI != "" {
			return dataURI
		}
	}

	// 3. Search directory of the executable or install location for .ico / .png
	dirs := []string{}
	cleanedExe := cleanPath(exePath)
	if cleanedExe != "" {
		dirs = append(dirs, filepath.Dir(cleanedExe))
		dirs = append(dirs, filepath.Dir(filepath.Dir(cleanedExe)))
	}
	cleanedInst := cleanPath(instLoc)
	if cleanedInst != "" {
		dirs = append(dirs, cleanedInst)
	}

	baseExe := ""
	if cleanedExe != "" {
		baseExe = strings.TrimSuffix(filepath.Base(cleanedExe), filepath.Ext(cleanedExe))
	}

	candidateNames := []string{
		"app.ico", "icon.ico", "favicon.ico", "logo.ico",
		"app.png", "icon.png", "logo.png",
		"Assets/Logo.png", "Assets/Square44x44Logo.png", "VisualElements/Logo.png",
	}
	if baseExe != "" {
		candidateNames = append([]string{baseExe + ".ico", baseExe + ".png"}, candidateNames...)
	}

	for _, d := range dirs {
		if d == "" || d == "." || d == "/" || d == "\\" {
			continue
		}
		for _, name := range candidateNames {
			p := filepath.Join(d, name)
			if data, err := os.ReadFile(p); err == nil && len(data) > 0 && len(data) <= 512*1024 {
				return encodeDataURI(p, data)
			}
		}
	}

	return ""
}

func cleanPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Extract path from quotes if present
	if strings.HasPrefix(raw, `"`) {
		if endIdx := strings.Index(raw[1:], `"`); endIdx != -1 {
			return raw[1 : 1+endIdx]
		}
	}
	// Strip icon index (e.g. "C:\Program Files\app.exe,0")
	if idx := strings.LastIndex(raw, ","); idx > 0 {
		raw = strings.TrimSpace(raw[:idx])
	}
	return strings.Trim(raw, `"'`)
}

func extractIconFromWinFile(filePath string) string {
	filePath = cleanPath(filePath)
	if filePath == "" {
		return ""
	}
	if _, err := os.Stat(filePath); err != nil {
		return ""
	}

	pUTF16, err := syscall.UTF16PtrFromString(filePath)
	if err != nil {
		return ""
	}

	var hIcon uintptr

	// 1. Try PrivateExtractIconsW first (extract 48x48 icon)
	var phIcon [1]uintptr
	var pIconID [1]uint32
	ret, _, _ := procPrivateExtractIconsW.Call(
		uintptr(unsafe.Pointer(pUTF16)),
		0,  // icon index
		48, // width
		48, // height
		uintptr(unsafe.Pointer(&phIcon[0])),
		uintptr(unsafe.Pointer(&pIconID[0])),
		1,
		0,
	)
	if ret > 0 && phIcon[0] != 0 {
		hIcon = phIcon[0]
	}

	// 2. Fallback to SHGetFileInfoW
	if hIcon == 0 {
		var sfi shFileInfoW
		const SHGFI_ICON = 0x000000100
		const SHGFI_LARGEICON = 0x000000000
		r, _, _ := procSHGetFileInfoW.Call(
			uintptr(unsafe.Pointer(pUTF16)),
			0,
			uintptr(unsafe.Pointer(&sfi)),
			uintptr(unsafe.Sizeof(sfi)),
			SHGFI_ICON|SHGFI_LARGEICON,
		)
		if r != 0 && sfi.HIcon != 0 {
			hIcon = sfi.HIcon
		}
	}

	if hIcon == 0 {
		return ""
	}
	defer procDestroyIcon.Call(hIcon)

	// 3. Convert HICON to PNG via GDI+
	var pBitmap uintptr
	r, _, _ := procGdipCreateBitmapFromHICON.Call(hIcon, uintptr(unsafe.Pointer(&pBitmap)))
	if r != 0 || pBitmap == 0 {
		return ""
	}
	defer procGdipDisposeImage.Call(pBitmap)

	// 4. Save to temp PNG file and read back
	tmpFile, err := os.CreateTemp("", "kite_icon_*.png")
	if err != nil {
		return ""
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	tmpUTF16, err := syscall.UTF16PtrFromString(tmpPath)
	if err != nil {
		return ""
	}

	saveRet, _, _ := procGdipSaveImageToFile.Call(
		pBitmap,
		uintptr(unsafe.Pointer(tmpUTF16)),
		uintptr(unsafe.Pointer(&clsidPNG)),
		0,
	)
	if saveRet != 0 {
		return ""
	}

	pngData, err := os.ReadFile(tmpPath)
	if err != nil || len(pngData) == 0 {
		return ""
	}

	return fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(pngData))
}

func encodeDataURI(path string, data []byte) string {
	mime := "image/png"
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".svg":
		mime = "image/svg+xml"
	case ".ico":
		mime = "image/x-icon"
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	}
	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))
}
