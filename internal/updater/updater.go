package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"sync"
	"time"

	"github.com/KiteXRay/desktop/internal/osspecific/root"
	"golang.org/x/mod/semver"
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
	ChecksumURL  string `json:"checksumUrl,omitempty"`
	ExpectedSHA  string `json:"expectedSha,omitempty"`
}

var (
	cacheMu           sync.Mutex
	cachedReleaseInfo *ReleaseInfo
	cachedETag        string
	cachedRepo        string
	cachedTime        time.Time
)

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

func normalizeSemVer(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") && !strings.HasPrefix(v, "V") {
		v = "v" + v
	} else if strings.HasPrefix(v, "V") {
		v = "v" + v[1:]
	}
	parts := strings.SplitN(v, "-", 2)
	base := parts[0]
	dots := strings.Count(base, ".")
	if dots == 0 {
		base += ".0.0"
	} else if dots == 1 {
		base += ".0"
	}
	if len(parts) > 1 {
		v = base + "-" + parts[1]
	} else {
		v = base
	}
	return v
}

func IsNewerVersion(latest, current string) bool {
	normLatest := normalizeSemVer(latest)
	normCurrent := normalizeSemVer(current)

	if semver.IsValid(normLatest) && semver.IsValid(normCurrent) {
		return semver.Compare(normLatest, normCurrent) > 0
	}

	// Fallback to legacy numeric slice comparison (e.g. 4-part versions)
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

var isDebian = isDebianSystem

func isDebianSystem() bool {
	if _, err := os.Stat("/etc/debian_version"); err == nil {
		return true
	}
	if _, err := exec.LookPath("dpkg"); err == nil {
		return true
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
			if goarch == "amd64" && (strings.Contains(name, "arm64") || strings.Contains(name, "arm")) {
				continue
			}
			if goarch == "arm64" && (strings.Contains(name, "amd64") || strings.Contains(name, "x64")) {
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
			// Skip wrong architectures
			if goarch == "amd64" && (strings.Contains(name, "arm64") || strings.Contains(name, "aarch64") || strings.Contains(name, "armv")) {
				continue
			}
			if goarch == "arm64" && (strings.Contains(name, "amd64") || strings.Contains(name, "x86_64") || strings.Contains(name, "x64")) {
				continue
			}
			if strings.Contains(name, "linux") {
				score += 10
			}
			if strings.Contains(name, goarch) || (goarch == "amd64" && (strings.Contains(name, "x86_64") || strings.Contains(name, "x64"))) {
				score += 10
			}
			if isDebian() {
				if strings.HasSuffix(name, ".deb") {
					score += 25
				} else if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") {
					score += 10
				}
			} else {
				// Non-Debian: skip .deb packages entirely so they are never picked
				if strings.HasSuffix(name, ".deb") {
					continue
				}
				if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") {
					score += 25
				}
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

func FindChecksumAsset(assets []GitHubReleaseAsset, targetAssetName string) *GitHubReleaseAsset {
	targetBase := strings.ToLower(targetAssetName)
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if name == targetBase+".sha256" || name == targetBase+".sha256sum" {
			return &assets[i]
		}
	}
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if name == "checksums.txt" || name == "sha256sums" || name == "sha256sums.txt" {
			return &assets[i]
		}
	}
	return nil
}

func ParseExpectedChecksum(content, targetAssetName string) string {
	lines := strings.Split(content, "\n")
	targetBase := filepath.Base(targetAssetName)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			hash := fields[0]
			file := strings.TrimPrefix(fields[1], "*")
			file = filepath.Base(file)
			if strings.EqualFold(file, targetBase) && len(hash) == 64 {
				return strings.ToLower(hash)
			}
		} else if len(fields) == 1 && len(fields[0]) == 64 {
			return strings.ToLower(fields[0])
		}
	}
	return ""
}

func VerifyFileSHA256(filePath, expectedSHA string) error {
	expectedSHA = strings.ToLower(strings.TrimSpace(expectedSHA))
	if expectedSHA == "" {
		return nil
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file for checksum verification: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("calculate sha256: %w", err)
	}
	actualSHA := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actualSHA, expectedSHA) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA, actualSHA)
	}
	return nil
}

