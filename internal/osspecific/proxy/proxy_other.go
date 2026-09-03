//go:build !linux && !windows && !darwin

package proxy

func SetSystemProxy(enabled bool, host string, httpPort, socksPort int) error {
	return nil
}

func IsSystemProxyEnabled() bool {
	return false
}
