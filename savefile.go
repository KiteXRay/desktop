package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/KiteXRay/desktop/internal/bridge"
	"github.com/KiteXRay/desktop/internal/connlist"
	"github.com/KiteXRay/desktop/internal/osspecific/root"
	"github.com/KiteXRay/desktop/internal/subscription"
)

const (
	configSubdir   = "kite"
	configFileName = "connections.json"
)

type SavedState struct {
	ID             string `json:"id,omitempty"`
	SubscriptionID string `json:"subscriptionId,omitempty"`
	Link           string `json:"link"`
	Label          string `json:"label"`
	TotalRead      int64  `json:"totalRead,omitempty"`
	TotalWritten   int64  `json:"totalWritten,omitempty"`
}

type WindowGeometry struct {
	Width       int  `json:"width,omitempty"`
	Height      int  `json:"height,omitempty"`
	X           int  `json:"x,omitempty"`
	Y           int  `json:"y,omitempty"`
	Maximized   bool `json:"maximized,omitempty"`
	HasPosition bool `json:"hasPosition,omitempty"`
}

type HotkeyConfig struct {
	Enabled       bool   `json:"enabled"`
	ToggleWindow  string `json:"toggleWindow,omitempty"`
	ToggleConnect string `json:"toggleConnect,omitempty"`
}

type AppConfigFile struct {
	TunnelMode     string                      `json:"tunnelMode"`
	TunnelDeviceIP string                      `json:"tunnelDeviceIp,omitempty"`
	TunnelDNS      string                      `json:"tunnelDns,omitempty"`
	Window         *WindowGeometry             `json:"window,omitempty"`
	Hotkey         *HotkeyConfig               `json:"hotkey,omitempty"`
	CompactMode    bool                        `json:"compactMode,omitempty"`
	Subscriptions  []subscription.Subscription `json:"subscriptions,omitempty"`
	BridgeGroups   []bridge.BridgeGroup        `json:"bridgeGroups,omitempty"`
	BridgeRules    []bridge.BridgeRule         `json:"bridgeRules,omitempty"`
	Connections    []SavedState                `json:"connections"`
}

type SaveFile struct {
	filePath       string
	altPath        string
	tunnelMode     string
	tunnelDeviceIP string
	tunnelDNS      string
	window         *WindowGeometry
	hotkey         *HotkeyConfig
	compactMode    bool
	subscriptions  []subscription.Subscription
	bridgeGroups   []bridge.BridgeGroup
	bridgeRules    []bridge.BridgeRule
	lastSavedItems []SavedState
	mu             sync.Mutex
}

func serialize(item *connlist.Item) SavedState {
	return SavedState{
		ID:             item.ID(),
		SubscriptionID: item.SubscriptionID(),
		Link:           item.Link(),
		Label:          item.Label(),
		TotalRead:      item.BytesRead(),
		TotalWritten:   item.BytesWritten(),
	}
}

func NewSaveFileWithPath(path string) *SaveFile {
	dir := filepath.Dir(path)
	_ = os.MkdirAll(dir, 0755)

	defaultMode := "tunnel"
	if runtime.GOOS == "darwin" {
		defaultMode = "proxy"
	}

	defWin := "Ctrl+Shift+K"
	defConn := "Ctrl+Shift+C"
	if runtime.GOOS == "darwin" {
		defWin = "Cmd+Shift+K"
		defConn = "Cmd+Shift+C"
	}

	return &SaveFile{
		filePath:       path,
		tunnelMode:     defaultMode,
		tunnelDeviceIP: "192.18.0.1",
		tunnelDNS:      "8.8.8.8",
		window: &WindowGeometry{
			Width:       1024,
			Height:      700,
			HasPosition: false,
		},
		hotkey: &HotkeyConfig{
			Enabled:       true,
			ToggleWindow:  defWin,
			ToggleConnect: defConn,
		},
		compactMode:    false,
		subscriptions:  make([]subscription.Subscription, 0),
		bridgeGroups:   make([]bridge.BridgeGroup, 0),
		bridgeRules:    make([]bridge.BridgeRule, 0),
		lastSavedItems: make([]SavedState, 0),
	}
}