func CheckForUpdate(ctx context.Context, repo string, currentVersion string) (*ReleaseInfo, error) {
	repoPath := strings.TrimPrefix(repo, "https://github.com/")
	repoPath = strings.TrimPrefix(repoPath, "http://github.com/")
	repoPath = strings.TrimSuffix(repoPath, ".git")
	repoPath = strings.Trim(repoPath, "/")

	if repoPath == "" {
		repoPath = "KiteXRay/desktop"
	}

	cacheMu.Lock()
	if cachedRepo == repoPath && cachedReleaseInfo != nil && time.Since(cachedTime) < 5*time.Minute {
		info := *cachedReleaseInfo
		info.CurrentVer = currentVersion
		info.Available = IsNewerVersion(info.LatestVer, currentVersion)
		cacheMu.Unlock()
		return &info, nil
	}
	reqETag := ""
	if cachedRepo == repoPath {
		reqETag = cachedETag
	}
	cacheMu.Unlock()

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repoPath)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Kite-Desktop-Updater")
	if reqETag != "" {
		req.Header.Set("If-None-Match", reqETag)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		cacheMu.Lock()
		if cachedReleaseInfo != nil {
			cachedTime = time.Now()
			info := *cachedReleaseInfo
			info.CurrentVer = currentVersion
			info.Available = IsNewerVersion(info.LatestVer, currentVersion)
			cacheMu.Unlock()
			return &info, nil
		}
		cacheMu.Unlock()
	}

	if resp.StatusCode == http.StatusNotFound {
		return &ReleaseInfo{
			Available:  false,
			CurrentVer: currentVersion,
		}, nil
	}

	if resp.StatusCode == http.StatusForbidden {
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return nil, errors.New("GitHub API rate limit exceeded. Please try again later.")
		}
		return nil, fmt.Errorf("github api returned status %d", resp.StatusCode)
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

		if chkAsset := FindChecksumAsset(rel.Assets, matchedAsset.Name); chkAsset != nil {
			info.ChecksumURL = chkAsset.BrowserDownloadURL
			chkReq, errChk := http.NewRequestWithContext(ctx, "GET", chkAsset.BrowserDownloadURL, nil)
			if errChk == nil {
				chkReq.Header.Set("User-Agent", "Kite-Desktop-Updater")
				if chkResp, errDo := client.Do(chkReq); errDo == nil && chkResp.StatusCode == http.StatusOK {
					chkBytes, _ := io.ReadAll(io.LimitReader(chkResp.Body, 64*1024))
					chkResp.Body.Close()
					info.ExpectedSHA = ParseExpectedChecksum(string(chkBytes), matchedAsset.Name)
				}
			}
		}
	} else {
		info.AssetURL = rel.HTMLURL
	}

	cacheMu.Lock()
	cachedRepo = repoPath
	cachedETag = resp.Header.Get("ETag")
	cachedTime = time.Now()
	cachedReleaseInfo = info
	cacheMu.Unlock()

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
	destDir = filepath.Clean(destDir)
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

	// Guard against decompression bombs (max 500 MB)
	limitedReader := io.LimitReader(gzr, 500*1024*1024)
	tr := tar.NewReader(limitedReader)
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
		// Reject absolute paths or traversal
		if filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, "..") {
			continue
		}
		target := filepath.Join(destDir, cleanName)
		rel, err := filepath.Rel(destDir, target)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", err
			}
			mode := header.FileInfo().Mode() & 0755
			if mode == 0 {
				mode = 0644
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, mode)
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

func ExtractZip(zipPath, destDir string) (string, error) {
	destDir = filepath.Clean(destDir)
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var appBundlePath string
	for _, f := range r.File {
		cleanName := filepath.Clean(f.Name)
		if filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, "..") {
			continue
		}
		target := filepath.Join(destDir, cleanName)
		rel, err := filepath.Rel(destDir, target)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", err
			}
			if strings.HasSuffix(cleanName, ".app") && appBundlePath == "" {
				appBundlePath = target
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return "", err
		}

		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		mode := f.FileInfo().Mode() & 0755
		if mode == 0 {
			mode = 0644
		}
		outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, mode)
		if err != nil {
			rc.Close()
			return "", err
		}
		_, err = io.Copy(outFile, io.LimitReader(rc, 500*1024*1024))
		outFile.Close()
		rc.Close()
		if err != nil {
			return "", err
		}
	}

	if appBundlePath == "" {
		entries, _ := os.ReadDir(destDir)
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".app") {
				appBundlePath = filepath.Join(destDir, e.Name())
				break
			}
		}
	}

	return appBundlePath, nil
}

