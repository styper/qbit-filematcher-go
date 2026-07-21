package filematcher

import (
	"path/filepath"
	"testing"
)

func TestBestMatchIndexPrefersSavePath(t *testing.T) {
	save := filepath.Join(string(filepath.Separator), "mnt", "media", "Torrents", "KeepForAWhille",
		"Ayreon - The Theater Equation (2016) Blu-ray 1080i AVC DD 5.1")
	rel := filepath.Join("CERTIFICATE", "BACKUP", "id.bdmv")
	correct := filepath.Join(save, rel)
	wrong := filepath.Join(string(filepath.Separator), "mnt", "media", "Torrents", "Downloads",
		"AYREON UNIVERSE", "CERTIFICATE", "BACKUP", "id.bdmv")

	// Shorter wrong path first — old heuristic favored this.
	candidates := []string{wrong, correct}
	got := bestMatchIndex(save, "Ayreon - The Theater Equation (2016) Blu-ray 1080i AVC DD 5.1",
		"CERTIFICATE/BACKUP/id.bdmv", candidates)
	if got != 1 {
		t.Fatalf("selected %d (%s), want correct under save_path", got, candidates[got])
	}

	// Exact match still wins even if listed later.
	got = bestMatchIndex(save, "", "CERTIFICATE/BACKUP/id.bdmv", []string{wrong, correct})
	if got != 1 {
		t.Fatalf("exact under save_path: got %d", got)
	}
}

func TestBestMatchIndexFallsBackToTorrentName(t *testing.T) {
	name := "Ayreon - The Theater Equation (2016) Blu-ray 1080i AVC DD 5.1"
	correct := filepath.Join(string(filepath.Separator), "mnt", "media", "Torrents", name,
		"CERTIFICATE", "BACKUP", "id.bdmv")
	wrong := filepath.Join(string(filepath.Separator), "mnt", "media", "Torrents", "Downloads",
		"AYREON UNIVERSE", "CERTIFICATE", "BACKUP", "id.bdmv")

	got := bestMatchIndex("", name, "CERTIFICATE/BACKUP/id.bdmv", []string{wrong, correct})
	if got != 1 {
		t.Fatalf("selected %d (%s), want name-context match", got, candidatesAt(got, wrong, correct))
	}
}

func TestBestMatchIndexUnderSavePathOnly(t *testing.T) {
	save := filepath.Join(string(filepath.Separator), "data", "old")
	under := filepath.Join(save, "other", "file.mkv")
	elsewhere := filepath.Join(string(filepath.Separator), "data", "elsewhere", "file.mkv")
	got := bestMatchIndex(save, "Demo", "file.mkv", []string{elsewhere, under})
	if got != 1 {
		t.Fatalf("selected %d, want candidate under save_path", got)
	}
}

func candidatesAt(i int, a, b string) string {
	if i == 0 {
		return a
	}
	return b
}
