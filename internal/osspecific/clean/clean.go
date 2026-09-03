package clean

// ClearStuckNetwork cleans up any lingering TUN interfaces and custom VPN routing rules left by stuck or killed processes.
func ClearStuckNetwork() error {
	return clearStuckNetworkOS()
}
