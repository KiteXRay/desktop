//go:build windows

package bridge

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// GetRunningProcesses returns the names of all currently running processes on Windows.
func GetRunningProcesses() ([]string, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	var procs []string
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil, err
	}

	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if name != "" {
			procs = append(procs, name)
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}

	return procs, nil
}