func NewSaveFile() *SaveFile {
	home, _ := os.UserHomeDir()
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		configDir = filepath.Join(home, ".config")
	}

	primaryPath := filepath.Join(configDir, configSubdir, configFileName)
	var altPath string
	if home != "" {
		if runtime.GOOS == "darwin" {
			altPath = filepath.Join(home, ".config", configSubdir, configFileName)
		} else {
			altPath = filepath.Join(home, ".local", "share", configSubdir, configFileName)
		}
	}

	primaryDir := filepath.Dir(primaryPath)
	if !isDirWritable(primaryDir) {
		fixPathOwnership(primaryDir)
		if !isDirWritable(primaryDir) && altPath != "" {
			slog.Warn("Primary config directory not writable, switching to alternate path", "primary", primaryPath, "alt", altPath)
			sf := NewSaveFileWithPath(altPath)
			sf.altPath = primaryPath
			return sf
		}
	}

	sf := NewSaveFileWithPath(primaryPath)
	sf.altPath = altPath
	return sf
}

func isDirWritable(dir string) bool {
	_ = os.MkdirAll(dir, 0755)
	testFile := filepath.Join(dir, ".kite_test_perm")
	if err := os.WriteFile(testFile, []byte("1"), 0644); err != nil {
		return false
	}
	_ = os.Remove(testFile)
	return true
}

func fixPathOwnership(targetPath string) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return
	}
	_ = os.Chmod(targetPath, 0755)
	if tunnelBin, err := root.FindTunnelBinary(); err == nil && tunnelBin != "" {
		cmd := exec.Command(tunnelBin, "--fix-perms", targetPath)
		_ = cmd.Run()
	}
}

func (s *SaveFile) GetTunnelMode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.tunnelMode {
	case "proxy":
		return "proxy"
	case "bridge", "per_app":
		return "bridge"
	default:
		return "tunnel"
	}
}

func (s *SaveFile) SetTunnelMode(mode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch mode {
	case "proxy":
		s.tunnelMode = "proxy"
	case "bridge", "per_app":
		s.tunnelMode = "bridge"
	default:
		s.tunnelMode = "tunnel"
	}
}

func (s *SaveFile) GetTunnelSettings() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	devIP := s.tunnelDeviceIP
	if devIP == "" {
		devIP = "192.18.0.1"
	}
	dns := s.tunnelDNS
	if dns == "" {
		dns = "8.8.8.8"
	}
	return devIP, dns
}

func (s *SaveFile) FilePath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.filePath
}

func (s *SaveFile) SetTunnelSettings(deviceIP, dns string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	deviceIP = strings.TrimSpace(deviceIP)
	dns = strings.TrimSpace(dns)
	if deviceIP != "" {
		s.tunnelDeviceIP = deviceIP
	}
	if dns != "" {
		s.tunnelDNS = dns
	}
}

func (s *SaveFile) GetSubscriptions() []subscription.Subscription {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := make([]subscription.Subscription, len(s.subscriptions))
	copy(res, s.subscriptions)
	return res
}

func (s *SaveFile) SetSubscriptions(subs []subscription.Subscription) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscriptions = make([]subscription.Subscription, len(subs))
	copy(s.subscriptions, subs)
}

func (s *SaveFile) GetBridgeGroups() []bridge.BridgeGroup {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := make([]bridge.BridgeGroup, len(s.bridgeGroups))
	copy(res, s.bridgeGroups)
	return res
}

func (s *SaveFile) SetBridgeGroups(groups []bridge.BridgeGroup) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bridgeGroups = make([]bridge.BridgeGroup, len(groups))
	copy(s.bridgeGroups, groups)
}

func (s *SaveFile) GetBridgeRules() []bridge.BridgeRule {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := make([]bridge.BridgeRule, len(s.bridgeRules))
	copy(res, s.bridgeRules)
	return res
}

func (s *SaveFile) SetBridgeRules(rules []bridge.BridgeRule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bridgeRules = make([]bridge.BridgeRule, len(rules))
	copy(s.bridgeRules, rules)
}

func (s *SaveFile) GetWindowGeometry() *WindowGeometry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.window == nil {
		return &WindowGeometry{Width: 1024, Height: 700}
	}
	cpy := *s.window
	return &cpy
}

func (s *SaveFile) SetWindowGeometry(geom WindowGeometry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.window = &geom
}

func (s *SaveFile) GetHotkeyConfig() *HotkeyConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hotkey == nil {
		defWin := "Ctrl+Shift+K"
		defConn := "Ctrl+Shift+C"
		if runtime.GOOS == "darwin" {
			defWin = "Cmd+Shift+K"
			defConn = "Cmd+Shift+C"
		}
		return &HotkeyConfig{Enabled: true, ToggleWindow: defWin, ToggleConnect: defConn}
	}
	cpy := *s.hotkey
	return &cpy
}

func (s *SaveFile) SetHotkeyConfig(cfg HotkeyConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hotkey = &cfg
}

