package filematcher

import "testing"

func TestIsQBittorrentProcessName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"qbittorrent", true},
		{"qBittorrent", true},
		{"qbittorrent\n", true},
		{"qbittorrent-nox", true},
		{"qbittorrent.exe", true},
		{"QBITTORRENT.EXE", true},
		{"qbittorrent_user", false},
		{"sudo", false},
		{"su", false},
		{"", false},
		{"not-qbittorrent", false},
		{"/usr/bin/qbittorrent", false}, // basename checked by callers
	}
	for _, tt := range tests {
		if got := isQBittorrentProcessName(tt.name); got != tt.want {
			t.Errorf("isQBittorrentProcessName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
