package main

import (
	"encoding/json"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKiteTunnel_CheckFlag(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "--check")
	out, err := cmd.CombinedOutput()
	// In unprivileged CI / sandbox / user environment, --check returns code 1 and privilege failure message
	if err != nil {
		require.Contains(t, string(out), "privilege check failed")
	}
}

func TestKiteTunnel_EventSerialization(t *testing.T) {
	ev := Event{
		Event:     "ready",
		Interface: "kite0",
		BytesIn:   1024,
		BytesOut:  2048,
	}

	data, err := json.Marshal(ev)
	require.NoError(t, err)

	var parsed Event
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)
	require.Equal(t, "ready", parsed.Event)
	require.Equal(t, "kite0", parsed.Interface)
	require.Equal(t, int64(1024), parsed.BytesIn)
	require.Equal(t, int64(2048), parsed.BytesOut)
}

func TestParseBypassIPs(t *testing.T) {
	res := parseBypassIPs("198.51.100.1", "203.0.113.5, 198.51.100.1, 192.0.2.1/32, 2001:db8::1, invalid-ip")
	require.Equal(t, []string{"198.51.100.1", "203.0.113.5", "192.0.2.1"}, res)

	resEmpty := parseBypassIPs("", "")
	require.Empty(t, resEmpty)
}


