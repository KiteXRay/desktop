//go:build darwin

package proxy

import (
	"bufio"
	"bytes"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type rawService struct {
	name   string
	device string
}

// getActiveServices returns currently active macOS network services (e.g. "Wi-Fi", "Ethernet")
// by checking which underlying network devices have an active link and IPv4 address.
func getActiveServices() []string {
	out, err := exec.Command("networksetup", "-listnetworkserviceorder").Output()
	if err != nil {
		return []string{"Wi-Fi"}
	}

	var parsed []rawService
	var currentName string

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "(Hardware Port:") {
			// Format: (Hardware Port: Wi-Fi, Device: en0)
			devIdx := strings.Index(line, "Device: ")
			if devIdx != -1 && currentName != "" {
				devStr := line[devIdx+len("Device: "):]
				devStr = strings.TrimRight(devStr, ")")
				devStr = strings.TrimSpace(devStr)
				parsed = append(parsed, rawService{
					name:   currentName,
					device: devStr,
				})
			}
			currentName = ""
			continue
		}
		if strings.HasPrefix(line, "(") && strings.Contains(line, ") ") {
			parts := strings.SplitN(line, ") ", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				if name != "" && !strings.Contains(name, "*") {
					currentName = name
				}
			}
		}
	}

	var active []string
	for _, s := range parsed {
		if s.device == "" {
			continue
		}
		ifc, err := net.InterfaceByName(s.device)
		if err != nil || ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				ip4 := ipNet.IP.To4()
				if ip4 != nil && !ip4.IsLoopback() && !ip4.IsUnspecified() && !ip4.IsLinkLocalUnicast() {
					active = append(active, s.name)
					break
				}
			}
		}
	}

	if len(active) > 0 {
		return active
	}

	// Fallback to first available non-disabled service
	if len(parsed) > 0 {
		return []string{parsed[0].name}
	}

	return []string{"Wi-Fi"}
}

func SetSystemProxy(enabled bool, host string, httpPort, socksPort int) error {
	services := getActiveServices()
	hPort := strconv.Itoa(httpPort)
	sPort := strconv.Itoa(socksPort)

	var wg sync.WaitGroup
	for _, svc := range services {
		svc := svc
		if enabled {
			wg.Add(3)
			go func() {
				defer wg.Done()
				_ = exec.Command("networksetup", "-setwebproxy", svc, host, hPort).Run()
			}()
			go func() {
				defer wg.Done()
				_ = exec.Command("networksetup", "-setsecurewebproxy", svc, host, hPort).Run()
			}()
			go func() {
				defer wg.Done()
				_ = exec.Command("networksetup", "-setsocksfirewallproxy", svc, host, sPort).Run()
			}()
		} else {
			wg.Add(3)
			go func() {
				defer wg.Done()
				_ = exec.Command("networksetup", "-setwebproxystate", svc, "off").Run()
			}()
			go func() {
				defer wg.Done()
				_ = exec.Command("networksetup", "-setsecurewebproxystate", svc, "off").Run()
			}()
			go func() {
				defer wg.Done()
				_ = exec.Command("networksetup", "-setsocksfirewallproxystate", svc, "off").Run()
			}()
		}
	}
	wg.Wait()
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
