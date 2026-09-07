package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/KiteXRay/desktop/internal/connlist"
	"github.com/energye/systray"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestAppForTray(t *testing.T) *App {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "connections.json")
	items := connlist.New()
	saveFile := NewSaveFileWithPath(cfgPath)

	app := &App{
		items:      items,
		saveFile:   saveFile,
		stopTicker: make(chan struct{}),
		tunnelMode: saveFile.GetTunnelMode(),
	}
	return app
}

// triggerMenuItemClick simulates a click event on an energye/systray.MenuItem
func triggerMenuItemClick(item *systray.MenuItem) bool {
	v := reflect.ValueOf(item).Elem()
	field := v.FieldByName("click")
	if field.IsValid() && !field.IsNil() {
		fn := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface().(func())
		fn()
		return true
	}
	return false
}

func TestTrayModeMenu_InitialState(t *testing.T) {
	app := newTestAppForTray(t)
	tc := &TrayController{app: app}
	app.onTrayUpdate = tc.updateMenu

	// Render tray menu
	tc.updateMenu()

	require.NotNil(t, tc.mMode, "Mode parent menu item should exist")
	assert.Contains(t, tc.mMode.String(), `"Mode"`)
	require.Len(t, tc.mModeItems, 3, "Mode should have 3 sub-menu items: tunnel, proxy, bridge")

	// Default mode should be tunnel
	assert.Equal(t, "tunnel", app.GetTunnelMode())

	tunnelItem := tc.mModeItems["tunnel"]
	proxyItem := tc.mModeItems["proxy"]
	bridgeItem := tc.mModeItems["bridge"]

	require.NotNil(t, tunnelItem)
	require.NotNil(t, proxyItem)
	require.NotNil(t, bridgeItem)

	// Tunnel should be checked and have active bullet
	assert.True(t, tunnelItem.Checked(), "Tunnel item should be checked")
	assert.Contains(t, tunnelItem.String(), "● Tunnel")

	// Proxy and Bridge should be unchecked and have inactive bullet
	assert.False(t, proxyItem.Checked(), "Proxy item should not be checked")
	assert.Contains(t, proxyItem.String(), "○ Proxy")

	assert.False(t, bridgeItem.Checked(), "Bridge item should not be checked")
	assert.Contains(t, bridgeItem.String(), "○ Bridge")
}

func TestTrayModeMenu_SwitchMode(t *testing.T) {
	app := newTestAppForTray(t)
	tc := &TrayController{app: app}
	app.onTrayUpdate = tc.updateMenu

	tc.updateMenu()

	// 1. Switch to Proxy mode
	err := app.SetTunnelMode("proxy")
	require.NoError(t, err)
	assert.Equal(t, "proxy", app.GetTunnelMode())

	// onTrayUpdate was called automatically by SetTunnelMode
	require.Len(t, tc.mModeItems, 3)
	assert.False(t, tc.mModeItems["tunnel"].Checked(), "Tunnel item should not be checked")
	assert.Contains(t, tc.mModeItems["tunnel"].String(), "○ Tunnel")
	assert.True(t, tc.mModeItems["proxy"].Checked(), "Proxy item should be checked")
	assert.Contains(t, tc.mModeItems["proxy"].String(), "● Proxy")
	assert.False(t, tc.mModeItems["bridge"].Checked(), "Bridge item should not be checked")
	assert.Contains(t, tc.mModeItems["bridge"].String(), "○ Bridge")

	// 2. Switch to Bridge mode
	err = app.SetTunnelMode("bridge")
	require.NoError(t, err)
	assert.Equal(t, "bridge", app.GetTunnelMode())

	require.Len(t, tc.mModeItems, 3)
	assert.False(t, tc.mModeItems["tunnel"].Checked(), "Tunnel item should not be checked")
	assert.Contains(t, tc.mModeItems["tunnel"].String(), "○ Tunnel")
	assert.False(t, tc.mModeItems["proxy"].Checked(), "Proxy item should not be checked")
	assert.Contains(t, tc.mModeItems["proxy"].String(), "○ Proxy")
	assert.True(t, tc.mModeItems["bridge"].Checked(), "Bridge item should be checked")
	assert.Contains(t, tc.mModeItems["bridge"].String(), "● Bridge")

	// 3. Switch back to Tunnel mode
	err = app.SetTunnelMode("tunnel")
	require.NoError(t, err)
	assert.Equal(t, "tunnel", app.GetTunnelMode())

	require.Len(t, tc.mModeItems, 3)
	assert.True(t, tc.mModeItems["tunnel"].Checked(), "Tunnel item should be checked")
	assert.Contains(t, tc.mModeItems["tunnel"].String(), "● Tunnel")
	assert.False(t, tc.mModeItems["proxy"].Checked(), "Proxy item should not be checked")
	assert.Contains(t, tc.mModeItems["proxy"].String(), "○ Proxy")
	assert.False(t, tc.mModeItems["bridge"].Checked(), "Bridge item should not be checked")
	assert.Contains(t, tc.mModeItems["bridge"].String(), "○ Bridge")
}

func TestTrayModeMenu_SwitchModeViaTrayController(t *testing.T) {
	app := newTestAppForTray(t)
	tc := &TrayController{app: app}
	app.onTrayUpdate = tc.updateMenu

	tc.updateMenu()

	// Switch via switchMode method (which is invoked on tray click)
	tc.switchMode("proxy")

	// Wait for goroutine to execute SetTunnelMode
	require.Eventually(t, func() bool {
		return app.GetTunnelMode() == "proxy" && tc.mModeItems["proxy"].Checked()
	}, 1*time.Second, 10*time.Millisecond)

	assert.False(t, tc.mModeItems["tunnel"].Checked())
	assert.True(t, tc.mModeItems["proxy"].Checked())
	assert.False(t, tc.mModeItems["bridge"].Checked())
}

func TestTrayModeMenu_ClickTrigger(t *testing.T) {
	app := newTestAppForTray(t)
	tc := &TrayController{app: app}
	app.onTrayUpdate = tc.updateMenu

	tc.updateMenu()

	// Trigger the click callback directly on the bridge sub-menu item
	bridgeItem := tc.mModeItems["bridge"]
	require.NotNil(t, bridgeItem)
	clicked := triggerMenuItemClick(bridgeItem)
	require.True(t, clicked, "Should have triggered click handler on bridgeItem")

	// Wait for mode to update
	require.Eventually(t, func() bool {
		return app.GetTunnelMode() == "bridge" && tc.mModeItems["bridge"].Checked()
	}, 1*time.Second, 10*time.Millisecond)

	assert.False(t, tc.mModeItems["tunnel"].Checked())
	assert.False(t, tc.mModeItems["proxy"].Checked())
	assert.True(t, tc.mModeItems["bridge"].Checked())
	assert.Contains(t, tc.mModeItems["bridge"].String(), "● Bridge")
}

func TestTrayModeMenu_SubmenuParentHierarchy(t *testing.T) {
	app := newTestAppForTray(t)
	tc := &TrayController{app: app}
	app.onTrayUpdate = tc.updateMenu

	tc.updateMenu()

	for modeKey, item := range tc.mModeItems {
		itemStr := item.String()
		assert.True(t, strings.Contains(itemStr, "parent"), "Mode sub-item %s should have parent in %s", modeKey, itemStr)
	}
}
