//go:build windows

package proxy

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

var (
	wininet               = syscall.NewLazyDLL("wininet.dll")
	procInternetSetOption = wininet.NewProc("InternetSetOptionW")
)

const (
	INTERNET_OPTION_SETTINGS_CHANGED = 39
	INTERNET_OPTION_REFRESH          = 37
)

func SetSystemProxy(enabled bool, host string, httpPort, socksPort int) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if !enabled {
		_ = k.SetDWordValue("ProxyEnable", 0)
	} else {
		proxyServer := fmt.Sprintf("http=%s:%d;https=%s:%d;socks=%s:%d", host, httpPort, host, httpPort, host, socksPort)
		_ = k.SetStringValue("ProxyServer", proxyServer)
		_ = k.SetStringValue("ProxyOverride", "<local>;localhost;127.*;10.*;172.16.*;192.168.*")
		_ = k.SetDWordValue("ProxyEnable", 1)
	}

	// Notify WinINet that proxy settings changed so browsers and applications reload immediately
	_, _, _ = procInternetSetOption.Call(0, uintptr(INTERNET_OPTION_SETTINGS_CHANGED), 0, 0)
	_, _, _ = procInternetSetOption.Call(0, uintptr(INTERNET_OPTION_REFRESH), 0, 0)

	return nil
}

func IsSystemProxyEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	val, _, err := k.GetIntegerValue("ProxyEnable")
	return err == nil && val == 1
}
