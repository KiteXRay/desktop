//go:build !linux && !windows && !darwin

package appscan

func scanInstalledAppsOS() ([]AppInfo, error) {
	return []AppInfo{}, nil
}
