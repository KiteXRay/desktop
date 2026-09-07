package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/KiteXRay/desktop/internal/bridge"
	"github.com/KiteXRay/desktop/internal/connlist"
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

type AppConfigFile struct {
	TunnelMode     string                      `json:"tunnelMode"`
	TunnelDeviceIP string                      `json:"tunnelDeviceIp,omitempty"`
	TunnelDNS      string                      `json:"tunnelDns,omitempty"`
	Subscriptions  []subscription.Subscription `json:"subscriptions,omitempty"`
	BridgeGroups   []bridge.BridgeGroup        `json:"bridgeGroups,omitempty"`
	BridgeRules    []bridge.BridgeRule         `json:"bridgeRules,omitempty"`
	Connections    []SavedState                `json:"connections"`
}

type SaveFile struct {
	filePath       string
	tunnelMode     string
	tunnelDeviceIP string
	tunnelDNS      string
	subscriptions  []subscription.Subscription
	bridgeGroups   []bridge.BridgeGroup
	bridgeRules    []bridge.BridgeRule
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

	return &SaveFile{
		filePath:       path,
		tunnelMode:     "tunnel",
		tunnelDeviceIP: "192.18.0.1",
		tunnelDNS:      "8.8.8.8",
		subscriptions:  make([]subscription.Subscription, 0),
		bridgeGroups:   make([]bridge.BridgeGroup, 0),
		bridgeRules:    make([]bridge.BridgeRule, 0),
	}
}

func NewSaveFile() *SaveFile {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}

	appConfigDir := filepath.Join(configDir, configSubdir)
	return NewSaveFileWithPath(filepath.Join(appConfigDir, configFileName))
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

	dir := filepath.Dir(s.filePath)
	_ = os.MkdirAll(dir, 0755)
	tmpFile, err := os.CreateTemp(dir, "connections-*.tmp")
	if err != nil {
		// Fallback to direct write if temp file creation fails
		if err := os.WriteFile(s.filePath, b, 0644); err != nil {
			slog.Error("failed to write connections file", "error", err, "path", s.filePath)
		}
		return
	}
	tmpName := tmpFile.Name()

	if _, err := tmpFile.Write(b); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		slog.Error("failed to write temp connections file", "error", err)
		return
	}
	_ = tmpFile.Sync()
	_ = tmpFile.Close()

	if err := os.Rename(tmpName, s.filePath); err != nil {
		_ = os.Remove(tmpName)
		slog.Error("failed to replace connections file", "error", err, "path", s.filePath)
	}
}

// Load loads saved items into list, migrating from Fyne preferences or legacy array if needed.
func (s *SaveFile) Load(list *connlist.Collection) {
	// If config file does not exist, check for existing legacy goxray config or Fyne preferences
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		configDir := filepath.Dir(filepath.Dir(s.filePath))
		legacyGoxrayPath := filepath.Join(configDir, "goxray", configFileName)
		if legacyData, err := os.ReadFile(legacyGoxrayPath); err == nil && len(legacyData) > 0 {
			_ = os.WriteFile(s.filePath, legacyData, 0644)
		} else {
			s.tryMigrateFromFyne(list)
			return
		}
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		slog.Error("failed to read connections file", "error", err)
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
		s.mu.Unlock()

		for _, item := range appCfg.Connections {
			if err := list.AddItemWithSubscription(item.ID, item.Label, item.Link, item.SubscriptionID, item.TotalRead, item.TotalWritten); err != nil {
				slog.Error("failed to load item", "error", err, "label", item.Label)
			}
		}
		return
	}

	// Fallback to legacy array format: []SavedState
	loadedItems := make([]SavedState, 0)
	if err := json.Unmarshal(data, &loadedItems); err != nil {
		slog.Error("failed to unmarshal connections file", "error", err)
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
