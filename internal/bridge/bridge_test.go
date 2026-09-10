package bridge

import (
	"testing"
)

func TestBridgeRule_Matches(t *testing.T) {
	tests := []struct {
		pattern   string
		candidate string
		expected  bool
	}{
		// Wildcard tests
		{"Discovery*.exe", "Discovery.exe", true},
		{"Discovery*.exe", "Discovery-d.exe", true},
		{"Discovery*.exe", "C:\\Games\\Discovery\\Discovery.exe", true},
		{"Discovery*.exe", "other.exe", false},

		// Literal tests
		{"curl", "curl", true},
		{"curl", "/usr/bin/curl", true},
		{"curl", "C:\\tools\\curl.exe", true}, // literal "curl" matches "curl.exe"
		{"curl.exe", "curl", true},            // "curl.exe" matches "curl"
		{"curl*", "curl.exe", true},

		// Case-insensitivity
		{"CHROME.EXE", "chrome.exe", true},
		{"chrome.exe", "CHROME.EXE", true},

		// Regex
		{"^Dis[cx]overy.*\\.exe$", "Disxovery-d.exe", true},
		{"^Dis[cx]overy.*\\.exe$", "Discovery.exe", true},
	}

	for _, tt := range tests {
		rule := BridgeRule{
			ID:      "test",
			Pattern: tt.pattern,
			Enabled: true,
		}
		res := rule.Matches(tt.candidate)
		if res != tt.expected {
			t.Errorf("Matches(%q, %q) = %v; expected %v", tt.pattern, tt.candidate, res, tt.expected)
		}
	}
}

func TestGetRunningProcesses(t *testing.T) {
	procs, err := GetRunningProcesses()
	if err != nil {
		t.Fatalf("GetRunningProcesses failed: %v", err)
	}
	if len(procs) == 0 {
		t.Errorf("expected at least 1 running process")
	}
}

func TestCheckRunningProcesses_DisabledGroup(t *testing.T) {
	procs, err := GetRunningProcesses()
	if err != nil || len(procs) == 0 {
		t.Skip("skipping process check test: no processes returned")
	}

	firstProc := procs[0]
	rules := []BridgeRule{
		{ID: "r1", GroupID: "g1", Pattern: firstProc, Enabled: true},
		{ID: "r2", GroupID: "g2", Pattern: firstProc, Enabled: true},
	}
	groups := []BridgeGroup{
		{ID: "g1", Name: "Active Group", Enabled: true},
		{ID: "g2", Name: "Disabled Group", Enabled: false},
	}

	counts := CheckRunningProcesses(rules, groups)
	if counts["r1"] == 0 {
		t.Errorf("expected r1 to have matched running process, got 0")
	}
	if counts["r2"] != 0 {
		t.Errorf("expected r2 in disabled group to have count 0, got %d", counts["r2"])
	}
}

func TestBridgeDialer_DisabledGroup(t *testing.T) {
	rules := []BridgeRule{
		{ID: "r1", GroupID: "g1", Pattern: "test.exe", Enabled: true, ProxyTarget: "socks5://127.0.0.1:10808"},
	}
	groups := []BridgeGroup{
		{ID: "g1", Name: "Test Group", Enabled: false},
	}

	dialer := NewBridgeDialer("", func() []BridgeRule { return rules }, nil, func() []BridgeGroup { return groups })
	matched := dialer.matchRule("test.exe")
	if matched != nil {
		t.Errorf("expected nil rule match because group is disabled, got %v", matched)
	}

	// Enable group and test again
	groups[0].Enabled = true
	matched = dialer.matchRule("test.exe")
	if matched == nil {
		t.Errorf("expected rule match after group enabled, got nil")
	}
}

func TestIsExcludedInterface(t *testing.T) {
	excluded := []string{
		"utun0", "utun3", "tun0", "tap0", "wintun", "kite0",
		"awdl0", "llw0", "bridge0", "gif0", "stf0", "anpi0",
	}
	for _, name := range excluded {
		if !isExcludedInterface(name) {
			t.Errorf("expected %q to be excluded", name)
		}
	}

	allowed := []string{
		"en0", "en1", "eth0", "wlan0", "eno1", "wlp2s0",
	}
	for _, name := range allowed {
		if isExcludedInterface(name) {
			t.Errorf("expected %q to NOT be excluded", name)
		}
	}
}

func TestBridgeDirectProxy_Creation(t *testing.T) {
	px := newBridgeDirectProxy()
	if px == nil {
		t.Fatalf("expected non-nil direct proxy")
	}
	if px.Addr() != "" {
		t.Errorf("expected empty Addr() for direct, got %q", px.Addr())
	}
}

func TestBridgeBypass_StoreAndCleanup(t *testing.T) {
	boundInterfaceIndex.Store(42)
	name := "en0"
	boundInterfaceName.Store(&name)

	CleanupBridgeBypass()

	if boundInterfaceIndex.Load() != 0 {
		t.Errorf("expected boundInterfaceIndex 0 after cleanup, got %d", boundInterfaceIndex.Load())
	}
	if boundInterfaceName.Load() != nil {
		t.Errorf("expected boundInterfaceName nil after cleanup, got %v", boundInterfaceName.Load())
	}
}

