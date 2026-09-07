package bridge

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type BridgeGroup struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type BridgeRule struct {
	ID          string `json:"id"`
	GroupID     string `json:"groupId,omitempty"`
	Pattern     string `json:"pattern"`     // e.g. "Discovery*.exe", "curl"
	ProxyTarget string `json:"proxyTarget"` // e.g. "socks5://127.0.0.1:10808", "http://127.0.0.1:10809"
	ProxyType   string `json:"proxyType"`   // "socks5" | "http"
	Enabled     bool   `json:"enabled"`
	Description string `json:"description,omitempty"`
}

// CompilePattern turns a user pattern (which may be a wildcard like "Discovery*.exe" or a regex) into a compiled Regexp.
func CompilePattern(pattern string) (*regexp.Regexp, error) {
	trimmed := strings.TrimSpace(pattern)
	if trimmed == "" {
		return nil, fmt.Errorf("empty pattern")
	}

	var regexStr string
	// Check if user specified explicit regex or wildcard
	if strings.ContainsAny(trimmed, "*?") && !strings.ContainsAny(trimmed, "^$[]()\\") {
		// Convert simple wildcard to regex
		escaped := regexp.QuoteMeta(trimmed)
		escaped = strings.ReplaceAll(escaped, "\\*", ".*")
		escaped = strings.ReplaceAll(escaped, "\\?", ".")
		regexStr = "(?i)^" + escaped + "$"
	} else if !strings.HasPrefix(trimmed, "^") && !strings.HasSuffix(trimmed, "$") && !strings.Contains(trimmed, ".*") {
		// Literal name matching (e.g. "curl", "chrome.exe")
		escaped := regexp.QuoteMeta(trimmed)
		regexStr = "(?i)^" + escaped + "$"
	} else {
		// Direct regex
		if !strings.HasPrefix(trimmed, "(?i)") {
			regexStr = "(?i)" + trimmed
		} else {
			regexStr = trimmed
		}
	}

	return regexp.Compile(regexStr)
}

// Matches returns true if the candidate process name or path matches the rule pattern.
func (r *BridgeRule) Matches(candidate string) bool {
	if !r.Enabled {
		return false
	}
	re, err := CompilePattern(r.Pattern)
	if err != nil {
		return false
	}

	// Match against candidate (full path or base name, normalizing both slash types)
	norm := strings.ReplaceAll(candidate, "\\", "/")
	idx := strings.LastIndex(norm, "/")
	base := norm
	if idx >= 0 && idx < len(norm)-1 {
		base = norm[idx+1:]
	}
	baseNoExt := strings.TrimSuffix(base, filepath.Ext(base))

	return re.MatchString(base) ||
		re.MatchString(candidate) ||
		re.MatchString(norm) ||
		re.MatchString(baseNoExt) ||
		re.MatchString(baseNoExt+".exe")
}

// LaunchWithProxy launches a command or executable configured to route through the specified proxy target.
func LaunchWithProxy(targetExe, proxyTarget, proxyType string) error {
	if proxyTarget == "" {
		if proxyType == "http" {
			proxyTarget = "http://127.0.0.1:10809"
		} else {
			proxyTarget = "socks5://127.0.0.1:10808"
		}
	}

	cleanedPath := filepath.Clean(strings.TrimSpace(targetExe))
	cmd := exec.Command(cleanedPath)

	lower := strings.ToLower(cleanedPath)
	if strings.Contains(lower, "chrome") || strings.Contains(lower, "edge") || strings.Contains(lower, "brave") ||
		strings.Contains(lower, "chromium") || strings.Contains(lower, "opera") || strings.Contains(lower, "vivaldi") ||
		strings.Contains(lower, "yandex") || strings.Contains(lower, "arc") {
		cmd.Args = append(cmd.Args, fmt.Sprintf("--proxy-server=%s", proxyTarget))
		if runtime.GOOS == "windows" {
			appBase := strings.TrimSuffix(filepath.Base(cleanedPath), filepath.Ext(cleanedPath))
			userDataDir := filepath.Join(os.TempDir(), fmt.Sprintf("kite_proxy_%s", appBase))
			cmd.Args = append(cmd.Args, fmt.Sprintf("--user-data-dir=%s", userDataDir), "--no-first-run")
		}
	}

	cmd.Env = append(os.Environ(),
		fmt.Sprintf("ALL_PROXY=%s", proxyTarget),
		fmt.Sprintf("all_proxy=%s", proxyTarget),
		fmt.Sprintf("HTTP_PROXY=%s", proxyTarget),
		fmt.Sprintf("http_proxy=%s", proxyTarget),
		fmt.Sprintf("HTTPS_PROXY=%s", proxyTarget),
		fmt.Sprintf("https_proxy=%s", proxyTarget),
	)

	return cmd.Start()
}