func findEnclosingAppBundle(exePath string) string {
	curr := exePath
	for {
		if strings.HasSuffix(curr, ".app") {
			return curr
		}
		parent := filepath.Dir(curr)
		if parent == curr || parent == "." || parent == "/" {
			break
		}
		curr = parent
	}
	return ""
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
			_ = os.Chmod(downloadedFilePath, 0644)
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
			targetExe := "/opt/kite/kite"
			if _, err := os.Stat(targetExe); err != nil {
				if currentExe, errExe := os.Executable(); errExe == nil {
					targetExe = currentExe
				}
			}
			return root.RelaunchApp(targetExe)
		}

		if strings.HasSuffix(downloadedFilePath, ".tar.gz") || strings.HasSuffix(downloadedFilePath, ".tgz") {
			tmpExtract, err := os.MkdirTemp("", "kite_update_*")
			if err != nil {
				return fmt.Errorf("failed to create temp extraction directory: %w", err)
			}
			_ = os.Chmod(tmpExtract, 0700)
			defer os.RemoveAll(tmpExtract)

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

			// Check if we can write directly without root
			canWriteDirectly := false
			testFile, errTest := os.CreateTemp(filepath.Dir(currentExe), ".write_test_*")
			if errTest == nil {
				_ = testFile.Close()
				_ = os.Remove(testFile.Name())
				canWriteDirectly = true
			}

			if canWriteDirectly {
				tmpNew := currentExe + ".new"
				_ = os.Remove(tmpNew)
				if errCopy := copyFile(newBinary, tmpNew, 0755); errCopy == nil {
					if errRename := os.Rename(tmpNew, currentExe); errRename == nil {
						// Re-assign network capabilities if setcap is available
						if setcapPath, errCap := exec.LookPath("setcap"); errCap == nil {
							_ = exec.Command(setcapPath, "cap_net_raw,cap_net_admin,cap_net_bind_service+eip", currentExe).Run()
						}
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

# Atomically replace /opt/kite/kite without truncating running inode
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
		if strings.HasSuffix(downloadedFilePath, ".zip") {
			tmpExtract, err := os.MkdirTemp("", "kite_mac_update_*")
			if err != nil {
				return fmt.Errorf("create temp extraction directory: %w", err)
			}
			_ = os.Chmod(tmpExtract, 0700)
			defer os.RemoveAll(tmpExtract)

			appBundle, err := ExtractZip(downloadedFilePath, tmpExtract)
			if err != nil {
				return fmt.Errorf("extract macos update archive: %w", err)
			}
			if appBundle == "" {
				return errors.New("no .app bundle found in macOS update archive")
			}

			currentExe, _ := os.Executable()
			targetApp := findEnclosingAppBundle(currentExe)
			if targetApp == "" {
				targetApp = "/Applications/Kite.app"
			}

			slog.Info("Applying macOS update...", "source", appBundle, "target", targetApp)

			var installErr error
			testFile, errTest := os.CreateTemp(filepath.Dir(targetApp), ".write_test_*")
			if errTest == nil {
				_ = testFile.Close()
				_ = os.Remove(testFile.Name())
				cmd := exec.Command("ditto", appBundle, targetApp)
				if out, errDitto := cmd.CombinedOutput(); errDitto != nil {
					installErr = fmt.Errorf("ditto copy failed: %s (%w)", string(out), errDitto)
				}
			} else {
				script := fmt.Sprintf(`do shell script "ditto %q %q" with administrator privileges`, appBundle, targetApp)
				cmd := exec.Command("osascript", "-e", script)
				if out, errOSA := cmd.CombinedOutput(); errOSA != nil {
					installErr = fmt.Errorf("osascript admin install failed: %s (%w)", string(out), errOSA)
				}
			}

			if installErr != nil {
				return installErr
			}

			slog.Info("macOS update applied, relaunching application...", "app", targetApp)
			if onPreQuit != nil {
				onPreQuit()
			}
			_ = exec.Command("open", "-n", targetApp).Start()
			os.Exit(0)
			return nil
		}

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
