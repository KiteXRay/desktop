package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goxray/core/client"
)

func launchAppWithProxy(appName, targetPath string) error {
	socksPort := client.DefaultSocksPort
	httpPort := client.DefaultHTTPPort

	socksProxy := fmt.Sprintf("socks5://127.0.0.1:%d", socksPort)
	httpProxy := fmt.Sprintf("http://127.0.0.1:%d", httpPort)

	var cmd *exec.Cmd

	if targetPath != "" {
		// Clean and validate target path
		cleanedPath := filepath.Clean(targetPath)
		cmd = exec.Command(cleanedPath)
		lower := strings.ToLower(cleanedPath)
		if strings.Contains(lower, "chrome") || strings.Contains(lower, "edge") || strings.Contains(lower, "brave") ||
			strings.Contains(lower, "chromium") || strings.Contains(lower, "opera") || strings.Contains(lower, "vivaldi") ||
			strings.Contains(lower, "yandex") || strings.Contains(lower, "arc") {
			cmd.Args = append(cmd.Args, fmt.Sprintf("--proxy-server=%s", socksProxy))
			if runtime.GOOS == "windows" {
				appBase := strings.TrimSuffix(filepath.Base(cleanedPath), filepath.Ext(cleanedPath))
				userDataDir := filepath.Join(os.TempDir(), fmt.Sprintf("kite_proxy_%s", appBase))
				cmd.Args = append(cmd.Args, fmt.Sprintf("--user-data-dir=%s", userDataDir), "--no-first-run")
			}
		}
	} else {
		switch strings.ToLower(appName) {
		case "chrome", "google-chrome":
			switch runtime.GOOS {
			case "windows":
				userDataDir := filepath.Join(os.TempDir(), "kite_proxy_chrome")
				cmd = exec.Command("cmd", "/c", "start", "", "chrome", fmt.Sprintf("--proxy-server=%s", socksProxy), fmt.Sprintf("--user-data-dir=%s", userDataDir), "--no-first-run")
			case "darwin":
				cmd = exec.Command("open", "-a", "Google Chrome", "--args", fmt.Sprintf("--proxy-server=%s", socksProxy))
			default:
				cmd = exec.Command("google-chrome", fmt.Sprintf("--proxy-server=%s", socksProxy))
			}
		case "edge", "msedge":
			switch runtime.GOOS {
			case "windows":
				userDataDir := filepath.Join(os.TempDir(), "kite_proxy_edge")
				cmd = exec.Command("cmd", "/c", "start", "", "msedge", fmt.Sprintf("--proxy-server=%s", socksProxy), fmt.Sprintf("--user-data-dir=%s", userDataDir), "--no-first-run")
			case "darwin":
				cmd = exec.Command("open", "-a", "Microsoft Edge", "--args", fmt.Sprintf("--proxy-server=%s", socksProxy))
			default:
				cmd = exec.Command("microsoft-edge", fmt.Sprintf("--proxy-server=%s", socksProxy))
			}
		case "firefox":
			switch runtime.GOOS {
			case "windows":
				cmd = exec.Command("cmd", "/c", "start", "", "firefox")
			case "darwin":
				cmd = exec.Command("open", "-a", "Firefox")
			default:
				cmd = exec.Command("firefox")
			}
		case "telegram":
			switch runtime.GOOS {
			case "windows":
				cmd = exec.Command("cmd", "/c", "start", "", "telegram")
			case "darwin":
				cmd = exec.Command("open", "-a", "Telegram")
			default:
				cmd = exec.Command("telegram-desktop")
			}
		default:
			return fmt.Errorf("unknown app: %s", appName)
		}
	}

	cmd.Env = append(os.Environ(),
		fmt.Sprintf("ALL_PROXY=%s", socksProxy),
		fmt.Sprintf("all_proxy=%s", socksProxy),
		fmt.Sprintf("HTTP_PROXY=%s", httpProxy),
		fmt.Sprintf("http_proxy=%s", httpProxy),
		fmt.Sprintf("HTTPS_PROXY=%s", httpProxy),
		fmt.Sprintf("https_proxy=%s", httpProxy),
	)

	return cmd.Start()
}
