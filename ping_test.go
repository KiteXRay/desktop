package main

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPingTarget_Success(t *testing.T) {
	// Start a local TCP server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	host, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	latency := pingTarget(host, port, 1*time.Second)
	assert.GreaterOrEqual(t, latency, int64(1))
}

func TestPingTarget_Empty(t *testing.T) {
	assert.Equal(t, int64(-1), pingTarget("", "443", 500*time.Millisecond))
	assert.Equal(t, int64(-1), pingTarget("127.0.0.1", "", 500*time.Millisecond))
}

func TestPingTarget_Unreachable(t *testing.T) {
	// Closed port on 127.0.0.1 should fail with ECONNREFUSED
	latency := pingTarget("127.0.0.1", "59998", 100*time.Millisecond)
	assert.Equal(t, int64(-1), latency)

	// Non-existent domain should fail DNS resolution
	latency = pingTarget("nonexistent.invalid", "443", 100*time.Millisecond)
	assert.Equal(t, int64(-1), latency)
}
