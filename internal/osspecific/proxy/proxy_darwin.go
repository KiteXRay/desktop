//go:build darwin

package proxy

import (
	"bufio"
	"bytes"
	"os/exec"
	"strconv"
	"strings"
)

// getActiveServices returns primary macOS network services (e.g. "Wi-Fi", "Ethernet")
func getActiveServices() []string {
	out, err := exec.Command("networksetup", "-listnetworkserviceorder").Output()
	if err != nil {
		return []string{"Wi-Fi", "Ethernet"}
	}

	var services []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "(Hardware Port:") {
			// Format: (Hardware Port: Wi-Fi, Device: en0) -> preceding line had (1) Wi-Fi
			continue
		}
		if strings.HasPrefix(line, "(") && strings.Contains(line, ") ") {
			parts := strings.SplitN(line, ") ", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				if name != "" && !strings.Contains(name, "*") {
					services = append(services, name)
				}
			}
		}
	}

	if len(services) == 0 {
		services = []string{"Wi-Fi", "Ethernet"}
	}
	return services
}

func SetSystemProxy(enabled bool, host string, httpPort, socksPort int) error {
	services := getActiveServices()
	hPort := strconv.Itoa(httpPort)
	sPort := strconv.Itoa(socksPort)

	for _, svc := range services {
		if enabled {
			_ = exec.Command("networksetup", "-setwebproxy", svc, host, hPort).Run()
			_ = exec.Command("networksetup", "-setsecurewebproxy", svc, host, hPort).Run()
			_ = exec.Command("networksetup", "-setsocksfirewallproxy", svc, host, sPort).Run()

			_ = exec.Command("networksetup", "-setwebproxystate", svc, "on").Run()
			_ = exec.Command("networksetup", "-setsecurewebproxystate", svc, "on").Run()
			_ = exec.Command("networksetup", "-setsocksfirewallproxystate", svc, "on").Run()
		} else {
			_ = exec.Command("networksetup", "-setwebproxystate", svc, "off").Run()
			_ = exec.Command("networksetup", "-setsecurewebproxystate", svc, "off").Run()
			_ = exec.Command("networksetup", "-setsocksfirewallproxystate", svc, "off").Run()
		}
	}
	return nil
}

func IsSystemProxyEnabled() bool {
	services := getActiveServices()
	for _, svc := range services {
		out, err := exec.Command("networksetup", "-getwebproxy", svc).Output()
		if err == nil && strings.Contains(string(out), "Enabled: Yes") {
			return true
		}
	}
	return false
}
