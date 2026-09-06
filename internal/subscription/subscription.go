package subscription

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Subscription struct {
	ID          string `json:"id"`
	SubID       string `json:"subId"`
	URL         string `json:"url"`
	Label       string `json:"label"`
	Count       int    `json:"count"`
	LastUpdated int64  `json:"lastUpdated"`
	UserInfo    string `json:"userInfo,omitempty"`
}

var supportedSchemes = []string{
	"vless://",
	"vmess://",
	"ss://",
	"trojan://",
	"tuic://",
	"hysteria2://",
	"hysteria://",
	"wireguard://",
}

var (
	pathSubIDRegex    = regexp.MustCompile(`(?i)/(?:sub|subscribe|subscription|clash|link|client)/([a-zA-Z0-9_-]+)`)
	dataSidRegex      = regexp.MustCompile(`(?i)data-sid=["']([^"']+)["']`)
	dataSubUrlRegex   = regexp.MustCompile(`(?i)data-sub-url=["'][^"']*/sub/([a-zA-Z0-9_-]+)`)
	dataSubClashRegex = regexp.MustCompile(`(?i)data-subclash-url=["'][^"']*/clash/([a-zA-Z0-9_-]+)`)
	jsonSidRegex      = regexp.MustCompile(`(?i)["'](?:sid|subId|sub_id)["']\s*:\s*["']([^"']+)["']`)
	bodySubRegex      = regexp.MustCompile(`(?i)/(?:sub|clash|subscribe|subscription)/([a-zA-Z0-9_-]{6,})`)
	linkRegex         = regexp.MustCompile(`(?i)(?:vless|vmess|ss|trojan|tuic|hysteria2?|wireguard)://[^\s<>"']+`)
)

// IsSubscriptionURL returns true if the URL starts with http:// or https://.
func IsSubscriptionURL(input string) bool {
	trimmed := strings.TrimSpace(input)
	return strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://")
}

// IsConnectionLink returns true if the input starts with any supported proxy protocol scheme.
func IsConnectionLink(input string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	for _, scheme := range supportedSchemes {
		if strings.HasPrefix(trimmed, scheme) {
			return true
		}
	}
	return false
}

// ExtractLabelFromLink extracts the remark/label from a proxy connection URL.
func ExtractLabelFromLink(link string) string {
	u, err := url.Parse(strings.TrimSpace(link))
	if err == nil && u.Fragment != "" {
		if unescaped, err := url.QueryUnescape(u.Fragment); err == nil && unescaped != "" {
			return unescaped
		}
		return u.Fragment
	}
	// Fallback to scheme + host
	if u != nil && u.Host != "" {
		return fmt.Sprintf("%s-%s", u.Scheme, u.Host)
	}
	return "Connection"
}

// ExtractSubIDFromURL attempts to extract the subscription identifier from URL query or path.
func ExtractSubIDFromURL(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	// 1. Check known query parameters
	for _, qKey := range []string{"token", "subid", "sub_id", "id", "sid", "sub", "uuid", "key"} {
		if val := u.Query().Get(qKey); val != "" {
			return strings.TrimSpace(val)
		}
	}
	// 2. Check path pattern e.g. /sub/{subId}, /clash/{subId}, /subscription/{subId}
	if m := pathSubIDRegex.FindStringSubmatch(u.Path); len(m) > 1 {
		val := cleanSubID(m[1])
		if val != "" {
			return val
		}
	}
	return ""
}

func cleanSubID(val string) string {
	val = strings.TrimSpace(val)
	val = strings.TrimSuffix(val, ".json")
	val = strings.TrimSuffix(val, ".yaml")
	val = strings.TrimSuffix(val, ".yml")
	val = strings.TrimSuffix(val, ".txt")
	return val
}

// ExtractSubID extracts the subscription identifier from the URL, HTTP headers, or response body.
func ExtractSubID(subURL string, body []byte, headers http.Header) string {
	// 1. Extract from the main URL
	if sid := ExtractSubIDFromURL(subURL); sid != "" {
		return sid
	}
	// 2. Extract from response headers (e.g. Profile-Web-Page-Url)
	if headers != nil {
		if webURL := headers.Get("Profile-Web-Page-Url"); webURL != "" {
			if sid := ExtractSubIDFromURL(webURL); sid != "" {
				return sid
			}
		}
	}
	// 3. Extract from response body (HTML attributes, JSON, or path references)
	if len(body) > 0 {
		strBody := string(body)
		if m := dataSidRegex.FindStringSubmatch(strBody); len(m) > 1 && m[1] != "" {
			return cleanSubID(m[1])
		}
		if m := dataSubUrlRegex.FindStringSubmatch(strBody); len(m) > 1 && m[1] != "" {
			return cleanSubID(m[1])
		}
		if m := dataSubClashRegex.FindStringSubmatch(strBody); len(m) > 1 && m[1] != "" {
			return cleanSubID(m[1])
		}
		if m := jsonSidRegex.FindStringSubmatch(strBody); len(m) > 1 && m[1] != "" {
			return cleanSubID(m[1])
		}
		if m := bodySubRegex.FindStringSubmatch(strBody); len(m) > 1 && m[1] != "" {
			return cleanSubID(m[1])
		}
	}

	// 4. Fallback check for URL trailing path segment if length >= 8
	if u, err := url.Parse(strings.TrimSpace(subURL)); err == nil {
		cleanPath := strings.Trim(u.Path, "/")
		if cleanPath != "" {
			parts := strings.Split(cleanPath, "/")
			lastPart := cleanSubID(parts[len(parts)-1])
			commonKeywords := map[string]bool{
				"sub": true, "subscribe": true, "subscription": true, "clash": true,
				"link": true, "links": true, "client": true, "api": true,
				"v1": true, "v2": true, "get": true, "download": true, "config": true,
			}
			if len(lastPart) >= 8 && !commonKeywords[strings.ToLower(lastPart)] {
				return lastPart
			}
		}
	}

	return ""
}

