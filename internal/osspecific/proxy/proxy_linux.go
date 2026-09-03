//go:build linux

package proxy

import (
	"os/exec"
	"strconv"
	"strings"
)

func SetSystemProxy(enabled bool, host string, httpPort, socksPort int) error {
	if !enabled {
		cmd := exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "none")
		return cmd.Run()
	}

	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "manual").Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.http", "host", host).Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.http", "port", strconv.Itoa(httpPort)).Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.https", "host", host).Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.https", "port", strconv.Itoa(httpPort)).Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.socks", "host", host).Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.socks", "port", strconv.Itoa(socksPort)).Run()

	return nil
}

func IsSystemProxyEnabled() bool {
	out, err := exec.Command("gsettings", "get", "org.gnome.system.proxy", "mode").Output()
	if err != nil {
		return false
	}
	val := strings.TrimSpace(string(out))
	return val == "'manual'" || val == "manual"
}
