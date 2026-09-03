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