// FetchSubscription retrieves and parses connection links from a subscription URL.
func FetchSubscription(subURL, customLabel string) ([]string, *Subscription, error) {
	subURL = strings.TrimSpace(subURL)
	if !IsSubscriptionURL(subURL) {
		return nil, nil, errors.New("invalid subscription URL: must start with http:// or https://")
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Allow IP-based subscription endpoints with self-signed/domain certs
			},
		},
	}

	req, err := http.NewRequest("GET", subURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create subscription request: %w", err)
	}
	req.Header.Set("User-Agent", "v2rayN/6.23")
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("subscription returned HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read subscription body: %w", err)
	}

	userInfo := resp.Header.Get("Subscription-Userinfo")

	rawContent := strings.TrimSpace(string(body))
	links := parseSubscriptionContent(rawContent)
	if len(links) == 0 {
		return nil, nil, errors.New("no valid connection links found in subscription response")
	}

	subID := ExtractSubID(subURL, body, resp.Header)

	// If subID still empty, try fetching HTML page directly
	if subID == "" {
		htmlReq, err := http.NewRequest("GET", subURL, nil)
		if err == nil {
			htmlReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
			htmlReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
			if htmlResp, err := client.Do(htmlReq); err == nil {
				defer htmlResp.Body.Close()
				if htmlBody, err := io.ReadAll(io.LimitReader(htmlResp.Body, 1024*1024)); err == nil {
					subID = ExtractSubID(subURL, htmlBody, htmlResp.Header)
				}
			}
		}
	}

	// Fallback deterministic subID from hash if not found
	if subID == "" {
		h := sha256.Sum256([]byte(subURL))
		subID = hex.EncodeToString(h[:4])
	}

	groupName := fmt.Sprintf("Subscription-%s", subID)
	label := groupName
	if customLabel != "" {
		label = customLabel
	}

	sub := &Subscription{
		ID:          "sub-" + uuid.New().String()[:8],
		SubID:       subID,
		URL:         subURL,
		Label:       label,
		Count:       len(links),
		LastUpdated: time.Now().Unix(),
		UserInfo:    userInfo,
	}

	return links, sub, nil
}

// parseSubscriptionContent attempts to decode base64 body, falling back to regex extraction or plaintext lines.
func parseSubscriptionContent(raw string) []string {
	content := raw

	// Attempt base64 decode
	cleaned := strings.ReplaceAll(strings.ReplaceAll(raw, "\r", ""), "\n", "")
	if decoded, err := base64.StdEncoding.DecodeString(cleaned); err == nil && len(decoded) > 0 {
		content = string(decoded)
	} else if decoded, err := base64.RawStdEncoding.DecodeString(cleaned); err == nil && len(decoded) > 0 {
		content = string(decoded)
	} else if decoded, err := base64.URLEncoding.DecodeString(cleaned); err == nil && len(decoded) > 0 {
		content = string(decoded)
	} else if decoded, err := base64.RawURLEncoding.DecodeString(cleaned); err == nil && len(decoded) > 0 {
		content = string(decoded)
	}

	// Unescape HTML entities (e.g. &amp;)
	content = html.UnescapeString(content)

	var validLinks []string
	seen := make(map[string]bool)

	// If content contains HTML tags, extract directly using regex to avoid tag contamination
	if strings.Contains(content, "<") {
		matches := linkRegex.FindAllString(content, -1)
		for _, match := range matches {
			m := strings.TrimSpace(match)
			m = strings.TrimRight(m, "<>\"'\r\n ")
			if IsConnectionLink(m) && !seen[m] {
				seen[m] = true
				validLinks = append(validLinks, m)
			}
		}
		if len(validLinks) > 0 {
			return validLinks
		}
	}

	// Line-by-line parsing for standard plaintext or base64 decoded links
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		for _, scheme := range supportedSchemes {
			if strings.HasPrefix(lower, scheme) {
				if !seen[line] {
					seen[line] = true
					validLinks = append(validLinks, line)
				}
				break
			}
		}
	}

	if len(validLinks) == 0 {
		matches := linkRegex.FindAllString(content, -1)
		for _, match := range matches {
			m := strings.TrimSpace(match)
			m = strings.TrimRight(m, "<>\"'\r\n ")
			if IsConnectionLink(m) && !seen[m] {
				seen[m] = true
				validLinks = append(validLinks, m)
			}
		}
	}

	return validLinks
}
