package filematcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// ScanOptions controls disk scanning for match candidates.
type ScanOptions struct {
	SearchPaths []string
	ExcludeDirs []string // directory base names to skip (not full paths)
	// SelectBest, when true, automatically picks the best candidate per file
	// after scanning (see [Torrent.SelectBestMatches]).
	SelectBest bool
}

type lookupKey struct {
	ext  string
	size int64
}

// Scan walks SearchPaths and attaches candidate paths to each torrent's
// real files when extension and size match.
//
// Torrents are updated in place. Pad files are ignored.
// Returns the number of (torrent-file, candidate) pairs added.
//
// ctx may cancel a long walk; when cancelled, Scan returns the pairs added
// so far and ctx.Err().
//
//nolint:gocyclo // hot path: keep matching branches inline for scan speed
func Scan(ctx context.Context, torrents []*Torrent, opts ScanOptions) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	exclude := make(map[string]struct{}, len(opts.ExcludeDirs))
	for _, d := range opts.ExcludeDirs {
		if d != "" {
			exclude[d] = struct{}{}
		}
	}

	lookup := map[lookupKey][]struct {
		t     *Torrent
		index int
	}{}
	interesting := map[string]struct{}{}

	for _, t := range torrents {
		if t == nil {
			continue
		}
		for _, f := range t.Info.RealFiles() {
			ext := f.Extension()
			if ext == "" {
				continue
			}
			interesting[ext] = struct{}{}
			k := lookupKey{ext: ext, size: f.Size}
			lookup[k] = append(lookup[k], struct {
				t     *Torrent
				index int
			}{t: t, index: f.Index})
		}
	}

	if len(interesting) == 0 {
		return 0, nil
	}

	added := 0
	stack := make([]string, 0, len(opts.SearchPaths))
	for _, p := range opts.SearchPaths {
		if p != "" {
			stack = append(stack, p)
		}
	}

	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return added, err
		}

		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		entries, err := os.ReadDir(current)
		if err != nil {
			continue
		}
		for i, entry := range entries {
			if i%64 == 0 {
				if err := ctx.Err(); err != nil {
					return added, err
				}
			}

			name := entry.Name()
			full := filepath.Join(current, name)

			// Do not follow symlinks (matches qBittorrent tooling expectations).
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}

			if entry.IsDir() {
				if _, skip := exclude[name]; !skip {
					stack = append(stack, full)
				}
				continue
			}
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() {
				continue
			}

			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
			if ext == "" {
				continue
			}
			if _, ok := interesting[ext]; !ok {
				continue
			}

			k := lookupKey{ext: ext, size: info.Size()}
			matches := lookup[k]
			if len(matches) == 0 {
				continue
			}
			for _, m := range matches {
				fm := m.t.Matches[m.index]
				if fm == nil {
					fm = &FileMatch{SelectedIndex: 0}
					m.t.Matches[m.index] = fm
				}
				fm.Candidates = append(fm.Candidates, full)
				added++
			}
		}
	}

	if err := ctx.Err(); err != nil {
		return added, err
	}

	if opts.SelectBest {
		seen := map[*Torrent]struct{}{}
		for _, t := range torrents {
			if t == nil {
				continue
			}
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			t.SelectBestMatches()
		}
	}

	return added, nil
}

// SelectBestMatches chooses one candidate per file using path context from
// the current save_path (when set) and a path-similarity heuristic.
//
// Preference order:
//  1. Exact filepath.Join(save_path, relative path) when present
//  2. Candidates still under the current save_path (fuzzy among those)
//  3. Fuzzy against Join(save_path, path), or Join(torrent name, path) if
//     save_path is empty, else the relative path alone
func (t *Torrent) SelectBestMatches() {
	for _, f := range t.Info.RealFiles() {
		m := t.Matches[f.Index]
		if m == nil || len(m.Candidates) == 0 {
			continue
		}
		if len(m.Candidates) > 1 {
			m.SelectedIndex = bestMatchIndex(t.Info.SavePath, t.Info.Name, f.Path, m.Candidates)
		}
		m.AutoIndex = m.SelectedIndex
	}
}

func bestMatchIndex(savePath, torrentName, filePath string, candidates []string) int {
	rel := filepath.FromSlash(filePath)
	if savePath != "" {
		expected := filepath.Clean(filepath.Join(savePath, rel))
		for i, c := range candidates {
			if filepath.Clean(c) == expected {
				return i
			}
		}

		under := make([]int, 0, len(candidates))
		root := filepath.Clean(savePath)
		for i, c := range candidates {
			if pathUnderRoot(c, root) {
				under = append(under, i)
			}
		}
		if len(under) == 1 {
			return under[0]
		}
		if len(under) > 1 {
			subset := make([]string, len(under))
			for j, i := range under {
				subset[j] = candidates[i]
			}
			return under[bestFuzzyIndex(expected, subset)]
		}
	}

	return bestFuzzyIndex(fuzzyTarget(savePath, torrentName, rel), candidates)
}

func fuzzyTarget(savePath, torrentName, relPath string) string {
	if savePath != "" {
		return filepath.Join(savePath, relPath)
	}
	if torrentName != "" {
		return filepath.Join(torrentName, relPath)
	}
	return relPath
}

func bestFuzzyIndex(target string, candidates []string) int {
	bestIdx := 0
	bestScore := -1
	normTarget := normalizeForFuzzy(target)
	for i, c := range candidates {
		score := stringRatio(normTarget, normalizeForFuzzy(c))
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return bestIdx
}

// pathUnderRoot reports whether path is root or a descendant of root.
func pathUnderRoot(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func normalizeForFuzzy(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// stringRatio returns a 0..100 similarity score (simple ratio).
func stringRatio(a, b string) int {
	if a == b {
		return 100
	}
	if a == "" || b == "" {
		return 0
	}
	dist := levenshtein(a, b)
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	return (100 * (maxLen - dist)) / maxLen
}

func levenshtein(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	if len(rb) == 0 {
		return len(ra)
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := cur[j-1] + 1
			sub := prev[j-1] + cost
			cur[j] = min(del, ins, sub)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
