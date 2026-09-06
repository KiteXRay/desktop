package updater

import (
	"os"
	"path/filepath"
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
		// SemVer 2.0 pre-release to stable upgrade
		{"1.0.1", "1.0.1-beta.1", true},
		{"1.0.1-beta.2", "1.0.1-beta.1", true},
		{"1.0.1-rc.1", "1.0.1-beta.2", true},
		{"1.0.1", "1.0.1-rc.1", true},
		{"1.0.1-beta.1", "1.0.1", false},
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

	// Test Debian preference for .deb
	origIsDebian := isDebian
	defer func() { isDebian = origIsDebian }()

	mixedAssets := []GitHubReleaseAsset{
		{Name: "kite-linux-amd64.tar.gz", BrowserDownloadURL: "https://example.com/kite-linux-amd64.tar.gz"},
		{Name: "kite_1.1.0_amd64.deb", BrowserDownloadURL: "https://example.com/kite_1.1.0_amd64.deb"},
		{Name: "kite_1.1.0_arm64.deb", BrowserDownloadURL: "https://example.com/kite_1.1.0_arm64.deb"},
	}

	isDebian = func() bool { return true }
	debAsset := SelectAsset(mixedAssets, "linux", "amd64")
	if debAsset == nil || debAsset.Name != "kite_1.1.0_amd64.deb" {
		t.Errorf("expected deb asset on debian amd64, got %v", debAsset)
	}

	// On non-Debian, .deb should NEVER be selected even if it scores well on name
	isDebian = func() bool { return false }
	tarAsset := SelectAsset(mixedAssets, "linux", "amd64")
	if tarAsset == nil || tarAsset.Name != "kite-linux-amd64.tar.gz" {
		t.Errorf("expected tar.gz asset on non-debian amd64, got %v", tarAsset)
	}

	// Test non-Debian with only .deb available -> should return nil
	onlyDebAssets := []GitHubReleaseAsset{
		{Name: "kite_1.1.0_amd64.deb", BrowserDownloadURL: "https://example.com/kite_1.1.0_amd64.deb"},
	}
	if got := SelectAsset(onlyDebAssets, "linux", "amd64"); got != nil {
		t.Errorf("expected nil for .deb asset on non-debian, got %v", got)
	}
}

func TestParseExpectedChecksum(t *testing.T) {
	content := `# SHA256 Checksums
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  kite-linux-amd64.tar.gz
a591a6d40bf420404a011733cfb7b190d62c65bf0bcda32b57b277d9ad9f146e *kite_1.1.0_amd64.deb
`
	gotLinux := ParseExpectedChecksum(content, "kite-linux-amd64.tar.gz")
	if gotLinux != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("unexpected checksum for linux: %s", gotLinux)
	}

	gotDeb := ParseExpectedChecksum(content, "kite_1.1.0_amd64.deb")
	if gotDeb != "a591a6d40bf420404a011733cfb7b190d62c65bf0bcda32b57b277d9ad9f146e" {
		t.Errorf("unexpected checksum for deb: %s", gotDeb)
	}

	// Test single-hash checksum file
	singleHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855\n"
	gotSingle := ParseExpectedChecksum(singleHash, "any-file.tar.gz")
	if gotSingle != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("unexpected checksum for single-hash: %s", gotSingle)
	}
}

func TestVerifyFileSHA256(t *testing.T) {
	// Create a temp file with known content
	tmp := t.TempDir()
	testFile := filepath.Join(tmp, "test.txt")
	err := os.WriteFile(testFile, []byte("hello world\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// sha256("hello world\n") = a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447
	validSHA := "a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447"
	invalidSHA := "0000000000000000000000000000000000000000000000000000000000000000"

	if err := VerifyFileSHA256(testFile, validSHA); err != nil {
		t.Errorf("expected valid checksum, got error: %v", err)
	}

	if err := VerifyFileSHA256(testFile, invalidSHA); err == nil {
		t.Errorf("expected checksum mismatch error, got nil")
	}

	// Empty checksum should pass without error (no-op)
	if err := VerifyFileSHA256(testFile, ""); err != nil {
		t.Errorf("expected empty checksum to pass, got error: %v", err)
	}
}

