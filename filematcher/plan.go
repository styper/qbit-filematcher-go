package filematcher

import (
	"fmt"
	"path/filepath"
	"sort"
)

// PlanOptions controls save-plan generation.
type PlanOptions struct {
	// AllowIncomplete permits plans when only some real files are matched.
	// Incomplete saves typically clear piece bitfields so qBittorrent rechecks.
	AllowIncomplete bool
}

// MakePlan computes the NoSubfolder-style save_path and mapped_files for the
// currently selected matches.
//
// save_path starts as the longest common ancestor of the *parent directories* of the
// selected matches, then moves one directory up (so a single file at
// /data/foo/bar.mkv yields save_path=/data and mapped=[foo/bar.mkv]).
// It does not move above the filesystem root.
func (t *Torrent) MakePlan(opts PlanOptions) (*SavePlan, error) {
	status := t.Status()
	switch status {
	case MatchNone:
		return nil, fmt.Errorf("%w: no files in the torrent have a match", ErrNoMatches)
	case MatchPartial:
		if !opts.AllowIncomplete {
			return nil, fmt.Errorf("%w: not all files have a match (pass AllowIncomplete to bypass)", ErrIncomplete)
		}
	}

	parents := make([]string, 0, len(t.Info.RealFiles()))
	for _, f := range t.Info.RealFiles() {
		m := t.Matches[f.Index]
		if m == nil || !m.HasMatch() {
			continue
		}
		parents = append(parents, filepath.Dir(m.Selected()))
	}
	savePath, err := commonPath(parents)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot determine save path: %w", ErrNoMatches, err)
	}
	savePath = oneLevelUp(savePath)

	mapped := make([]string, 0, len(t.Info.Files))
	incomplete := false
	for _, f := range t.Info.Files {
		if f.IsPad {
			mapped = append(mapped, "")
			continue
		}
		m := t.Matches[f.Index]
		if m != nil && m.HasMatch() {
			rel, err := filepath.Rel(savePath, m.Selected())
			if err != nil {
				return nil, fmt.Errorf("%w: %w", ErrInvalidPlan, err)
			}
			mapped = append(mapped, filepath.ToSlash(rel))
		} else {
			mapped = append(mapped, filepath.ToSlash(f.Path))
			incomplete = true
		}
	}

	seen := map[string]int{}
	for _, entry := range mapped {
		if entry == "" {
			continue
		}
		seen[entry]++
	}
	var dups []string
	for p, n := range seen {
		if n > 1 {
			dups = append(dups, p)
		}
	}
	if len(dups) > 0 {
		return nil, fmt.Errorf("%w: multiple files map to the same disk path(s): %v", ErrDuplicateMatch, dups)
	}

	return &SavePlan{
		SavePath:     savePath,
		MappedFiles:  mapped,
		IsIncomplete: incomplete,
	}, nil
}

// DuplicateSelectedIndexes returns real-file indexes that share the same
// selected on-disk path with at least one other file (plan would fail with
// ErrDuplicateMatch). Indexes are sorted ascending.
func (t *Torrent) DuplicateSelectedIndexes() []int {
	byPath := map[string][]int{}
	for _, f := range t.Info.RealFiles() {
		m := t.Matches[f.Index]
		if m == nil || !m.HasMatch() {
			continue
		}
		p := filepath.Clean(m.Selected())
		byPath[p] = append(byPath[p], f.Index)
	}
	var out []int
	for _, idxs := range byPath {
		if len(idxs) < 2 {
			continue
		}
		out = append(out, idxs...)
	}
	sort.Ints(out)
	return out
}

// oneLevelUp returns the parent of path, or path itself when it is already
// the filesystem root (e.g. "/" or "C:\\").
func oneLevelUp(path string) string {
	clean := filepath.Clean(path)
	parent := filepath.Dir(clean)
	if parent == clean {
		return clean
	}
	return parent
}

// ValidatePlan checks that plan is structurally consistent with t.
func (t *Torrent) ValidatePlan(plan *SavePlan) error {
	if plan == nil {
		return ErrInvalidPlan
	}
	if len(plan.MappedFiles) != len(t.Info.Files) {
		return fmt.Errorf("%w: plan has %d mapped_files, torrent has %d files",
			ErrInvalidPlan, len(plan.MappedFiles), len(t.Info.Files))
	}
	if !filepath.IsAbs(plan.SavePath) {
		return fmt.Errorf("%w: plan.save_path must be absolute: %s", ErrInvalidPlan, plan.SavePath)
	}
	for i, mapped := range plan.MappedFiles {
		f := t.Info.Files[i]
		if f.IsPad {
			if mapped != "" {
				return fmt.Errorf("%w: mapped_files[%d]: pad file must map to '', got %q", ErrInvalidPlan, i, mapped)
			}
			continue
		}
		if mapped == "" {
			return fmt.Errorf("%w: mapped_files[%d]: real file cannot map to ''", ErrInvalidPlan, i)
		}
		if isObviouslyAbsolute(mapped) {
			return fmt.Errorf("%w: mapped_files[%d] must be relative, got absolute: %q", ErrInvalidPlan, i, mapped)
		}
	}
	return nil
}

func isObviouslyAbsolute(pathStr string) bool {
	if pathStr == "" {
		return false
	}
	return filepath.IsAbs(pathStr) || filepath.IsAbs(filepath.FromSlash(pathStr))
}

// commonPath returns the longest common ancestor of paths.
func commonPath(paths []string) (string, error) {
	if len(paths) == 0 {
		return "", fmt.Errorf("no paths")
	}
	parts := make([][]string, len(paths))
	for i, p := range paths {
		clean := filepath.Clean(p)
		if clean == "." || clean == "" {
			return "", fmt.Errorf("empty path")
		}
		parts[i] = splitPath(clean)
	}
	common := []string{}
	for i := 0; ; i++ {
		if i >= len(parts[0]) {
			break
		}
		chunk := parts[0][i]
		ok := true
		for _, p := range parts[1:] {
			if i >= len(p) || p[i] != chunk {
				ok = false
				break
			}
		}
		if !ok {
			break
		}
		common = append(common, chunk)
	}
	if len(common) == 0 {
		return "", fmt.Errorf("no common path between inputs")
	}
	return joinPath(common), nil
}

func splitPath(p string) []string {
	p = filepath.Clean(p)
	vol := filepath.VolumeName(p)
	rest := p
	if vol != "" {
		rest = p[len(vol):]
	}
	rest = stringsTrimSep(rest)
	var out []string
	if vol != "" {
		out = append(out, vol)
	} else if filepath.IsAbs(p) {
		out = append(out, string(filepath.Separator))
	}
	if rest == "" {
		return out
	}
	out = append(out, splitNonEmpty(rest, filepath.Separator)...)
	return out
}

func joinPath(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if parts[0] == string(filepath.Separator) {
		return string(filepath.Separator) + filepath.Join(parts[1:]...)
	}
	// Windows volume like "C:"
	if len(parts[0]) == 2 && parts[0][1] == ':' {
		if len(parts) == 1 {
			return parts[0] + string(filepath.Separator)
		}
		return parts[0] + string(filepath.Separator) + filepath.Join(parts[1:]...)
	}
	return filepath.Join(parts...)
}

func stringsTrimSep(s string) string {
	for len(s) > 0 && (s[0] == '/' || s[0] == '\\') {
		s = s[1:]
	}
	return s
}

func splitNonEmpty(s string, sep byte) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep || (sep == '/' && s[i] == '\\') || (sep == '\\' && s[i] == '/') {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
