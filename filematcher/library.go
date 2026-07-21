package filematcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Torrent is a loaded torrent plus its current match state.
type Torrent struct {
	Info    Info
	Paths   Paths
	Matches map[int]*FileMatch // keyed by File.Index for real (non-pad) files

	torrentDict    map[string]any
	fastresumeDict map[string]any
}

// Status returns the overall match completeness for this torrent.
func (t *Torrent) Status() MatchStatus {
	real := t.Info.RealFiles()
	if len(real) == 0 {
		return MatchNone
	}
	matched := 0
	for _, f := range real {
		if m := t.Matches[f.Index]; m != nil && m.HasMatch() {
			matched++
		}
	}
	switch {
	case matched == 0:
		return MatchNone
	case matched == len(real):
		return MatchAll
	default:
		return MatchPartial
	}
}

// Library is a collection of torrents keyed by infohash v1.
type Library struct {
	Torrents   map[string]*Torrent
	LoadErrors map[string]error
}

// LoadOptions controls how BT_backup is loaded.
type LoadOptions struct {
	// Strict fails the whole load on the first unreadable pair. When false,
	// bad pairs are recorded in Library.LoadErrors and skipped.
	Strict bool
}

// LoadLibrary reads paired .torrent / .fastresume files from a BT_backup directory.
func LoadLibrary(btBackup string, opts LoadOptions) (*Library, error) {
	entries, err := os.ReadDir(btBackup)
	if err != nil {
		return nil, fmt.Errorf("filematcher: read BT_backup: %w", err)
	}

	lib := &Library{
		Torrents:   make(map[string]*Torrent),
		LoadErrors: make(map[string]error),
	}

	type pair struct {
		torrent    string
		fastresume string
	}
	pairs := map[string]*pair{}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		switch {
		case strings.HasSuffix(name, ".torrent"):
			hash := strings.TrimSuffix(name, ".torrent")
			if hash == "" {
				continue
			}
			p := pairs[hash]
			if p == nil {
				p = &pair{}
				pairs[hash] = p
			}
			p.torrent = filepath.Join(btBackup, name)
		case strings.HasSuffix(name, ".fastresume"):
			hash := strings.TrimSuffix(name, ".fastresume")
			if hash == "" {
				continue
			}
			p := pairs[hash]
			if p == nil {
				p = &pair{}
				pairs[hash] = p
			}
			p.fastresume = filepath.Join(btBackup, name)
		}
	}

	for hash, p := range pairs {
		if p.torrent == "" || p.fastresume == "" {
			continue
		}
		t, err := LoadTorrent(Paths{TorrentFile: p.torrent, FastresumeFile: p.fastresume}, hash)
		if err != nil {
			if opts.Strict {
				return nil, err
			}
			lib.LoadErrors[hash] = err
			continue
		}
		lib.Torrents[t.Info.HashV1] = t
	}

	return lib, nil
}

// FilterOptions selects which torrents remain in a filtered library.
type FilterOptions struct {
	Hashes       []string // exact infohash v1 matches
	Names        []string // all tokens must appear in the torrent name
	Tags         []string
	MatchAllTags bool   // when true every tag must be present; otherwise any
	SavePathRoot string // when set, keep torrents whose content lives under this root
}

// Filter returns a new library containing only torrents that match opts.
// The original library is not modified.
func (l *Library) Filter(opts FilterOptions) *Library {
	out := &Library{
		Torrents:   make(map[string]*Torrent),
		LoadErrors: make(map[string]error, len(l.LoadErrors)),
	}
	for k, v := range l.LoadErrors {
		out.LoadErrors[k] = v
	}
	for hash, t := range l.Torrents {
		if matchFilter(t, opts) {
			out.Torrents[hash] = t
		}
	}
	return out
}

// List returns torrents in arbitrary map order as a slice.
func (l *Library) List() []*Torrent {
	out := make([]*Torrent, 0, len(l.Torrents))
	for _, t := range l.Torrents {
		out = append(out, t)
	}
	return out
}

func matchFilter(t *Torrent, opts FilterOptions) bool {
	if len(opts.Hashes) > 0 {
		ok := false
		want := make(map[string]struct{}, len(opts.Hashes))
		for _, h := range opts.Hashes {
			want[strings.ToLower(h)] = struct{}{}
		}
		if _, ok = want[strings.ToLower(t.Info.HashV1)]; !ok {
			return false
		}
	}

	if len(opts.Tags) > 0 {
		tagSet := make(map[string]struct{}, len(t.Info.Tags))
		for _, tag := range t.Info.Tags {
			tagSet[tag] = struct{}{}
		}
		if opts.MatchAllTags {
			for _, want := range opts.Tags {
				if _, ok := tagSet[want]; !ok {
					return false
				}
			}
		} else {
			any := false
			for _, want := range opts.Tags {
				if _, ok := tagSet[want]; ok {
					any = true
					break
				}
			}
			if !any {
				return false
			}
		}
	}

	if len(opts.Names) > 0 {
		normalized := normalizeName(t.Info.Name)
		for _, token := range opts.Names {
			if tok := normalizeName(token); tok == "" || !strings.Contains(normalized, tok) {
				return false
			}
		}
	}

	if opts.SavePathRoot != "" {
		root := filepath.Clean(opts.SavePathRoot)
		if t.Info.SavePath == "" {
			return false
		}
		sp := filepath.Clean(t.Info.SavePath)
		if sp == root || strings.HasPrefix(sp, root+string(os.PathSeparator)) {
			return true
		}
		for _, f := range t.Info.RealFiles() {
			full := filepath.Clean(filepath.Join(sp, filepath.FromSlash(f.Path)))
			if full == root || strings.HasPrefix(full, root+string(os.PathSeparator)) {
				return true
			}
		}
		return false
	}

	return true
}

func normalizeName(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
