package filematcher

import (
	"path/filepath"
	"testing"
)

func TestDuplicateSelectedIndexes(t *testing.T) {
	shared := filepath.Join(string(filepath.Separator), "media", "cover.png")
	disc2 := filepath.Join(string(filepath.Separator), "media", "disc2", "cover.png")
	other := filepath.Join(string(filepath.Separator), "media", "other.png")
	tor := &Torrent{
		Info: Info{
			Files: []File{
				{Index: 0, Path: "Disc 1/cover.png", Size: 1},
				{Index: 1, Path: "Disc 2/cover.png", Size: 1},
				{Index: 2, Path: "other.png", Size: 2},
			},
		},
		Matches: map[int]*FileMatch{
			0: {Candidates: []string{shared}, SelectedIndex: 0},
			1: {Candidates: []string{shared, disc2}, SelectedIndex: 0},
			2: {Candidates: []string{other}, SelectedIndex: 0},
		},
	}
	got := tor.DuplicateSelectedIndexes()
	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("DuplicateSelectedIndexes = %v, want [0 1]", got)
	}

	tor.Matches[1].SelectedIndex = 1
	if got := tor.DuplicateSelectedIndexes(); len(got) != 0 {
		t.Fatalf("after fixing selection got %v, want none", got)
	}
}
