package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/KiteXRay/desktop/internal/osspecific/root"
)

type GitHubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

type GitHubRelease struct {
	TagName     string               `json:"tag_name"`
	Name        string               `json:"name"`
	Body        string               `json:"body"`
	HTMLURL     string               `json:"html_url"`
	Draft       bool                 `json:"draft"`
	Prerelease  bool                 `json:"prerelease"`
	Assets      []GitHubReleaseAsset `json:"assets"`
}

type ReleaseInfo struct {
	Available    bool   `json:"available"`
	CurrentVer   string `json:"currentVersion"`
	LatestVer    string `json:"latestVersion"`
	ReleaseTitle string `json:"releaseTitle"`
	ReleaseNotes string `json:"releaseNotes"`
	ReleaseURL   string `json:"releaseUrl"`
	AssetURL     string `json:"assetUrl"`
	AssetName    string `json:"assetName"`
	AssetSize    int64  `json:"assetSize"`
}

func parseVersion(v string) []int {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if idx := strings.IndexAny(v, "-+"); idx != -1 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			break
		}
		nums = append(nums, n)
	}
	return nums
}

func IsNewerVersion(latest, current string) bool {
	lNums := parseVersion(latest)
	cNums := parseVersion(current)

	maxLen := len(lNums)
	if len(cNums) > maxLen {
		maxLen = len(cNums)
	}

	for i := 0; i < maxLen; i++ {
		l := 0
		if i < len(lNums) {
			l = lNums[i]
		}
		c := 0
		if i < len(cNums) {
			c = cNums[i]
		}
		if l > c {
			return true
		}
		if l < c {
			return false
		}
	}
	return false
}

func SelectAsset(assets []GitHubReleaseAsset, goos, goarch string) *GitHubReleaseAsset {
	if len(assets) == 0 {
		return nil
	}
	var bestAsset *GitHubReleaseAsset
	bestScore := -1

	for i := range assets {
		asset := &assets[i]
		name := strings.ToLower(asset.Name)
		score := 0

		switch goos {
		case "windows":
			if !strings.HasSuffix(name, ".exe") && !strings.HasSuffix(name, ".zip") {
				continue
			}
			if strings.Contains(name, "windows") || strings.Contains(name, "win") || strings.HasSuffix(name, ".exe") {
				score += 10
			}
			if strings.Contains(name, "installer") || strings.Contains(name, "setup") {
				score += 20 // prefer installer over raw zip
			}
			if strings.Contains(name, goarch) || (goarch == "amd64" && strings.Contains(name, "x64")) {
				score += 5
			}
		case "linux":
			if strings.HasSuffix(name, ".exe") || strings.HasSuffix(name, ".dmg") {
				continue
			}
			if strings.Contains(name, "linux") {
				score += 10
			}
			if strings.Contains(name, goarch) {
				score += 10
			}
			if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") {
				score += 5
			}
		case "darwin":
			if strings.HasSuffix(name, ".exe") || strings.Contains(name, "linux") {
				continue
			}
			if strings.Contains(name, "macos") || strings.Contains(name, "darwin") || strings.Contains(name, "mac") {
				score += 10
			}
			if strings.Contains(name, "universal") || strings.Contains(name, goarch) {
				score += 5
			}
			if strings.HasSuffix(name, ".dmg") || strings.HasSuffix(name, ".zip") {
				score += 5
			}
		}

		if score > bestScore {
			bestScore = score
			bestAsset = asset
		}
	}

	return bestAsset
}

func CheckForUpdate(ctx context.Context, repo string, currentVersion string) (*ReleaseInfo, error) {
	repoPath := strings.TrimPrefix(repo, "https://github.com/")
	repoPath = strings.TrimPrefix(repoPath, "http://github.com/")
	repoPath = strings.TrimSuffix(repoPath, ".git")
	repoPath = strings.Trim(repoPath, "/")

	if repoPath == "" {
		repoPath = "KiteXRay/desktop"
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repoPath)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Kite-Desktop-Updater")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &ReleaseInfo{
			Available:  false,
			CurrentVer: currentVersion,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	var rel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release json: %w", err)
	}

	available := IsNewerVersion(rel.TagName, currentVersion)
	matchedAsset := SelectAsset(rel.Assets, runtime.GOOS, runtime.GOARCH)

	info := &ReleaseInfo{
		Available:    available,
		CurrentVer:   currentVersion,
		LatestVer:    rel.TagName,
		ReleaseTitle: rel.Name,
		ReleaseNotes: rel.Body,
		ReleaseURL:   rel.HTMLURL,
	}

	if matchedAsset != nil {
		info.AssetURL = matchedAsset.BrowserDownloadURL
		info.AssetName = matchedAsset.Name
		info.AssetSize = matchedAsset.Size
	} else {
		info.AssetURL = rel.HTMLURL
	}

	return info, nil
}