func (s *SaveFile) GetCompactMode() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.compactMode
}

func (s *SaveFile) SetCompactMode(compact bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.compactMode = compact
}

// SaveWindowAndSettings persists window geometry and current settings without requiring list reload.
func (s *SaveFile) SaveWindowAndSettings() {
	s.mu.Lock()
	defer s.mu.Unlock()

	tMode := s.tunnelMode
	if tMode == "" {
		tMode = "tunnel"
	}
	devIP := s.tunnelDeviceIP
	if devIP == "" {
		devIP = "192.18.0.1"
	}
	dns := s.tunnelDNS
	if dns == "" {
		dns = "8.8.8.8"
	}

	cfg := AppConfigFile{
		TunnelMode:     tMode,
		TunnelDeviceIP: devIP,
		TunnelDNS:      dns,
		Window:         s.window,
		Hotkey:         s.hotkey,
		CompactMode:    s.compactMode,
		Subscriptions:  s.subscriptions,
		BridgeGroups:   s.bridgeGroups,
		BridgeRules:    s.bridgeRules,
		Connections:    s.lastSavedItems,
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		slog.Error("failed to marshal config", "error", err)
		return
	}

	err1 := saveToDisk(s.filePath, b)
	if err1 != nil {
		fixPathOwnership(filepath.Dir(s.filePath))
		_ = saveToDisk(s.filePath, b)
	}
	if s.altPath != "" && s.altPath != s.filePath {
		_ = saveToDisk(s.altPath, b)
	}
}

// Update saves list, tunnel mode, settings, subscriptions, bridge groups, and bridge rules atomically into JSON file.
func (s *SaveFile) Update(list *connlist.Collection) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := list.All()
	toSave := make([]SavedState, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		toSave = append(toSave, serialize(item))
	}
	s.lastSavedItems = toSave

	tMode := s.tunnelMode
	if tMode == "" {
		tMode = "tunnel"
	}
	devIP := s.tunnelDeviceIP
	if devIP == "" {
		devIP = "192.18.0.1"
	}
	dns := s.tunnelDNS
	if dns == "" {
		dns = "8.8.8.8"
	}

	cfg := AppConfigFile{
		TunnelMode:     tMode,
		TunnelDeviceIP: devIP,
		TunnelDNS:      dns,
		Window:         s.window,
		Hotkey:         s.hotkey,
		CompactMode:    s.compactMode,
		Subscriptions:  s.subscriptions,
		BridgeGroups:   s.bridgeGroups,
		BridgeRules:    s.bridgeRules,
		Connections:    toSave,
	}


	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		slog.Error("failed to marshal config", "error", err)
		return
	}

	err1 := saveToDisk(s.filePath, b)
	if err1 != nil {
		slog.Warn("Failed to save to primary config path, attempting permission fix and retry", "path", s.filePath, "error", err1)
		fixPathOwnership(filepath.Dir(s.filePath))
		err1 = saveToDisk(s.filePath, b)
	}

	if s.altPath != "" && s.altPath != s.filePath {
		err2 := saveToDisk(s.altPath, b)
		if err2 != nil && err1 != nil {
			slog.Error("Failed to save config to both primary and alternate paths", "primary", s.filePath, "alt", s.altPath, "err1", err1, "err2", err2)
		}
	} else if err1 != nil {
		slog.Error("Failed to save config to disk", "path", s.filePath, "error", err1)
	}
}

func saveToDisk(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		_ = os.Chmod(dir, 0755)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	// 1. Try atomic temp file write
	tmpFile, err := os.CreateTemp(dir, "connections-*.tmp")
	if err == nil {
		tmpName := tmpFile.Name()
		_, writeErr := tmpFile.Write(data)
		_ = tmpFile.Sync()
		_ = tmpFile.Close()

		if writeErr == nil {
			if renameErr := os.Rename(tmpName, path); renameErr == nil {
				return nil
			}
			// Rename may fail on some systems if destination exists with different perms
			_ = os.Chmod(path, 0644)
			_ = os.Remove(path)
			if renameErr2 := os.Rename(tmpName, path); renameErr2 == nil {
				return nil
			}
		}
		_ = os.Remove(tmpName)
	}

	// 2. Direct write fallback
	_ = os.Chmod(path, 0644)
	return os.WriteFile(path, data, 0644)
}

