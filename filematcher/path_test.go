package filematcher

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultBTBackup(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("user home dir unavailable")
	}

	var rel string
	switch runtime.GOOS {
	case "windows":
		rel = filepath.Join("AppData", "Local", "qBittorrent", "BT_backup")
	case "linux":
		rel = filepath.Join(".local", "share", "qBittorrent", "BT_backup")
	case "darwin":
		rel = filepath.Join(".local", "share", "data", "qBittorrent", "BT_backup")
	default:
		if got := DefaultBTBackup(); got != "" {
			t.Fatalf("DefaultBTBackup()=%q want empty", got)
		}
		return
	}

	want := filepath.Join(home, rel)
	if got := DefaultBTBackup(); got != want {
		t.Fatalf("DefaultBTBackup()=%q want %q", got, want)
	}
}
