package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/KiteXRay/desktop/internal/connlist"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveFile_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "connections.json")

	sf := NewSaveFileWithPath(cfgPath)
	require.NotNil(t, sf)

	devIP, dns := sf.GetTunnelSettings()
	assert.Equal(t, "192.18.0.1", devIP)
	assert.Equal(t, "8.8.8.8", dns)
	assert.Equal(t, "tunnel", sf.GetTunnelMode())
	assert.Empty(t, sf.GetSubscriptions())
	assert.Empty(t, sf.GetBridgeRules())
}

func TestSaveFile_SetAndPersistTunnelSettings(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "connections.json")

	sf1 := NewSaveFileWithPath(cfgPath)
	sf1.SetTunnelSettings("10.10.0.1", "1.1.1.1")
	sf1.SetTunnelMode("proxy")

	list1 := connlist.New()
	sf1.Update(list1)

	// Verify file was written
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"tunnelDns": "1.1.1.1"`)
	assert.Contains(t, string(data), `"tunnelDeviceIp": "10.10.0.1"`)
	assert.Contains(t, string(data), `"tunnelMode": "proxy"`)

	// Load into a new SaveFile instance
	sf2 := NewSaveFileWithPath(cfgPath)
	list2 := connlist.New()
	sf2.Load(list2)

	devIP, dns := sf2.GetTunnelSettings()
	assert.Equal(t, "10.10.0.1", devIP)
	assert.Equal(t, "1.1.1.1", dns)
	assert.Equal(t, "proxy", sf2.GetTunnelMode())
}

func TestSaveFile_TrimWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "connections.json")

	sf := NewSaveFileWithPath(cfgPath)
	sf.SetTunnelSettings("  172.16.0.1  ", "  9.9.9.9  ")

	devIP, dns := sf.GetTunnelSettings()
	assert.Equal(t, "172.16.0.1", devIP)
	assert.Equal(t, "9.9.9.9", dns)
}

func TestSaveFile_ZeroConnectionsDoesNotBreakLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "connections.json")

	// Write json with zero connections and custom DNS
	jsonContent := `{
  "tunnelMode": "tunnel",
  "tunnelDeviceIp": "192.18.0.2",
  "tunnelDns": "1.0.0.1",
  "connections": []
}`
	err := os.WriteFile(cfgPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	sf := NewSaveFileWithPath(cfgPath)
	list := connlist.New()
	sf.Load(list)

	devIP, dns := sf.GetTunnelSettings()
	assert.Equal(t, "192.18.0.2", devIP)
	assert.Equal(t, "1.0.0.1", dns)
	assert.Equal(t, 0, len(list.All()))
}

func TestApp_GetTunnelSettingsDTO(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "connections.json")

	sf := NewSaveFileWithPath(cfgPath)
	sf.SetTunnelSettings("192.18.0.5", "1.1.1.1")

	items := connlist.New()
	app := &App{
		items:    items,
		saveFile: sf,
	}

	dto := app.GetTunnelSettings()
	assert.Equal(t, "192.18.0.5", dto.DeviceIP)
	assert.Equal(t, "1.1.1.1", dto.DNS)
}
