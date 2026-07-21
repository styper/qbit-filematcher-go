//go:build windows

package filematcher

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

func qBittorrentProcessRunning() (bool, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, fmt.Errorf("filematcher: list processes: %w", err)
	}
	defer windows.CloseHandle(snap)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snap, &entry); err != nil {
		return false, fmt.Errorf("filematcher: list processes: %w", err)
	}

	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if isQBittorrentProcessName(name) {
			return true, nil
		}
		if err := windows.Process32Next(snap, &entry); err != nil {
			break
		}
	}
	return false, nil
}
