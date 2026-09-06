//go:build linux

package bridge

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type cachedProc struct {
	name      string
	pid       uint32
	expiresAt time.Time
}

var (
	linuxProcCacheMu sync.RWMutex
	linuxProcCache   = make(map[uint64]cachedProc) // inode -> proc
)

// getInodeByPort reads /proc/net/tcp, /proc/net/tcp6, /proc/net/udp, or /proc/net/udp6 to find the inode for a port.
func getInodeByPort(isTCP bool, port uint16) uint64 {
	var files []string
	if isTCP {
		files = []string{"/proc/net/tcp", "/proc/net/tcp6"}
	} else {
		files = []string{"/proc/net/udp", "/proc/net/udp6"}
	}

	portHex := fmt.Sprintf("%04X", port)

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		isHeader := true
		for scanner.Scan() {
			if isHeader {
				isHeader = false
				continue
			}
			line := strings.TrimSpace(scanner.Text())
			fields := strings.Fields(line)
			if len(fields) < 10 {
				continue
			}

			localAddr := fields[1]
			parts := strings.Split(localAddr, ":")
			if len(parts) == 2 && strings.EqualFold(parts[1], portHex) {
				inodeStr := fields[9]
				inode, err := strconv.ParseUint(inodeStr, 10, 64)
				_ = f.Close()
				if err == nil && inode != 0 {
					return inode
				}
				return 0
			}
		}
		_ = f.Close()
	}

	return 0
}

// findProcessByInode scans /proc/[pid]/fd to locate which process owns the socket inode.
func findProcessByInode(inode uint64) (string, uint32) {
	if inode == 0 {
		return "", 0
	}

	linuxProcCacheMu.RLock()
	if item, found := linuxProcCache[inode]; found && time.Now().Before(item.expiresAt) {
		linuxProcCacheMu.RUnlock()
		return item.name, item.pid
	}
	linuxProcCacheMu.RUnlock()

	targetSocket := fmt.Sprintf("socket:[%d]", inode)

	procDir, err := os.Open("/proc")
	if err != nil {
		return "", 0
	}
	defer procDir.Close()

	names, err := procDir.Readdirnames(-1)
	if err != nil {
		return "", 0
	}

	for _, name := range names {
		pid, err := strconv.ParseUint(name, 10, 32)
		if err != nil {
			continue
		}

		fdDirPath := filepath.Join("/proc", name, "fd")
		fds, err := os.ReadDir(fdDirPath)
		if err != nil {
			continue
		}

		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDirPath, fd.Name()))
			if err != nil {
				continue
			}
			if link == targetSocket {
				// Found process
				commBytes, _ := os.ReadFile(filepath.Join("/proc", name, "comm"))
				comm := strings.TrimSpace(string(commBytes))
				if comm == "" {
					exeLink, _ := os.Readlink(filepath.Join("/proc", name, "exe"))
					comm = filepath.Base(exeLink)
				}

				linuxProcCacheMu.Lock()
				linuxProcCache[inode] = cachedProc{
					name:      comm,
					pid:       uint32(pid),
					expiresAt: time.Now().Add(10 * time.Second),
				}
				linuxProcCacheMu.Unlock()

				return comm, uint32(pid)
			}
		}
	}

	return "", 0
}

// GetProcessByPort looks up the owning process name and PID for a local socket port on Linux.
func GetProcessByPort(isTCP bool, port uint16) (string, uint32) {
	inode := getInodeByPort(isTCP, port)
	if inode == 0 {
		return "", 0
	}
	return findProcessByInode(inode)
}
