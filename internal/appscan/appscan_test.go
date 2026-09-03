package appscan

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetInstalledApps(t *testing.T) {
	apps, err := GetInstalledApps()
	assert.NoError(t, err)
	assert.NotNil(t, apps)

	if runtime.GOOS == "linux" {
		assert.NotEmpty(t, apps, "expected to find at least one installed application on Linux")
		for _, app := range apps {
			assert.NotEmpty(t, app.Name)
			assert.NotEmpty(t, app.ExePath)
		}
	}
}

func TestParseDesktopFile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping desktop file test on non-linux")
	}

	tmpDir := t.TempDir()
	iconPath := filepath.Join(tmpDir, "test-icon.png")
	err := os.WriteFile(iconPath, []byte("fake-png-data"), 0644)
	require.NoError(t, err)

	desktopContent := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Test Application
Exec=/bin/sh %%u
Icon=%s
Comment=A test desktop application
NoDisplay=false
`, iconPath)
	desktopPath := filepath.Join(tmpDir, "test.desktop")
	err = os.WriteFile(desktopPath, []byte(desktopContent), 0644)
	require.NoError(t, err)

	app, err := parseDesktopFile(desktopPath)
	assert.NoError(t, err)
	require.NotNil(t, app)
	assert.Equal(t, "Test Application", app.Name)
	assert.Contains(t, app.Icon, "data:image/png;base64,")
	assert.Equal(t, "A test desktop application", app.Description)
	assert.Equal(t, "/bin/sh", app.ExePath)
}

func TestResolveExecCommand(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping on non-linux")
	}

	assert.Equal(t, "/bin/sh", resolveExecCommand("/bin/sh %F"))
	assert.Equal(t, "/bin/sh", resolveExecCommand("env FOO=bar /bin/sh"))
	assert.Equal(t, "", resolveExecCommand("%u"))
	assert.Equal(t, "", resolveExecCommand(""))
}
