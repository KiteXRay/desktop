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
