//go:build linux

package appscan

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var snapRevRegex = regexp.MustCompile(`^/snap/([^/]+)/\d+/(.*)$`)

func findLinuxIcon(rawIcon, exePath string) string {
	rawIcon = strings.TrimSpace(rawIcon)
	if rawIcon == "" && exePath == "" {
		return ""
	}

	// 1. Direct path check (including Snap revision normalization)
	if rawIcon != "" {
		resolvedPath := rawIcon
		if snapRevRegex.MatchString(rawIcon) {
			candidate := snapRevRegex.ReplaceAllString(rawIcon, "/snap/$1/current/$2")
			if _, err := os.Stat(candidate); err == nil {
				resolvedPath = candidate
			}
		}

		if filepath.IsAbs(resolvedPath) {
			if data, err := os.ReadFile(resolvedPath); err == nil && len(data) > 0 && len(data) <= 512*1024 {
				return encodeDataURI(resolvedPath, data)
			}
		}
	}

	homeDir, _ := os.UserHomeDir()
	iconSearchDirs := []string{
		"/usr/share/pixmaps",
		"/usr/share/icons/hicolor/48x48/apps",
		"/usr/share/icons/hicolor/64x64/apps",
		"/usr/share/icons/hicolor/32x32/apps",
		"/usr/share/icons/hicolor/128x128/apps",
		"/usr/share/icons/hicolor/256x256/apps",
		"/usr/share/icons/hicolor/scalable/apps",
		"/usr/share/icons/breeze/apps/48",
		"/usr/share/icons/breeze-dark/apps/48",
		"/usr/share/icons/Yaru/48x48/apps",
		"/usr/share/icons/Yaru/32x32/apps",
		"/usr/share/icons/Adwaita/48x48/apps",
		"/usr/share/icons/Adwaita/scalable/apps",
		"/usr/local/share/pixmaps",
		"/usr/local/share/icons/hicolor/48x48/apps",
		"/usr/local/share/icons/hicolor/scalable/apps",
		"/var/lib/flatpak/exports/share/icons/hicolor/48x48/apps",
		"/var/lib/flatpak/exports/share/icons/hicolor/scalable/apps",
		"/var/lib/snapd/desktop/icons",
	}

	if homeDir != "" {
		iconSearchDirs = append(iconSearchDirs,
			filepath.Join(homeDir, ".local/share/icons/hicolor/48x48/apps"),
			filepath.Join(homeDir, ".local/share/icons/hicolor/scalable/apps"),
			filepath.Join(homeDir, ".local/share/icons"),
			filepath.Join(homeDir, ".local/share/pixmaps"),
		)
	}

	// 2. Search common icon directories
	if rawIcon != "" {
		extensions := []string{"", ".png", ".svg", ".xpm", ".ico"}
		variants := []string{rawIcon, strings.ToLower(rawIcon)}

		for _, dir := range iconSearchDirs {
			for _, v := range variants {
				for _, ext := range extensions {
					target := filepath.Join(dir, v+ext)
					if data, err := os.ReadFile(target); err == nil && len(data) > 0 && len(data) <= 512*1024 {
						return encodeDataURI(target, data)
					}
				}
			}
		}
	}

	// 3. Search directory of the executable
	if exePath != "" {
		exeDir := filepath.Dir(exePath)
		baseExe := strings.TrimSuffix(filepath.Base(exePath), filepath.Ext(exePath))
		candidates := []string{
			"icon.png", "icon.svg", "logo.png", "logo.svg",
			baseExe + ".png", baseExe + ".svg",
			"app.png", "app.svg",
		}
		for _, c := range candidates {
			target := filepath.Join(exeDir, c)
			if data, err := os.ReadFile(target); err == nil && len(data) > 0 && len(data) <= 512*1024 {
				return encodeDataURI(target, data)
			}
		}
	}

	return ""
}

func encodeDataURI(path string, data []byte) string {
	mime := "image/png"
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".svg":
		mime = "image/svg+xml"
	case ".ico", ".xpm":
		mime = "image/x-icon"
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	}
	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))
}
