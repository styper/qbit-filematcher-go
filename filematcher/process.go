package filematcher

import "strings"

// IsQBittorrentRunning reports whether a qBittorrent process appears to be
// running on this host.
func IsQBittorrentRunning() (bool, error) {
	return qBittorrentProcessRunning()
}

// isQBittorrentProcessName reports whether name is a qBittorrent binary
// basename (comm / argv0 / Windows ExeFile), not an incidental substring
// like a username in a shell cmdline.
func isQBittorrentProcessName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimSuffix(name, ".exe")
	switch name {
	case "qbittorrent", "qbittorrent-nox":
		return true
	default:
		return false
	}
}
