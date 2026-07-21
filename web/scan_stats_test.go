package web

import (
	"testing"
	"time"

	"github.com/styper/qbit-filematcher-go/filematcher"
)

func TestFormatElapsed(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{500 * time.Millisecond, "0s"},
		{3 * time.Second, "3s"},
		{3*time.Minute + 17*time.Second, "3m17s"},
		{1*time.Hour + 2*time.Minute + 3*time.Second, "1h2m3s"},
	}
	for _, tt := range tests {
		if got := formatElapsed(tt.d); got != tt.want {
			t.Errorf("formatElapsed(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestCountNoun(t *testing.T) {
	tests := []struct {
		n          int
		singular   string
		pluralForm string
		want       string
	}{
		{0, "torrent", "torrents", "0 torrents"},
		{1, "torrent", "torrents", "1 torrent"},
		{2, "torrent", "torrents", "2 torrents"},
		{1, "file", "files", "1 file"},
		{1457, "match", "matches", "1457 matches"},
	}
	for _, tt := range tests {
		if got := countNoun(tt.n, tt.singular, tt.pluralForm); got != tt.want {
			t.Errorf("countNoun(%d, %q, %q) = %q, want %q", tt.n, tt.singular, tt.pluralForm, got, tt.want)
		}
	}
}

func TestLibraryScanStats(t *testing.T) {
	lib := &filematcher.Library{
		Torrents: map[string]*filematcher.Torrent{
			"a": {
				Info: filematcher.Info{
					HashV1: "a",
					Files: []filematcher.File{
						{Index: 0, Path: "one.mkv", Size: 1},
						{Index: 1, Path: "two.mkv", Size: 2},
						{Index: 2, Path: ".pad", Size: 3, IsPad: true},
					},
				},
				Matches: map[int]*filematcher.FileMatch{
					0: {Candidates: []string{"/disk/one.mkv"}},
					1: {},
				},
			},
			"b": {
				Info: filematcher.Info{
					HashV1: "b",
					Files:  []filematcher.File{{Index: 0, Path: "solo.mkv", Size: 9}},
				},
				Matches: map[int]*filematcher.FileMatch{
					0: {Candidates: []string{"/disk/solo.mkv", "/other/solo.mkv"}},
				},
			},
		},
	}
	torrents, files, matches := libraryScanStats(lib)
	if torrents != 2 || files != 3 || matches != 2 {
		t.Fatalf("libraryScanStats = %d, %d, %d; want 2, 3, 2", torrents, files, matches)
	}
}
