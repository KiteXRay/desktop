//go:build darwin

package appscan

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func scanInstalledAppsOS() ([]AppInfo, error) {
	homeDir, _ := os.UserHomeDir()
	appDirs := []string{
		"/Applications",
		"/System/Applications",
	}
	if homeDir != "" {
		appDirs = append(appDirs, filepath.Join(homeDir, "Applications"))
	}

	seenPaths := make(map[string]bool)
	seenNames := make(map[string]bool)
	var result []AppInfo

	for _, dir := range appDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".app") {
				continue
			}

			bundlePath := filepath.Join(dir, entry.Name())
			app := parseMacAppBundle(bundlePath)
			if app == nil {
				continue
			}

			nameKey := strings.ToLower(app.Name)
			exeKey := strings.ToLower(app.ExePath)
			if seenNames[nameKey] || seenPaths[exeKey] {
				continue
			}

			seenNames[nameKey] = true
			seenPaths[exeKey] = true
			result = append(result, *app)
		}
	}

	return result, nil
}

func parseMacAppBundle(bundlePath string) *AppInfo {
	plistPath := filepath.Join(bundlePath, "Contents", "Info.plist")
	plistData, err := os.ReadFile(plistPath)
	if err != nil {
		return nil
	}

	content := string(plistData)
	appName := extractPlistString(content, "CFBundleDisplayName")
	if appName == "" {
		appName = extractPlistString(content, "CFBundleName")
	}
	if appName == "" {
		appName = strings.TrimSuffix(filepath.Base(bundlePath), ".app")
	}

	exeName := extractPlistString(content, "CFBundleExecutable")
	if exeName == "" {
		exeName = appName
	}
	exePath := filepath.Join(bundlePath, "Contents", "MacOS", exeName)
	if fi, err := os.Stat(exePath); err != nil || fi.IsDir() {
		return nil
	}

	// Icon
	iconFile := extractPlistString(content, "CFBundleIconFile")
	if iconFile == "" {
		iconFile = "AppIcon"
	}
	if !strings.HasSuffix(iconFile, ".icns") {
		iconFile += ".icns"
	}

	icnsPath := filepath.Join(bundlePath, "Contents", "Resources", iconFile)
	iconURI := ""
	if pngData := extractPNGFromICNS(icnsPath); len(pngData) > 0 {
		iconURI = fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(pngData))
	}

	return &AppInfo{
		Name:    appName,
		ExePath: exePath,
		Icon:    iconURI,
	}
}

func extractPlistString(content, key string) string {
	keyTag := "<key>" + key + "</key>"
	idx := strings.Index(content, keyTag)
	if idx == -1 {
		return ""
	}
	rest := content[idx+len(keyTag):]
	strTag := "<string>"
	sIdx := strings.Index(rest, strTag)
	if sIdx == -1 || sIdx > 200 {
		return ""
	}
	eIdx := strings.Index(rest[sIdx:], "</string>")
	if eIdx == -1 {
		return ""
	}
	return strings.TrimSpace(rest[sIdx+len(strTag) : sIdx+eIdx])
}

func extractPNGFromICNS(icnsPath string) []byte {
	data, err := os.ReadFile(icnsPath)
	if err != nil || len(data) < 8 || string(data[:4]) != "icns" {
		return nil
	}

	offset := 8
	var bestPNG []byte
	pngMagic := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

	for offset+8 <= len(data) {
		chunkLen := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
		if chunkLen < 8 || offset+chunkLen > len(data) {
			break
		}
		chunkData := data[offset+8 : offset+chunkLen]
		if bytes.HasPrefix(chunkData, pngMagic) {
			if len(chunkData) > len(bestPNG) && len(chunkData) <= 512*1024 {
				bestPNG = chunkData
			}
		}
		offset += chunkLen
	}
	return bestPNG
}
