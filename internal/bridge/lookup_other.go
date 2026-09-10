//go:build !windows && !linux && !darwin

package bridge

func GetProcessByPort(isTCP bool, port uint16) (string, uint32) {
	return "", 0
}
