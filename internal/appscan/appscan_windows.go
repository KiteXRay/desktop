//go:build windows

package appscan

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func scanInstalledAppsOS() ([]AppInfo, error) {
	seenPaths := make(map[string]bool)
	seenNames := make(map[string]bool)
	var result []AppInfo

	addApp := func(name, exePath, icon, desc string) {
		name = strings.TrimSpace(name)
		exePath = strings.TrimSpace(exePath)
		if name == "" || exePath == "" {
			return
		}
		if fi, err := os.Stat(exePath); err != nil || fi.IsDir() {
			return
		}

		nameKey := strings.ToLower(name)
		exeKey := strings.ToLower(exePath)
		if seenNames[nameKey] || seenPaths[exeKey] {
			return
		}
		seenNames[nameKey] = true
		seenPaths[exeKey] = true

		result = append(result, AppInfo{
			Name:        name,
			ExePath:     exePath,
			Icon:        icon,
			Description: desc,
		})
	}

	// 1. Scan Registry Uninstall Keys
	regRoots := []struct {
		k    registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	}

	for _, root := range regRoots {
		k, err := registry.OpenKey(root.k, root.path, registry.READ)
		if err != nil {
			continue
		}

		subKeys, err := k.ReadSubKeyNames(-1)
		k.Close()
		if err != nil {
			continue
		}

		for _, sub := range subKeys {
			sk, err := registry.OpenKey(root.k, root.path+`\`+sub, registry.READ)
			if err != nil {
				continue
			}

			// Skip system components and updates
			if sysComp, _, err := sk.GetIntegerValue("SystemComponent"); err == nil && sysComp == 1 {
				sk.Close()
				continue
			}
			if parent, _, err := sk.GetStringValue("ParentKeyName"); err == nil && parent != "" {
				sk.Close()
				continue
			}

			dispName, _, err := sk.GetStringValue("DisplayName")
			if err != nil || dispName == "" {
				sk.Close()
				continue
			}

			dispIcon, _, _ := sk.GetStringValue("DisplayIcon")
			instLoc, _, _ := sk.GetStringValue("InstallLocation")
			comments, _, _ := sk.GetStringValue("Comments")
			sk.Close()

			// Extract executable from DisplayIcon
			exePath := cleanWindowsExePath(dispIcon)
			if exePath == "" && instLoc != "" {
				// Search install location for executable
				exePath = findMainExeInDir(instLoc)
			}

			if exePath != "" {
				iconURI := findWindowsIcon(dispIcon, exePath, instLoc)
				addApp(dispName, exePath, iconURI, comments)
			}
		}
	}

	// 2. Scan Well-Known User Programs in %LOCALAPPDATA%\Programs & %APPDATA%
	localAppData := os.Getenv("LOCALAPPDATA")
	appData := os.Getenv("APPDATA")
	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")

	wellKnownDirs := []string{
		filepath.Join(localAppData, "Programs"),
		filepath.Join(appData, "Telegram Desktop"),
		filepath.Join(programFiles, "Google", "Chrome", "Application"),
		filepath.Join(programFilesX86, "Google", "Chrome", "Application"),
		filepath.Join(programFiles, "Mozilla Firefox"),
		filepath.Join(programFilesX86, "Mozilla Firefox"),
		filepath.Join(programFiles, "Microsoft", "Edge", "Application"),
		filepath.Join(programFilesX86, "Microsoft", "Edge", "Application"),
		filepath.Join(programFiles, "BraveSoftware", "Brave-Browser", "Application"),
	}

	for _, dir := range wellKnownDirs {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			scanExeInDir(dir, addApp, 2)
		}
	}

	return result, nil
}

var wellKnownNames = map[string]string{
	"chrome":   "Google Chrome",
	"msedge":   "Microsoft Edge",
	"firefox":  "Mozilla Firefox",
	"brave":    "Brave Browser",
	"telegram": "Telegram Desktop",
	"code":     "Visual Studio Code",
	"discord":  "Discord",
	"steam":    "Steam",
	"spotify":  "Spotify",
	"slack":    "Slack",
}

func cleanWindowsExePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// If quoted, extract inside quotes
	if strings.HasPrefix(raw, `"`) {
		endIdx := strings.Index(raw[1:], `"`)
		if endIdx != -1 {
			candidate := raw[1 : 1+endIdx]
			if strings.HasSuffix(strings.ToLower(candidate), ".exe") {
				if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
					return candidate
				}
			}
		}
	}
	// Strip icon index (e.g. "C:\Program Files\app.exe,0")
	if idx := strings.LastIndex(raw, ","); idx > 0 {
		raw = strings.TrimSpace(raw[:idx])
	}
	raw = strings.Trim(raw, `"'`)
	if strings.HasSuffix(strings.ToLower(raw), ".exe") {
		if fi, err := os.Stat(raw); err == nil && !fi.IsDir() {
			return raw
		}
	}
	// Check if there are trailing flags (e.g. app.exe /flag)
	if idx := strings.Index(strings.ToLower(raw), ".exe"); idx != -1 {
		candidate := raw[:idx+4]
		candidate = strings.Trim(candidate, `"'`)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate
		}
	}
	return ""
}

func findMainExeInDir(dir string) string {
	dir = strings.Trim(strings.TrimSpace(dir), `"'`)
	if dir == "" {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".exe") {
			nameLower := strings.ToLower(entry.Name())
			// Skip uninstallers and helpers
			if strings.Contains(nameLower, "unins") || strings.Contains(nameLower, "setup") || strings.Contains(nameLower, "update") {
				continue
			}
			return filepath.Join(dir, entry.Name())
		}
	}
	return ""
}

func scanExeInDir(dir string, addApp func(name, exePath, icon, desc string), maxDepth int) {
	if maxDepth <= 0 {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			scanExeInDir(path, addApp, maxDepth-1)
		} else if strings.HasSuffix(strings.ToLower(entry.Name()), ".exe") {
			baseName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			lower := strings.ToLower(baseName)
			if strings.Contains(lower, "unins") || strings.Contains(lower, "crashpad") || strings.Contains(lower, "helper") || strings.Contains(lower, "setup") || strings.Contains(lower, "update") {
				continue
			}
			displayName := baseName
			if friendly, ok := wellKnownNames[lower]; ok {
				displayName = friendly
			}
			iconURI := findWindowsIcon("", path, dir)
			addApp(displayName, path, iconURI, "")
		}
	}
}
