package filematcher

import "path/filepath"

// LocationStatus describes whether selected on-disk paths already match the
// torrent's current fastresume layout (save_path + file / mapped_files paths).
type LocationStatus int

const (
	// LocationNone means no real file has a candidate (match status no_matches).
	LocationNone LocationStatus = iota
	// LocationIncomplete means some, but not all, real files have a selected candidate.
	LocationIncomplete
	// LocationCurrent means every real file's selection matches the current layout.
	LocationCurrent
	// LocationChanged means every real file has a selection, but at least one
	// differs from the current fastresume path.
	LocationChanged
)

func (s LocationStatus) String() string {
	switch s {
	case LocationNone:
		return "none"
	case LocationIncomplete:
		return "incomplete"
	case LocationCurrent:
		return "current"
	case LocationChanged:
		return "changed"
	default:
		return "unknown"
	}
}

// Label is the short UI name for Match Status (None / Partial / Current / Changed).
func (s LocationStatus) Label() string {
	switch s {
	case LocationNone:
		return "None"
	case LocationIncomplete:
		return "Partial"
	case LocationCurrent:
		return "Current"
	case LocationChanged:
		return "Changed"
	default:
		return "Unknown"
	}
}

// LocationStatus compares selected candidates to the paths implied by the
// loaded fastresume (SavePath + each file's Path, which already includes
// mapped_files overrides from parse).
//
// This does not call MakePlan; it is cheap enough for list views.
func (t *Torrent) LocationStatus() LocationStatus {
	realFiles := t.Info.RealFiles()
	if len(realFiles) == 0 {
		return LocationNone
	}

	matched := 0
	anyChanged := false
	for _, f := range realFiles {
		m := t.Matches[f.Index]
		if m == nil || !m.HasMatch() {
			continue
		}
		matched++
		current := currentAbsolutePath(t.Info.SavePath, f.Path)
		selected := filepath.Clean(m.Selected())
		if current == "" || selected != current {
			anyChanged = true
		}
	}
	switch {
	case matched == 0:
		return LocationNone
	case matched < len(realFiles):
		return LocationIncomplete
	case anyChanged:
		return LocationChanged
	default:
		return LocationCurrent
	}
}

func currentAbsolutePath(savePath, rel string) string {
	if savePath == "" || rel == "" {
		return ""
	}
	return filepath.Clean(filepath.Join(savePath, filepath.FromSlash(rel)))
}
