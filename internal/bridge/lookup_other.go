//go:build !windows && !linux

package bridge

func GetProcessByPort(isTCP bool, port uint16) (string, uint32) {
	return "", 0
}
