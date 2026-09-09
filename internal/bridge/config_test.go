package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigWatcher(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "connections.json")

	initial := bridgeConfigFile{
		BridgeGroups: []BridgeGroup{{ID: "g1", Name: "Group 1", Enabled: true}},
		BridgeRules:  []BridgeRule{{ID: "r1", Pattern: "curl", Enabled: true}},
	}
	data, err := json.Marshal(initial)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	w := NewConfigWatcher(cfgPath)
	rules := w.GetRules()
	if len(rules) != 1 || rules[0].Pattern != "curl" {
		t.Fatalf("expected 1 rule with pattern 'curl', got %v", rules)
	}
	groups := w.GetGroups()
	if len(groups) != 1 || groups[0].Name != "Group 1" {
		t.Fatalf("expected 1 group with name 'Group 1', got %v", groups)
	}

	// Update file
	time.Sleep(10 * time.Millisecond) // Ensure mtime changes
	updated := bridgeConfigFile{
		BridgeGroups: []BridgeGroup{},
		BridgeRules: []BridgeRule{
			{ID: "r1", Pattern: "curl", Enabled: true},
			{ID: "r2", Pattern: "chrome", Enabled: true},
		},
	}
	data2, _ := json.Marshal(updated)
	if err := os.WriteFile(cfgPath, data2, 0644); err != nil {
		t.Fatal(err)
	}
	// Force mtime in future to guarantee reload detection across filesystems
	futureTime := time.Now().Add(1 * time.Second)
	_ = os.Chtimes(cfgPath, futureTime, futureTime)

	rules2 := w.GetRules()
	if len(rules2) != 2 {
		t.Fatalf("expected 2 rules after update, got %d", len(rules2))
	}
}
