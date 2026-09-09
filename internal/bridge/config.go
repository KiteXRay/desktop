package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type bridgeConfigFile struct {
	BridgeGroups []BridgeGroup `json:"bridgeGroups"`
	BridgeRules  []BridgeRule  `json:"bridgeRules"`
}

// ConfigWatcher periodically or on-demand reloads bridge rules and groups when the file changes.
type ConfigWatcher struct {
	path    string
	mu      sync.RWMutex
	lastMod time.Time
	rules   []BridgeRule
	groups  []BridgeGroup
}

// NewConfigWatcher creates a watcher for bridge rules at the specified path (or default paths if empty).
func NewConfigWatcher(path string) *ConfigWatcher {
	resolved := path
	if resolved == "" {
		resolved = resolveDefaultConfigPath()
	}

	w := &ConfigWatcher{
		path:   resolved,
		rules:  make([]BridgeRule, 0),
		groups: make([]BridgeGroup, 0),
	}
	w.reload()
	return w
}

func resolveDefaultConfigPath() string {
	candidates := []string{}
	if cfgDir, err := os.UserConfigDir(); err == nil && cfgDir != "" {
		candidates = append(candidates, filepath.Join(cfgDir, "kite", "connections.json"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates,
			filepath.Join(home, ".config", "kite", "connections.json"),
			filepath.Join(home, ".local", "share", "kite", "connections.json"),
		)
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return "connections.json"
}

func (w *ConfigWatcher) reload() {
	if w.path == "" {
		return
	}

	fi, err := os.Stat(w.path)
	if err != nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if !fi.ModTime().After(w.lastMod) && !w.lastMod.IsZero() {
		return
	}

	data, err := os.ReadFile(w.path)
	if err != nil {
		return
	}

	var cfg bridgeConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}

	w.lastMod = fi.ModTime()
	w.rules = cfg.BridgeRules
	w.groups = cfg.BridgeGroups
}

// GetRules returns the currently active bridge rules, re-reading from disk if the config file was updated.
func (w *ConfigWatcher) GetRules() []BridgeRule {
	w.reload()
	w.mu.RLock()
	defer w.mu.RUnlock()

	res := make([]BridgeRule, len(w.rules))
	copy(res, w.rules)
	return res
}

// GetGroups returns the currently active bridge groups, re-reading from disk if the config file was updated.
func (w *ConfigWatcher) GetGroups() []BridgeGroup {
	w.reload()
	w.mu.RLock()
	defer w.mu.RUnlock()

	res := make([]BridgeGroup, len(w.groups))
	copy(res, w.groups)
	return res
}
