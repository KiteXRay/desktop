package appscan

import (
	"sort"
	"strings"
)

// AppInfo represents an installed desktop application.
type AppInfo struct {
	Name        string `json:"name"`
	ExePath     string `json:"exePath"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
}

// GetInstalledApps scans and returns installed desktop applications on the current OS.
func GetInstalledApps() ([]AppInfo, error) {
	apps, err := scanInstalledAppsOS()
	if err != nil {
		return nil, err
	}

	// Sort alphabetically by name (case-insensitive)
	sort.Slice(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})

	return apps, nil
}
