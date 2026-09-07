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
