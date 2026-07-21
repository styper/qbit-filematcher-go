package filematcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadScanPlanSave(t *testing.T) {
	root := t.TempDir()
	bt := filepath.Join(root, "BT_backup")
	search := filepath.Join(root, "media")
	if err := os.MkdirAll(bt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(search, "shows", "Demo"), 0o755); err != nil {
		t.Fatal(err)
	}

	content := []byte("hello torrent file content!!")
	diskFile := filepath.Join(search, "shows", "Demo", "episode.mkv")
	if err := os.WriteFile(diskFile, content, 0o644); err != nil {
		t.Fatal(err)
	}

	hash := "aabbccddeeff00112233445566778899aabbccdd"
	info := map[string]any{
		"name":         "episode.mkv",
		"length":       int64(len(content)),
		"piece length": int64(16384),
		"pieces":       []byte("01234567890123456789"),
	}
	torrent := map[string]any{"info": info}
	fastresume := map[string]any{
		"save_path":    filepath.Join(root, "old"),
		"qBt-savePath": filepath.ToSlash(filepath.Join(root, "old")),
		"qBt-name":     "Demo.Show",
		"qBt-tags":     []any{"tv"},
		"added_time":   int64(1700000000),
		"paused":       int64(0),
		"pieces":       "xxxxxxxx",
	}

	torrentPath := filepath.Join(bt, hash+".torrent")
	frPath := filepath.Join(bt, hash+".fastresume")
	writeRaw(t, torrentPath, torrent)
	writeRaw(t, frPath, fastresume)

	lib, err := LoadLibrary(bt, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Torrents) != 1 {
		t.Fatalf("expected 1 torrent, got %d (errors=%v)", len(lib.Torrents), lib.LoadErrors)
	}

	filtered := lib.Filter(FilterOptions{Tags: []string{"tv"}, MatchAllTags: true})
	torrents := filtered.List()
	if len(torrents) != 1 {
		t.Fatalf("filter: got %d", len(torrents))
	}

	n, err := Scan(context.Background(), torrents, ScanOptions{
		SearchPaths: []string{search},
		SelectBest:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("scan added %d, want 1", n)
	}

	tor := torrents[0]
	if tor.Status() != MatchAll {
		t.Fatalf("status=%s", tor.Status())
	}

	plan, err := tor.MakePlan(PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantSave := filepath.Join(search, "shows")
	if plan.SavePath != wantSave {
		t.Fatalf("save_path=%q want %q", plan.SavePath, wantSave)
	}
	if len(plan.MappedFiles) != 1 || plan.MappedFiles[0] != "Demo/episode.mkv" {
		t.Fatalf("mapped=%v", plan.MappedFiles)
	}

	written, err := tor.Save(plan, SaveOptions{Backup: true, CheckQBitRunning: false})
	if err != nil {
		t.Fatal(err)
	}
	if !written {
		t.Fatal("expected write")
	}

	tor2, err := LoadTorrent(Paths{TorrentFile: torrentPath, FastresumeFile: frPath}, hash)
	if err != nil {
		t.Fatal(err)
	}
	if tor2.Info.SavePath != wantSave {
		t.Fatalf("reloaded save_path=%q", tor2.Info.SavePath)
	}

	backs, _ := filepath.Glob(frPath + ".*.bak")
	if len(backs) != 1 {
		t.Fatalf("expected 1 backup, got %v", backs)
	}
}

func TestMultiFilePlan(t *testing.T) {
	root := t.TempDir()
	bt := filepath.Join(root, "BT_backup")
	search := filepath.Join(root, "data")
	dir := filepath.Join(search, "Album")
	if err := os.MkdirAll(bt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	a := []byte("track-one-audio-bytes-aaaa")
	b := []byte("track-two-audio-bytes-bbbbb")
	if err := os.WriteFile(filepath.Join(dir, "01.flac"), a, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "02.flac"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	hash := "0123456789abcdef0123456789abcdef01234567"
	info := map[string]any{
		"name": "Album",
		"files": []any{
			map[string]any{"path": []any{"01.flac"}, "length": int64(len(a))},
			map[string]any{"path": []any{"02.flac"}, "length": int64(len(b))},
		},
		"piece length": int64(16384),
		"pieces":       []byte("01234567890123456789"),
	}
	writeRaw(t, filepath.Join(bt, hash+".torrent"), map[string]any{"info": info})
	writeRaw(t, filepath.Join(bt, hash+".fastresume"), map[string]any{
		"save_path": filepath.Join(root, "elsewhere"),
	})

	lib, err := LoadLibrary(bt, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	tor := lib.List()[0]
	if _, err := Scan(context.Background(), []*Torrent{tor}, ScanOptions{SearchPaths: []string{search}, SelectBest: true}); err != nil {
		t.Fatal(err)
	}
	plan, err := tor.MakePlan(PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.SavePath != search {
		t.Fatalf("save_path=%q want %q", plan.SavePath, search)
	}
	if len(plan.MappedFiles) != 2 || plan.MappedFiles[0] != "Album/01.flac" || plan.MappedFiles[1] != "Album/02.flac" {
		t.Fatalf("mapped=%v", plan.MappedFiles)
	}
}

func writeRaw(t *testing.T, path string, v map[string]any) {
	t.Helper()
	data, err := encodeBencode(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
