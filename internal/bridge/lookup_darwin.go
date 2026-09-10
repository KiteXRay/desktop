//go:build darwin

package bridge

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type darwinCachedProc struct {
	name      string
	pid       uint32
	expiresAt time.Time
}

var (
	darwinProcCacheMu sync.RWMutex
	darwinProcCache   = make(map[uint16]darwinCachedProc)
	darwinPidCacheMu  sync.RWMutex
	darwinPidCache    = make(map[uint32]string)
)

func GetProcessByPort(isTCP bool, port uint16) (string, uint32) {
	if port == 0 {
		return "", 0
	}

	darwinProcCacheMu.RLock()
	if item, found := darwinProcCache[port]; found && time.Now().Before(item.expiresAt) {
		darwinProcCacheMu.RUnlock()
		return item.name, item.pid
	}
	darwinProcCacheMu.RUnlock()

	// Use lsof -n -P -i :<port> -F pc
	// Machine-readable format: p<PID>, c<command>
	cmd := exec.Command("lsof", "-n", "-P", fmt.Sprintf("-i:%d", port), "-F", "pc")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		// Negative cache for 1s to avoid high subprocess churn during port retries
		darwinProcCacheMu.Lock()
		darwinProcCache[port] = darwinCachedProc{
			expiresAt: time.Now().Add(1 * time.Second),
		}
		darwinProcCacheMu.Unlock()
		return "", 0
	}

	var pid uint32
	var comm string

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case 'p':
			if p, err := strconv.ParseUint(line[1:], 10, 32); err == nil {
				pid = uint32(p)
			}
		case 'c':
			if comm == "" {
				comm = strings.TrimSpace(line[1:])
			}
		}
	}

	if pid == 0 && comm == "" {
		darwinProcCacheMu.Lock()
		darwinProcCache[port] = darwinCachedProc{
			expiresAt: time.Now().Add(1 * time.Second),
		}
		darwinProcCacheMu.Unlock()
		return "", 0
	}

	procName := comm
	if pid > 0 {
		procName = getProcessNameByDarwinPID(pid, comm)
	}

	darwinProcCacheMu.Lock()
	darwinProcCache[port] = darwinCachedProc{
		name:      procName,
		pid:       pid,
		expiresAt: time.Now().Add(5 * time.Second),
	}
	darwinProcCacheMu.Unlock()

	return procName, pid
}

func getProcessNameByDarwinPID(pid uint32, fallback string) string {
	darwinPidCacheMu.RLock()
	if name, found := darwinPidCache[pid]; found && name != "" {
		darwinPidCacheMu.RUnlock()
		return name
	}
	darwinPidCacheMu.RUnlock()

	out, _ := exec.Command("ps", "-p", strconv.FormatUint(uint64(pid), 10), "-o", "comm=").Output()
	name := strings.TrimSpace(string(out))
	if name == "" {
		name = fallback
	}

	if name != "" {
		darwinPidCacheMu.Lock()
		darwinPidCache[pid] = name
		darwinPidCacheMu.Unlock()
	}

	return name
}
