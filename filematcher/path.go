package filematcher

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ParsePath normalizes a qBittorrent portable path (qBt-* keys often use
// forward slashes even on Windows) into a native filesystem path.
func ParsePath(path string) string {
	if path == "" {
		return ""
	}
	if runtime.GOOS != "windows" {
		path = strings.ReplaceAll(path, "\\", "/")
	}
	return filepath.Clean(filepath.FromSlash(path))
}

// DefaultBTBackup returns the conventional qBittorrent BT_backup directory
// for the current OS, or "" when unknown.
func DefaultBTBackup() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(home, "AppData", "Local", "qBittorrent", "BT_backup")
	case "linux", "darwin":
		return filepath.Join(home, ".local", "share", "data", "qBittorrent", "BT_backup")
	default:
		return ""
	}
}
