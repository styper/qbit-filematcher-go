//go:build unix

package filematcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func qBittorrentProcessRunning() (bool, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		if os.IsNotExist(err) {
			// macOS and some BSDs have no /proc; treat as not running.
			return false, nil
		}
		return false, fmt.Errorf("filematcher: list processes: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name[0] < '0' || name[0] > '9' {
			continue
		}
		comm, err := os.ReadFile(filepath.Join("/proc", name, "comm"))
		if err != nil {
			continue
		}
		if isQBittorrentProcessName(string(comm)) {
			return true, nil
		}
		cmdline, err := os.ReadFile(filepath.Join("/proc", name, "cmdline"))
		if err != nil || len(cmdline) == 0 {
			continue
		}
		argv0, _, _ := strings.Cut(string(cmdline), "\x00")
		if isQBittorrentProcessName(filepath.Base(argv0)) {
			return true, nil
		}
	}
	return false, nil
}
