package filematcher

import (
	"path/filepath"
	"strings"
)

// MatchStatus describes how completely a torrent's real files are matched.
type MatchStatus int

const (
	// MatchNone means no real files have candidates.
	MatchNone MatchStatus = iota
	// MatchPartial means some, but not all, real files have candidates.
	MatchPartial
	// MatchAll means every real file has at least one candidate.
	MatchAll
)

func (s MatchStatus) String() string {
	switch s {
	case MatchNone:
		return "no_matches"
	case MatchPartial:
		return "partial_match"
	case MatchAll:
		return "all_matches"
	default:
		return "unknown"
	}
}

// File describes one entry from a torrent's file list.
type File struct {
	Index int
	Path  string // relative path inside the torrent (or mapped_files override)
	Size  int64
	IsPad bool
}

// Extension returns the lower-cased file extension without a leading dot,
// or "" if the path has no extension / is a pad file.
func (f File) Extension() string {
	if f.IsPad {
		return ""
	}
	ext := filepath.Ext(f.Path)
	if ext == "" {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(ext, "."))
}

// Info holds immutable metadata loaded from .torrent / .fastresume.
type Info struct {
	HashV1       string
	Name         string
	Files        []File
	Tags         []string
	AddedTime    int64  // unix seconds; 0 if unknown
	SavePath     string // from fastresume
	DownloadPath string // from fastresume; may be empty
}

// RealFiles returns non-pad files.
func (i Info) RealFiles() []File {
	out := make([]File, 0, len(i.Files))
	for _, f := range i.Files {
		if !f.IsPad {
			out = append(out, f)
		}
	}
	return out
}

// Paths locates the on-disk .torrent and .fastresume pair for a torrent.
type Paths struct {
	TorrentFile    string
	FastresumeFile string
}

// FileMatch holds candidate paths found on disk for one torrent file, and
// which candidate is currently selected.
type FileMatch struct {
	Candidates    []string
	SelectedIndex int // defaults to 0; clamped when reading Selected
	// AutoIndex is the index chosen by SelectBestMatches (or 0 before that).
	// Used by UIs to highlight manual overrides.
	AutoIndex int
}

// HasMatch reports whether at least one candidate exists.
func (m *FileMatch) HasMatch() bool {
	return m != nil && len(m.Candidates) > 0
}

// Selected returns the selected candidate path, or "" if none.
func (m *FileMatch) Selected() string {
	if !m.HasMatch() {
		return ""
	}
	if m.SelectedIndex < 0 || m.SelectedIndex >= len(m.Candidates) {
		return m.Candidates[0]
	}
	return m.Candidates[m.SelectedIndex]
}

// SavePlan is the computed fastresume update for a torrent.
type SavePlan struct {
	SavePath     string
	MappedFiles  []string // relative to SavePath; one entry per torrent file (incl. pads)
	IsIncomplete bool
}
