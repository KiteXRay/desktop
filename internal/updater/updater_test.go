package updater

import (
	"testing"
)

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		latest   string
		current  string
		expected bool
	}{
		{"v1.0.1", "1.0.0", true},
		{"v1.1.0", "1.0.9", true},
		{"2.0.0", "1.99.99", true},
		{"v1.0.0", "1.0.0", false},
		{"1.0.0", "1.0.0", false},
		{"0.9.9", "1.0.0", false},
		{"v0.9.0", "v1.0.0", false},
		{"v1.0.1-beta.1", "1.0.0", true},
		{"v1.0.0-rc1", "1.0.0", false},
		{"1.2.3.4", "1.2.3.3", true},
		{"v1.0", "1.0.0", false},
		{"v1.0.1", "v1.0", true},
	}

	for _, tt := range tests {
		got := IsNewerVersion(tt.latest, tt.current)
		if got != tt.expected {
			t.Errorf("IsNewerVersion(%q, %q) = %v; want %v", tt.latest, tt.current, got, tt.expected)
		}
	}
}

func TestSelectAsset(t *testing.T) {
	assets := []GitHubReleaseAsset{
		{Name: "kite-linux-amd64.tar.gz", BrowserDownloadURL: "https://example.com/kite-linux-amd64.tar.gz"},
		{Name: "kite-windows-setup.exe", BrowserDownloadURL: "https://example.com/kite-windows-setup.exe"},
		{Name: "kite-macos-universal.zip", BrowserDownloadURL: "https://example.com/kite-macos-universal.zip"},
	}

	linuxAsset := SelectAsset(assets, "linux", "amd64")
	if linuxAsset == nil || linuxAsset.Name != "kite-linux-amd64.tar.gz" {
		t.Errorf("expected linux asset, got %v", linuxAsset)
	}

	winAsset := SelectAsset(assets, "windows", "amd64")
	if winAsset == nil || winAsset.Name != "kite-windows-setup.exe" {
		t.Errorf("expected windows asset, got %v", winAsset)
	}

	macAsset := SelectAsset(assets, "darwin", "arm64")
	if macAsset == nil || macAsset.Name != "kite-macos-universal.zip" {
		t.Errorf("expected macos asset, got %v", macAsset)
	}
}