// Load loads saved items into list, searching primary, alternate, and legacy locations.
func (s *SaveFile) Load(list *connlist.Collection) {
	// Candidate search paths in priority order
	var paths []string
	if s.filePath != "" {
		paths = append(paths, s.filePath)
	}
	if s.altPath != "" && s.altPath != s.filePath {
		paths = append(paths, s.altPath)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths,
			filepath.Join(home, "Library", "Application Support", configSubdir, configFileName),
			filepath.Join(home, ".config", configSubdir, configFileName),
			filepath.Join(home, "Library", "Application Support", "goxray", configFileName),
			filepath.Join(home, ".config", "goxray", configFileName),
		)
	}

	var data []byte
	var loadedFrom string
	for _, p := range paths {
		if p == "" {
			continue
		}
		if d, err := os.ReadFile(p); err == nil && len(d) > 0 {
			var test json.RawMessage
			if err := json.Unmarshal(d, &test); err == nil {
				data = d
				loadedFrom = p
				break
			}
		}
	}

	if len(data) == 0 {
		s.tryMigrateFromFyne(list)
		return
	}

	var appCfg AppConfigFile
	if err := json.Unmarshal(data, &appCfg); err == nil {
		s.mu.Lock()
		if appCfg.TunnelMode != "" {
			s.tunnelMode = appCfg.TunnelMode
		}
		if appCfg.TunnelDeviceIP != "" {
			s.tunnelDeviceIP = strings.TrimSpace(appCfg.TunnelDeviceIP)
		}
		if appCfg.TunnelDNS != "" {
			s.tunnelDNS = strings.TrimSpace(appCfg.TunnelDNS)
		}
		if appCfg.Subscriptions != nil {
			for i := range appCfg.Subscriptions {
				sub := &appCfg.Subscriptions[i]
				if sub.SubID == "" {
					sub.SubID = subscription.ExtractSubIDFromURL(sub.URL)
				}
				if sub.SubID != "" && (sub.Label == "" || !strings.HasPrefix(sub.Label, "Subscription-")) {
					sub.Label = fmt.Sprintf("Subscription-%s", sub.SubID)
				}
			}
			s.subscriptions = appCfg.Subscriptions
		}
		if appCfg.BridgeGroups != nil {
			s.bridgeGroups = appCfg.BridgeGroups
		}
		if appCfg.BridgeRules != nil {
			s.bridgeRules = appCfg.BridgeRules
		}
		if appCfg.Window != nil {
			s.window = appCfg.Window
		}
		if appCfg.Hotkey != nil {
			s.hotkey = appCfg.Hotkey
		}
		s.compactMode = appCfg.CompactMode
		s.lastSavedItems = appCfg.Connections
		s.mu.Unlock()

		for _, item := range appCfg.Connections {
			if err := list.AddItemWithSubscription(item.ID, item.Label, item.Link, item.SubscriptionID, item.TotalRead, item.TotalWritten); err != nil {
				slog.Error("failed to load item", "error", err, "label", item.Label)
			}
		}


		// If loaded from alternate or legacy path, sync to primary target
		if loadedFrom != s.filePath {
			slog.Info("Syncing loaded configuration to primary path", "from", loadedFrom, "to", s.filePath)
			_ = saveToDisk(s.filePath, data)
		}
		return
	}

	// Fallback to legacy array format: []SavedState
	loadedItems := make([]SavedState, 0)
	if err := json.Unmarshal(data, &loadedItems); err != nil {
		slog.Error("failed to unmarshal connections file", "error", err, "path", loadedFrom)
		return
	}

	for _, item := range loadedItems {
		if err := list.AddItemWithSubscription(item.ID, item.Label, item.Link, item.SubscriptionID, item.TotalRead, item.TotalWritten); err != nil {
			slog.Error("failed to load item", "error", err, "label", item.Label)
		}
	}
}

// tryMigrateFromFyne reads legacy preferences.json and loads connections.
func (s *SaveFile) tryMigrateFromFyne(list *connlist.Collection) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	// Legacy Fyne preferences path on Linux
	legacyPath := filepath.Join(home, ".config", "fyne", "com.github.goxray.client.app", "preferences.json")
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return
	}

	var rawMap map[string]string
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return
	}

	connStr, ok := rawMap["connections_config"]
	if !ok || connStr == "" {
		return
	}

	var legacyItems []SavedState
	if err := json.Unmarshal([]byte(connStr), &legacyItems); err != nil {
		slog.Error("failed to unmarshal legacy connections", "error", err)
		return
	}

	slog.Info("Migrating legacy connections from Fyne", "count", len(legacyItems))
	for _, item := range legacyItems {
		if err := list.AddItem(item.Label, item.Link); err != nil {
			slog.Error("failed to add migrated item", "error", err, "label", item.Label)
		}
	}

	// Persist migrated items to the new config file
	s.Update(list)
}