func DownloadFile(ctx context.Context, downloadURL, destPath string, progressFn func(downloaded, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Kite-Desktop-Updater")

	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	total := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 64*1024)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			_, wErr := out.Write(buf[:n])
			if wErr != nil {
				return wErr
			}
			downloaded += int64(n)
			if progressFn != nil {
				progressFn(downloaded, total)
			}
		}
		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			return rErr
		}
	}

	return nil
}

func ExtractTarGz(tarGzPath, destDir string) (string, error) {
	file, err := os.Open(tarGzPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var kiteBinPath string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") {
			continue
		}
		target := filepath.Join(destDir, cleanName)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", err
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return "", err
			}
			outFile.Close()

			if filepath.Base(target) == "kite" {
				kiteBinPath = target
			}
		}
	}

	if kiteBinPath == "" {
		return "", errors.New("kite binary not found in archive")
	}

	return kiteBinPath, nil
}

func ApplyDownloadedUpdate(downloadedFilePath, releaseURL string, onPreQuit func()) error {
	switch runtime.GOOS {
	case "windows":
		if strings.HasSuffix(strings.ToLower(downloadedFilePath), ".exe") {
			slog.Info("Launching Windows installer...", "path", downloadedFilePath)
			cmd := exec.Command(downloadedFilePath)
			if err := cmd.Start(); err != nil {
				return fmt.Errorf("launch installer: %w", err)
			}
			if onPreQuit != nil {
				onPreQuit()
			}
			os.Exit(0)
			return nil
		}
		_ = exec.Command("explorer.exe", "/select,", downloadedFilePath).Start()
		return nil

	case "linux":
		if strings.HasSuffix(downloadedFilePath, ".deb") {
			slog.Info("Installing Debian package via pkexec...", "path", downloadedFilePath)
			cmd := exec.Command("pkexec", "dpkg", "-i", downloadedFilePath)
			out, err := cmd.CombinedOutput()
			if err != nil {
				outStr := strings.TrimSpace(string(out))
				if outStr == "" {
					return fmt.Errorf("authentication cancelled or failed (%w)", err)
				}
				return fmt.Errorf("package installation failed: %s (%w)", outStr, err)
			}
			slog.Info("Deb package installed, restarting Kite...", "path", "/opt/kite/kite")
			if onPreQuit != nil {
				onPreQuit()
			}
			return root.RelaunchApp("/opt/kite/kite")
		}

		if strings.HasSuffix(downloadedFilePath, ".tar.gz") || strings.HasSuffix(downloadedFilePath, ".tgz") {
			tmpExtract := filepath.Join(os.TempDir(), fmt.Sprintf("kite_update_%d", time.Now().UnixNano()))
			if err := os.MkdirAll(tmpExtract, 0755); err != nil {
				return fmt.Errorf("failed to create temp extraction directory: %w", err)
			}
			newBinary, err := ExtractTarGz(downloadedFilePath, tmpExtract)
			if err != nil {
				return fmt.Errorf("failed to extract update archive: %w", err)
			}

			currentExe, errExe := os.Executable()
			if errExe != nil {
				currentExe = "/opt/kite/kite"
			}
			if realPath, err := filepath.EvalSymlinks(currentExe); err == nil {
				currentExe = realPath
			}

			// Check if we can write to currentExe directory without root
			canWriteDirectly := false
			testFile, errTest := os.CreateTemp(filepath.Dir(currentExe), ".write_test_*")
			if errTest == nil {
				_ = testFile.Close()
				_ = os.Remove(testFile.Name())
				canWriteDirectly = true
			}

			if canWriteDirectly {
				// Direct in-place update (e.g. running from user home directory)
				tmpNew := currentExe + ".new"
				_ = os.Remove(tmpNew)
				if errCopy := copyFile(newBinary, tmpNew, 0755); errCopy == nil {
					if errRename := os.Rename(tmpNew, currentExe); errRename == nil {
						slog.Info("In-place update successful, restarting Kite...", "path", currentExe)
						if onPreQuit != nil {
							onPreQuit()
						}
						return root.RelaunchApp(currentExe)
					}
				}
			}

			// System directory (/opt/kite, /usr/local/bin, etc.) requires elevated privileges via pkexec
			installerScript := filepath.Join(tmpExtract, "apply_update.sh")
			scriptContent := fmt.Sprintf(`#!/usr/bin/env bash
set -e
PKG_DIR=%q
NEW_BIN=%q
CUR_EXE=%q

mkdir -p /opt/kite

# Atomically replace /opt/kite/kite without truncating running inode (avoids Text file busy)
cp -f "$NEW_BIN" /opt/kite/kite.new
chmod 755 /opt/kite/kite.new
mv -f /opt/kite/kite.new /opt/kite/kite

# Assign network capabilities
if command -v setcap >/dev/null 2>&1; then
    setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip /opt/kite/kite 2>/dev/null || true
fi

# Install icons and desktop launcher if present in archive
if [ -f "$PKG_DIR/kite.png" ]; then
    cp -f "$PKG_DIR/kite.png" /opt/kite/kite.png
    chmod 644 /opt/kite/kite.png
    mkdir -p /usr/share/icons/hicolor/512x512/apps /usr/share/pixmaps
    cp -f "$PKG_DIR/kite.png" /usr/share/icons/hicolor/512x512/apps/kite.png 2>/dev/null || true
    cp -f "$PKG_DIR/kite.png" /usr/share/pixmaps/kite.png 2>/dev/null || true
fi

if [ -f "$PKG_DIR/grant_privileges.sh" ]; then
    cp -f "$PKG_DIR/grant_privileges.sh" /opt/kite/grant_privileges.sh
    chmod 755 /opt/kite/grant_privileges.sh
fi

if [ -f "$PKG_DIR/uninstall.sh" ]; then
    cp -f "$PKG_DIR/uninstall.sh" /opt/kite/uninstall.sh
    chmod 755 /opt/kite/uninstall.sh
fi

# Create CLI symlink
mkdir -p /usr/local/bin
ln -sf /opt/kite/kite /usr/local/bin/kite

# Create desktop entry
mkdir -p /usr/share/applications
cat << 'DESKTOP_EOF' > /usr/share/applications/kite.desktop
[Desktop Entry]
Name=Kite
Comment=Fast, minimal, and transparent desktop VPN client
Exec=/opt/kite/kite %%U
Icon=kite
Terminal=false
Type=Application
Categories=Network;VPN;Security;
StartupWMClass=kite
MimeType=x-scheme-handler/vless;x-scheme-handler/vmess;x-scheme-handler/trojan;x-scheme-handler/ss;
DESKTOP_EOF
chmod 644 /usr/share/applications/kite.desktop

if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database -q /usr/share/applications 2>/dev/null || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -q /usr/share/icons/hicolor 2>/dev/null || true
fi

# If currently running executable was outside /opt/kite/kite, update it too
if [ "$CUR_EXE" != "/opt/kite/kite" ] && [ -f "$CUR_EXE" ]; then
    cp -f "$NEW_BIN" "$CUR_EXE.new"
    chmod 755 "$CUR_EXE.new"
    mv -f "$CUR_EXE.new" "$CUR_EXE"
    if command -v setcap >/dev/null 2>&1; then
        setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip "$CUR_EXE" 2>/dev/null || true
    fi
fi
`, tmpExtract, newBinary, currentExe)

			if errWrite := os.WriteFile(installerScript, []byte(scriptContent), 0755); errWrite != nil {
				return fmt.Errorf("failed to prepare update script: %w", errWrite)
			}

			slog.Info("Elevating permissions with pkexec to install update...", "script", installerScript)
			cmd := exec.Command("pkexec", "bash", installerScript)
			out, err := cmd.CombinedOutput()
			if err != nil {
				outStr := strings.TrimSpace(string(out))
				if outStr == "" {
					return fmt.Errorf("authentication cancelled or failed (%w)", err)
				}
				return fmt.Errorf("installation failed: %s (%w)", outStr, err)
			}

			slog.Info("Update installed successfully, restarting Kite...", "path", currentExe)
			if onPreQuit != nil {
				onPreQuit()
			}
			return root.RelaunchApp(currentExe)
		}

		return fmt.Errorf("unrecognized package format for Linux update: %s", downloadedFilePath)

	case "darwin":
		_ = exec.Command("open", "-R", downloadedFilePath).Start()
		return nil

	default:
		return errors.New("unsupported operating system for automated update")
	}
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
