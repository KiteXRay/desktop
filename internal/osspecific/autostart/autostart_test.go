package autostart

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutostart_LinuxLifecycle(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping Linux autostart test on non-linux OS")
	}

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tempHome, ".config"))

	// Initially disabled
	assert.False(t, IsEnabled())

	// Enable autostart
	err := SetEnabled(true)
	require.NoError(t, err)

	assert.True(t, IsEnabled())

	// Verify file content
	targetPath := filepath.Join(tempHome, ".config", "autostart", "kite.desktop")
	data, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "[Desktop Entry]")
	assert.Contains(t, string(data), "Type=Application")
	assert.Contains(t, string(data), "Exec=")
	assert.Contains(t, string(data), "--autostart")
	assert.Contains(t, string(data), "StartupWMClass=kite")

	// Disable autostart
	err = SetEnabled(false)
	require.NoError(t, err)
	assert.False(t, IsEnabled())

	// Verify file removed
	_, err = os.Stat(targetPath)
	assert.True(t, os.IsNotExist(err))
}
