package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/KiteXRay/desktop/internal/bridge"
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

func TestSaveFile_BridgeGroupsAndRules(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "connections.json")

	sf1 := NewSaveFileWithPath(cfgPath)
	require.Empty(t, sf1.GetBridgeGroups())
	require.Empty(t, sf1.GetBridgeRules())

	groups := []bridge.BridgeGroup{
		{ID: "group-1", Name: "Browsers", Enabled: true},
		{ID: "group-2", Name: "Games", Enabled: false},
	}
	rules := []bridge.BridgeRule{
		{ID: "rule-1", GroupID: "group-1", Pattern: "chrome.exe", ProxyTarget: "socks5://127.0.0.1:10808", ProxyType: "socks5", Enabled: true},
		{ID: "rule-2", GroupID: "group-2", Pattern: "game.exe", ProxyTarget: "http://127.0.0.1:10809", ProxyType: "http", Enabled: true},
	}

	sf1.SetBridgeGroups(groups)
	sf1.SetBridgeRules(rules)
	list1 := connlist.New()
	sf1.Update(list1)

	// Load into a fresh SaveFile instance
	sf2 := NewSaveFileWithPath(cfgPath)
	list2 := connlist.New()
	sf2.Load(list2)

	loadedGroups := sf2.GetBridgeGroups()
	loadedRules := sf2.GetBridgeRules()

	require.Len(t, loadedGroups, 2)
	assert.Equal(t, "group-1", loadedGroups[0].ID)
	assert.Equal(t, "Browsers", loadedGroups[0].Name)
	assert.True(t, loadedGroups[0].Enabled)
	assert.Equal(t, "group-2", loadedGroups[1].ID)
	assert.Equal(t, "Games", loadedGroups[1].Name)
	assert.False(t, loadedGroups[1].Enabled)

	require.Len(t, loadedRules, 2)
	assert.Equal(t, "group-1", loadedRules[0].GroupID)
	assert.Equal(t, "chrome.exe", loadedRules[0].Pattern)
	assert.Equal(t, "group-2", loadedRules[1].GroupID)
	assert.Equal(t, "game.exe", loadedRules[1].Pattern)
}

func TestSaveFile_FallbackAndMigration(t *testing.T) {
	tmpDir := t.TempDir()
	primaryPath := filepath.Join(tmpDir, "primary", "connections.json")
	altPath := filepath.Join(tmpDir, "alt", "connections.json")

	// Pre-populate alternate path with configuration
	altContent := `{
  "tunnelMode": "bridge",
  "tunnelDeviceIp": "192.18.0.99",
  "tunnelDns": "1.1.1.1",
  "connections": []
}`
	require.NoError(t, os.MkdirAll(filepath.Dir(altPath), 0755))
	require.NoError(t, os.WriteFile(altPath, []byte(altContent), 0644))

	// Primary does not exist yet; Load should pick up altPath and sync to primaryPath
	sf := NewSaveFileWithPath(primaryPath)
	sf.altPath = altPath

	list := connlist.New()
	sf.Load(list)

	assert.Equal(t, "bridge", sf.GetTunnelMode())
	devIP, dns := sf.GetTunnelSettings()
	assert.Equal(t, "192.18.0.99", devIP)
	assert.Equal(t, "1.1.1.1", dns)

	// Verify it was synced to primaryPath
	primaryData, err := os.ReadFile(primaryPath)
	require.NoError(t, err)
	assert.Contains(t, string(primaryData), "192.18.0.99")
}

func TestSaveFile_WindowGeometryAndHotkeys(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "connections.json")

	sf1 := NewSaveFileWithPath(cfgPath)
	sf1.SetWindowGeometry(WindowGeometry{
		Width:       1280,
		Height:      800,
		X:           100,
		Y:           150,
		Maximized:   false,
		HasPosition: true,
	})
	sf1.SetHotkeyConfig(HotkeyConfig{
		Enabled:       true,
		ToggleWindow:  "Ctrl+Alt+K",
		ToggleConnect: "Ctrl+Alt+C",
	})
	sf1.SetCompactMode(true)

	list1 := connlist.New()
	sf1.Update(list1)

	// Verify json content
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"width": 1280`)
	assert.Contains(t, string(data), `"toggleWindow": "Ctrl+Alt+K"`)
	assert.Contains(t, string(data), `"compactMode": true`)

	// Load into sf2
	sf2 := NewSaveFileWithPath(cfgPath)
	list2 := connlist.New()
	sf2.Load(list2)

	geom := sf2.GetWindowGeometry()
	require.NotNil(t, geom)
	assert.Equal(t, 1280, geom.Width)
	assert.Equal(t, 800, geom.Height)
	assert.Equal(t, 100, geom.X)
	assert.Equal(t, 150, geom.Y)
	assert.True(t, geom.HasPosition)

	hk := sf2.GetHotkeyConfig()
	require.NotNil(t, hk)
	assert.True(t, hk.Enabled)
	assert.Equal(t, "Ctrl+Alt+K", hk.ToggleWindow)
	assert.Equal(t, "Ctrl+Alt+C", hk.ToggleConnect)

	assert.True(t, sf2.GetCompactMode())

	// Test SaveWindowAndSettings directly
	sf2.SetWindowGeometry(WindowGeometry{
		Width:       1400,
		Height:      900,
		X:           200,
		Y:           250,
		Maximized:   true,
		HasPosition: true,
	})
	sf2.SaveWindowAndSettings()

	sf3 := NewSaveFileWithPath(cfgPath)
	sf3.Load(connlist.New())
	geom3 := sf3.GetWindowGeometry()
	require.NotNil(t, geom3)
	assert.Equal(t, 1400, geom3.Width)
	assert.Equal(t, 900, geom3.Height)
	assert.True(t, geom3.Maximized)
}


