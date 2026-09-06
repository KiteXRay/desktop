package subscription

import (
	"encoding/base64"
	"testing"
)

func TestParseSubscriptionContent(t *testing.T) {
	rawLinks := "vless://uuid1@1.1.1.1:443?security=reality#Server1\r\nvmess://uuid2@2.2.2.2:443#Server2\nss://base64@3.3.3.3:8388#Server3\n"
	b64 := base64.StdEncoding.EncodeToString([]byte(rawLinks))

	links := parseSubscriptionContent(b64)
	if len(links) != 3 {
		t.Fatalf("expected 3 links from base64, got %d", len(links))
	}
	if links[0] != "vless://uuid1@1.1.1.1:443?security=reality#Server1" {
		t.Errorf("unexpected link[0]: %s", links[0])
	}
	if links[1] != "vmess://uuid2@2.2.2.2:443#Server2" {
		t.Errorf("unexpected link[1]: %s", links[1])
	}

	// Plaintext fallback
	linksPlain := parseSubscriptionContent(rawLinks)
	if len(linksPlain) != 3 {
		t.Fatalf("expected 3 links from plaintext, got %d", len(linksPlain))
	}
}

func TestExtractLabelFromLink(t *testing.T) {
	label := ExtractLabelFromLink("vless://user@host:443#My%20Server")
	if label != "My Server" {
		t.Errorf("expected 'My Server', got %q", label)
	}

	label2 := ExtractLabelFromLink("vless://user@host:443")
	if label2 != "vless-host:443" {
		t.Errorf("expected 'vless-host:443', got %q", label2)
	}
}

func TestIsSubscriptionURL(t *testing.T) {
	if !IsSubscriptionURL("https://example.com/sub/123") {
		t.Errorf("expected true for https url")
	}
	if !IsSubscriptionURL("http://192.168.1.1/sub") {
		t.Errorf("expected true for http url")
	}
	if IsSubscriptionURL("vless://uuid@host:443") {
		t.Errorf("expected false for vless url")
	}
}

func TestExtractSubIDFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://185.102.139.121:2096/sub/gjyv3r3la5ptq1lk", "gjyv3r3la5ptq1lk"},
		{"https://185.102.139.121:2096/sub/gjyv3r3la5ptq1lk/", "gjyv3r3la5ptq1lk"},
		{"https://example.com/api/v1/client/subscribe?token=tok12345", "tok12345"},
		{"https://example.com/clash/clash-token-99", "clash-token-99"},
		{"https://example.com/subscription/mysub.json", "mysub"},
	}

	for _, tc := range tests {
		got := ExtractSubIDFromURL(tc.url)
		if got != tc.expected {
			t.Errorf("ExtractSubIDFromURL(%q) = %q, expected %q", tc.url, got, tc.expected)
		}
	}
}

func TestExtractSubIDFromHTML(t *testing.T) {
	htmlContent := []byte(`<template id="subscription-data" data-sid="test-sub-id-abc" data-sub-url="https://domain/sub/test-sub-id-abc"></template>`)
	got := ExtractSubID("https://domain/some-page", htmlContent, nil)
	if got != "test-sub-id-abc" {
		t.Errorf("ExtractSubID from HTML = %q, expected 'test-sub-id-abc'", got)
	}
}

func TestParseSubscriptionHTMLWithTextarea(t *testing.T) {
	htmlContent := `<html><body>
<textarea id="subscription-links" style="display:none">vless://uuid1@1.1.1.1:443?security=reality#Server1
vmess://uuid2@2.2.2.2:443#Server2</textarea>
</body></html>`

	links := parseSubscriptionContent(htmlContent)
	if len(links) != 2 {
		t.Fatalf("expected 2 links extracted from HTML textarea, got %d", len(links))
	}
}
