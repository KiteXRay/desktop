package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/KiteXRay/desktop/internal/connlist"
)

const (
	configSubdir   = "kite"
	configFileName = "connections.json"
)

type SavedState struct {
	ID           string `json:"id,omitempty"`
	Link         string `json:"link"`
	Label        string `json:"label"`
	TotalRead    int64  `json:"totalRead,omitempty"`
	TotalWritten int64  `json:"totalWritten,omitempty"`
}

type AppConfigFile struct {
	TunnelMode  string       `json:"tunnelMode"`
	Connections []SavedState `json:"connections"`
}

type SaveFile struct {
	filePath   string
	tunnelMode string
	mu         sync.Mutex
}

func serialize(item *connlist.Item) SavedState {
	return SavedState{
		ID:           item.ID(),
		Link:         item.Link(),
		Label:        item.Label(),
		TotalRead:    item.BytesRead(),
		TotalWritten: item.BytesWritten(),
	}
}

func NewSaveFile() *SaveFile {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}

	appConfigDir := filepath.Join(configDir, configSubdir)
	_ = os.MkdirAll(appConfigDir, 0755)

	return &SaveFile{
		filePath:   filepath.Join(appConfigDir, configFileName),
		tunnelMode: "system",
	}
}

func (s *SaveFile) GetTunnelMode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tunnelMode == "" {
		return "system"
	}
	return s.tunnelMode
}

func (s *SaveFile) SetTunnelMode(mode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if mode == "per_app" {
		s.tunnelMode = "per_app"
	} else {
		s.tunnelMode = "system"
	}
}

// Update saves list and current tunnel mode atomically into JSON file.
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
		tMode = "system"
	}

	cfg := AppConfigFile{
		TunnelMode:  tMode,
		Connections: toSave,
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		slog.Error("failed to marshal config", "error", err)
		return
	}

	dir := filepath.Dir(s.filePath)
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
	if err := json.Unmarshal(data, &appCfg); err == nil && (len(appCfg.Connections) > 0 || appCfg.TunnelMode != "") {
		s.mu.Lock()
		if appCfg.TunnelMode != "" {
			s.tunnelMode = appCfg.TunnelMode
		}
		s.mu.Unlock()

		for _, item := range appCfg.Connections {
			if err := list.AddItemWithID(item.ID, item.Label, item.Link, item.TotalRead, item.TotalWritten); err != nil {
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
		if err := list.AddItemWithID(item.ID, item.Label, item.Link, item.TotalRead, item.TotalWritten); err != nil {
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
