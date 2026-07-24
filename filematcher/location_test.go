package filematcher

import (
	"path/filepath"
	"testing"
)

func TestLocationStatus(t *testing.T) {
	root := t.TempDir()
	save := filepath.Join(root, "downloads")
	cur := filepath.Join(save, "Show", "ep.mkv")
	other := filepath.Join(root, "other", "ep.mkv")

	tor := &Torrent{
		Info: Info{
			SavePath: save,
			Files: []File{
				{Index: 0, Path: "Show/ep.mkv", Size: 10},
			},
		},
		Matches: map[int]*FileMatch{},
	}

	if got := tor.LocationStatus(); got != LocationNone {
		t.Fatalf("no candidates: got %v, want none", got)
	}

	tor.Matches[0] = &FileMatch{Candidates: []string{cur}, SelectedIndex: 0}
	if got := tor.LocationStatus(); got != LocationCurrent {
		t.Fatalf("same path: got %v, want current", got)
	}

	tor.Matches[0] = &FileMatch{Candidates: []string{other, cur}, SelectedIndex: 0}
	if got := tor.LocationStatus(); got != LocationChanged {
		t.Fatalf("other selected: got %v, want changed", got)
	}

	tor.Matches[0].SelectedIndex = 1
	if got := tor.LocationStatus(); got != LocationCurrent {
		t.Fatalf("current reselected: got %v, want current", got)
	}
}

func TestLocationStatusPartial(t *testing.T) {
	root := t.TempDir()
	save := filepath.Join(root, "dl")
	a := filepath.Join(save, "a.mkv")
	b := filepath.Join(save, "b.mkv")

	tor := &Torrent{
		Info: Info{
			SavePath: save,
			Files: []File{
				{Index: 0, Path: "a.mkv", Size: 1},
				{Index: 1, Path: "b.mkv", Size: 1},
			},
		},
		Matches: map[int]*FileMatch{
			0: {Candidates: []string{a}, SelectedIndex: 0},
			// file 1 unmatched
		},
	}
	if got := tor.LocationStatus(); got != LocationIncomplete {
		t.Fatalf("partial: got %v, want incomplete", got)
	}

	tor.Matches[1] = &FileMatch{Candidates: []string{b}, SelectedIndex: 0}
	if got := tor.LocationStatus(); got != LocationCurrent {
		t.Fatalf("both current: got %v, want current", got)
	}

	tor.Matches[1] = &FileMatch{
		Candidates:    []string{filepath.Join(root, "elsewhere", "b.mkv")},
		SelectedIndex: 0,
	}
	if got := tor.LocationStatus(); got != LocationChanged {
		t.Fatalf("one moved: got %v, want changed", got)
	}
}

func TestLocationStatusString(t *testing.T) {
	if LocationNone.String() != "none" {
		t.Fatal(LocationNone.String())
	}
	if LocationCurrent.String() != "current" {
		t.Fatal(LocationCurrent.String())
	}
	if LocationChanged.String() != "changed" {
		t.Fatal(LocationChanged.String())
	}
	if LocationIncomplete.String() != "incomplete" {
		t.Fatal(LocationIncomplete.String())
	}
	if LocationNone.Label() != "None" || LocationIncomplete.Label() != "Partial" {
		t.Fatalf("labels: %q %q", LocationNone.Label(), LocationIncomplete.Label())
	}
	if LocationCurrent.Label() != "Current" || LocationChanged.Label() != "Changed" {
		t.Fatalf("labels: %q %q", LocationCurrent.Label(), LocationChanged.Label())
	}
}
