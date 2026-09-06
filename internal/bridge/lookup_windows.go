//go:build windows

package bridge

import (
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modiphlpapi             = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable = modiphlpapi.NewProc("GetExtendedTcpTable")
	procGetExtendedUdpTable = modiphlpapi.NewProc("GetExtendedUdpTable")
)

const (
	tcpTableOwnerPidAll = 5
	udpTableOwnerPid    = 1
	afInet              = 2
	afInet6             = 23
)

type mibTcpRowOwnerPid struct {
	dwState      uint32
	dwLocalAddr  uint32
	dwLocalPort  uint32
	dwRemoteAddr uint32
	dwRemotePort uint32
	dwOwningPid  uint32
}

type mibTcp6RowOwnerPid struct {
	ucLocalAddr     [16]byte
	dwLocalScopeId  uint32
	dwLocalPort     uint32
	ucRemoteAddr    [16]byte
	dwRemoteScopeId uint32
	dwRemotePort    uint32
	dwState         uint32
	dwOwningPid     uint32
}

type mibUdpRowOwnerPid struct {
	dwLocalAddr uint32
	dwLocalPort uint32
	dwOwningPid uint32
}

type mibUdp6RowOwnerPid struct {
	ucLocalAddr    [16]byte
	dwLocalScopeId uint32
	dwLocalPort    uint32
	dwOwningPid    uint32
}

func decodePort(dwPort uint32) uint16 {
	return uint16((dwPort&0xFF)<<8 | ((dwPort >> 8) & 0xFF))
}

type cachedProc struct {
	name      string
	expiresAt time.Time
}

var (
	procCacheMu sync.RWMutex
	procCache   = make(map[uint32]cachedProc)
)

func getProcessNameByPID(pid uint32) string {
	if pid == 0 {
		return ""
	}

	procCacheMu.RLock()
	if item, found := procCache[pid]; found && time.Now().Before(item.expiresAt) {
		procCacheMu.RUnlock()
		return item.name
	}
	procCacheMu.RUnlock()

	hProc, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(hProc)

	var buf [windows.MAX_LONG_PATH]uint16
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(hProc, 0, &buf[0], &size); err != nil {
		return ""
	}

	name := filepath.Base(windows.UTF16ToString(buf[:size]))
	procCacheMu.Lock()
	procCache[pid] = cachedProc{
		name:      name,
		expiresAt: time.Now().Add(10 * time.Second),
	}
	procCacheMu.Unlock()

	return name
}

func getTcpPid(localPort uint16) uint32 {
	if pid := getTcpPidAF(localPort, afInet); pid != 0 {
		return pid
	}
	return getTcpPidAF(localPort, afInet6)
}

func getTcpPidAF(localPort uint16, af uintptr) uint32 {
	var size uint32
	for attempt := 0; attempt < 3; attempt++ {
		ret, _, _ := procGetExtendedTcpTable.Call(
			0,
			uintptr(unsafe.Pointer(&size)),
			0,
			af,
			uintptr(tcpTableOwnerPidAll),
			0,
		)
		if ret != 0 && ret != 122 { // 122 = ERROR_INSUFFICIENT_BUFFER
			return 0
		}
		if size == 0 {
			return 0
		}

		buf := make([]byte, size)
		ret, _, _ = procGetExtendedTcpTable.Call(
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&size)),
			0,
			af,
			uintptr(tcpTableOwnerPidAll),
			0,
		)
		if ret == 122 {
			continue
		}
		if ret != 0 {
			return 0
		}

		numEntries := *(*uint32)(unsafe.Pointer(&buf[0]))
		if numEntries == 0 {
			return 0
		}

		srcPtr := uintptr(unsafe.Pointer(&buf[0])) + 4
		if af == afInet {
			rowSize := unsafe.Sizeof(mibTcpRowOwnerPid{})
			for i := uint32(0); i < numEntries; i++ {
				row := (*mibTcpRowOwnerPid)(unsafe.Pointer(srcPtr + uintptr(i)*rowSize))
				if decodePort(row.dwLocalPort) == localPort {
					return row.dwOwningPid
				}
			}
		} else {
			rowSize := unsafe.Sizeof(mibTcp6RowOwnerPid{})
			for i := uint32(0); i < numEntries; i++ {
				row := (*mibTcp6RowOwnerPid)(unsafe.Pointer(srcPtr + uintptr(i)*rowSize))
				if decodePort(row.dwLocalPort) == localPort {
					return row.dwOwningPid
				}
			}
		}
		break
	}
	return 0
}

func getUdpPid(localPort uint16) uint32 {
	if pid := getUdpPidAF(localPort, afInet); pid != 0 {
		return pid
	}
	return getUdpPidAF(localPort, afInet6)
}

func getUdpPidAF(localPort uint16, af uintptr) uint32 {
	var size uint32
	for attempt := 0; attempt < 3; attempt++ {
		ret, _, _ := procGetExtendedUdpTable.Call(
			0,
			uintptr(unsafe.Pointer(&size)),
			0,
			af,
			uintptr(udpTableOwnerPid),
			0,
		)
		if ret != 0 && ret != 122 {
			return 0
		}
		if size == 0 {
			return 0
		}

		buf := make([]byte, size)
		ret, _, _ = procGetExtendedUdpTable.Call(
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&size)),
			0,
			af,
			uintptr(udpTableOwnerPid),
			0,
		)
		if ret == 122 {
			continue
		}
		if ret != 0 {
			return 0
		}

		numEntries := *(*uint32)(unsafe.Pointer(&buf[0]))
		if numEntries == 0 {
			return 0
		}

		srcPtr := uintptr(unsafe.Pointer(&buf[0])) + 4
		if af == afInet {
			rowSize := unsafe.Sizeof(mibUdpRowOwnerPid{})
			for i := uint32(0); i < numEntries; i++ {
				row := (*mibUdpRowOwnerPid)(unsafe.Pointer(srcPtr + uintptr(i)*rowSize))
				if decodePort(row.dwLocalPort) == localPort {
					return row.dwOwningPid
				}
			}
		} else {
			rowSize := unsafe.Sizeof(mibUdp6RowOwnerPid{})
			for i := uint32(0); i < numEntries; i++ {
				row := (*mibUdp6RowOwnerPid)(unsafe.Pointer(srcPtr + uintptr(i)*rowSize))
				if decodePort(row.dwLocalPort) == localPort {
					return row.dwOwningPid
				}
			}
		}
		break
	}
	return 0
}

// GetProcessByPort looks up the owning process name and PID for a local socket port on Windows.
func GetProcessByPort(isTCP bool, port uint16) (string, uint32) {
	var pid uint32
	if isTCP {
		pid = getTcpPid(port)
	} else {
		pid = getUdpPid(port)
	}

	if pid == 0 {
		return "", 0
	}

	name := getProcessNameByPID(pid)
	return name, pid
}
