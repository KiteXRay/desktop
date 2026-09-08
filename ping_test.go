package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPingRoutedConnection_Empty(t *testing.T) {
	assert.Equal(t, int64(-1), pingRoutedConnection("", 500*time.Millisecond))
}

func TestPingRoutedConnection_InvalidLink(t *testing.T) {
	assert.Equal(t, int64(-1), pingRoutedConnection("invalid://link", 500*time.Millisecond))
	assert.Equal(t, int64(-1), pingRoutedConnection("not a url at all", 500*time.Millisecond))
}

func TestPingRoutedConnection_UnreachableServer(t *testing.T) {
	// Dummy VLESS link pointing to closed local port
	link := "vless://h1px412i-9138-s9m5-9b86-d47d74dd8541@127.0.0.1:59998?type=tcp&security=none#Test"
	latency := pingRoutedConnection(link, 200*time.Millisecond)
	assert.Equal(t, int64(-1), latency)
}

func TestApp_PingConnection_NotFound(t *testing.T) {
	app := NewApp()
	latency := app.PingConnection("non-existent-id")
	assert.Equal(t, int64(-1), latency)
}

func TestExtractServerHost(t *testing.T) {
	vlessLink := "vless://h1px412i-9138-s9m5-9b86-d47d74dd8541@198.51.100.1:443?type=tcp&security=none#Test"
	host, err := extractServerHost(vlessLink)
	assert.NoError(t, err)
	assert.Equal(t, "198.51.100.1", host)

	ip, err := resolveServerIP("198.51.100.1")
	assert.NoError(t, err)
	assert.Equal(t, "198.51.100.1", ip)

	_, err = extractServerHost("invalid://link")
	assert.Error(t, err)
}

func TestPingActiveConnection_Disconnected(t *testing.T) {
	app := NewApp()
	latency := app.pingActiveConnection(100 * time.Millisecond)
	assert.Equal(t, int64(-1), latency)
}

