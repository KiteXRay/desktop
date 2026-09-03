//go:build linux

package appscan

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func scanInstalledAppsOS() ([]AppInfo, error) {
	homeDir, _ := os.UserHomeDir()

	dirs := []string{
		"/usr/share/applications",
		"/usr/local/share/applications",
		"/var/lib/snapd/desktop/applications",
		"/var/lib/flatpak/exports/share/applications",
	}
	if homeDir != "" {
		dirs = append(dirs,
			filepath.Join(homeDir, ".local/share/applications"),
			filepath.Join(homeDir, ".local/share/flatpak/exports/share/applications"),
		)
	}

	seenPaths := make(map[string]bool)
	seenNames := make(map[string]bool)
	var result []AppInfo

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".desktop") {
				continue
			}

			fullPath := filepath.Join(dir, entry.Name())
			app, err := parseDesktopFile(fullPath)
			if err != nil || app == nil {
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

func parseDesktopFile(filePath string) (*AppInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inEntry := false
	isApp := false
	noDisplay := false
	hidden := false
	var name, execCmd, icon, comment string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inEntry = (line == "[Desktop Entry]")
			continue
		}
		if !inEntry || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])

		switch k {
		case "Type":
			if v == "Application" {
				isApp = true
			}
		case "NoDisplay":
			if strings.EqualFold(v, "true") {
				noDisplay = true
			}
		case "Hidden":
			if strings.EqualFold(v, "true") {
				hidden = true
			}
		case "Name":
			if name == "" {
				name = v
			}
		case "Exec":
			if execCmd == "" {
				execCmd = v
			}
		case "Icon":
			if icon == "" {
				icon = v
			}
		case "Comment":
			if comment == "" {
				comment = v
			}
		}
	}

	if !isApp || noDisplay || hidden || name == "" || execCmd == "" {
		return nil, nil
	}

	exePath := resolveExecCommand(execCmd)
	if exePath == "" {
		return nil, nil
	}

	iconURI := findLinuxIcon(icon, exePath)

	return &AppInfo{
		Name:        name,
		ExePath:     exePath,
		Icon:        iconURI,
		Description: comment,
	}, nil
}

func resolveExecCommand(execCmd string) string {
	tokens := strings.Fields(execCmd)
	if len(tokens) == 0 {
		return ""
	}

	bin := tokens[0]
	bin = strings.Trim(bin, `"'`)

	// Handle launcher wrappers like "env VAR=... binary"
	if (bin == "env" || bin == "/usr/bin/env") && len(tokens) > 1 {
		for _, token := range tokens[1:] {
			if !strings.Contains(token, "=") {
				bin = strings.Trim(token, `"'`)
				break
			}
		}
	}

	// Clean any stray desktop field codes (e.g. %f, %u, %F, %U)
	if strings.HasPrefix(bin, "%") {
		return ""
	}

	// Look up in PATH or check if absolute path exists
	if filepath.IsAbs(bin) {
		if fi, err := os.Stat(bin); err == nil && !fi.IsDir() {
			return bin
		}
	}

	if fullPath, err := exec.LookPath(bin); err == nil {
		return fullPath
	}

	return ""
}
