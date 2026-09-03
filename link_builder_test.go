package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildLinkFromMap_VLESS(t *testing.T) {
	cfg := map[string]string{
		"Protocol":       "vless",
		"Address":        "127.0.0.1",
		"Port":           "8080",
		"ID":             "h1px412i-9138-s9m5-9b86-d47d74dd8541",
		"Remark":         "TestVless",
		"Security":       "reality",
		"Network":        "tcp",
		"Flow":           "xtls-rprx-vision",
		"SNI":            "example.com",
		"TlsFingerprint": "chrome",
		"Pbk":            "4442383675fc0fb574c3e50abbe7d4c5",
		"Sid":            "0c",
		"Spx":            "/",
	}

	link, err := buildLinkFromMap(cfg)
	require.NoError(t, err)
	assert.Contains(t, link, "vless://")
	assert.Contains(t, link, "security=reality")
	assert.Contains(t, link, "sni=example.com")
	assert.Contains(t, link, "TestVless")
}

func TestBuildLinkFromMap_VMess(t *testing.T) {
	cfg := map[string]string{
		"Protocol": "vmess",
		"Address":  "127.0.0.1",
		"Port":     "443",
		"ID":       "h1px412i-9138-s9m5-9b86-d47d74dd8541",
		"Remark":   "TestVmess",
		"Network":  "ws",
		"Path":     "/ws",
	}

	link, err := buildLinkFromMap(cfg)
	require.NoError(t, err)
	assert.Contains(t, link, "vmess://")
}

func TestBuildLinkFromMap_Trojan(t *testing.T) {
	cfg := map[string]string{
		"Protocol": "trojan",
		"Address":  "127.0.0.1",
		"Port":     "443",
		"ID":       "secretpassword",
		"Remark":   "TestTrojan",
		"SNI":      "example.com",
	}

	link, err := buildLinkFromMap(cfg)
	require.NoError(t, err)
	assert.Contains(t, link, "trojan://secretpassword@127.0.0.1:443")
}
