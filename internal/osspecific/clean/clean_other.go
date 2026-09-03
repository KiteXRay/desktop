//go:build !linux && !darwin && !windows

package clean

func clearStuckNetworkOS() error {
	return nil
}
