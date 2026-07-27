package web

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/styper/qbit-filematcher-go/config"
	"github.com/styper/qbit-filematcher-go/filematcher"
)

func withLogBuffer(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	SetupWith(&buf, slog.LevelDebug, config.LogFormatText) // capture all levels in tests
	t.Cleanup(func() { SetupWith(os.Stdout, slog.LevelInfo, config.LogFormatText) })
	return &buf
}

func TestLogMatchScanParams(t *testing.T) {
	buf := withLogBuffer(t)
	logMatchScanParams(config.Settings{
		BTBackupLocation: "/data/BT_backup",
		SearchPaths:      []string{"/media/a", "/media/b"},
		ExcludeDirs:      []string{".Trash", "#recycle"},
	})
	got := buf.String()
	for _, want := range []string{
		" [INFO ] scan: starting",
		"bt_backup: /data/BT_backup",
		"search_paths: /media/a, /media/b",
		"exclude_dirs: .Trash, #recycle",
		"select_best: true",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("log missing %q in:\n%s", want, got)
		}
	}
	first := strings.SplitN(got, "\n", 2)[0]
	ts := strings.Fields(first)[0]
	if _, err := time.Parse("2006-01-02T15:04:05.000Z", ts); err != nil {
		t.Errorf("timestamp on first line: %v in %q", err, first)
	}
	lines := strings.Split(strings.TrimSpace(got), "\n")
	for i, line := range lines[1:] {
		if strings.HasPrefix(line, "20") && strings.Contains(line, " [") {
			t.Errorf("continuation line %d looks timestamped: %q", i+1, line)
		}
	}
}

func TestLogLibraryLoadErrors(t *testing.T) {
	buf := withLogBuffer(t)
	logLibraryLoadErrors(&filematcher.Library{
		LoadErrors: map[string]error{
			"bbbb": errors.New("bad fastresume"),
			"aaaa": errors.New("bad torrent"),
		},
	})
	got := buf.String()
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), got)
	}
	if !strings.Contains(lines[0], " [WARN ] scan: skipped aaaa:") {
		t.Errorf("first line should be sorted aaaa, got %q", lines[0])
	}
	if !strings.Contains(lines[1], " [WARN ] scan: skipped bbbb:") {
		t.Errorf("second line should be bbbb, got %q", lines[1])
	}

	buf.Reset()
	logLibraryLoadErrors(&filematcher.Library{})
	if buf.Len() != 0 {
		t.Errorf("empty LoadErrors should log nothing, got %q", buf.String())
	}
}

func TestLogMatchSave(t *testing.T) {
	buf := withLogBuffer(t)
	root := filepath.Join(string(filepath.Separator), "data")
	curSave := filepath.Join(root, "old")
	newSave := filepath.Join(root, "new")
	selected := filepath.Join(newSave, "Show", "ep.mkv")

	tor := &filematcher.Torrent{
		Info: filematcher.Info{
			HashV1:   "aabbccddeeff00112233445566778899aabbccdd",
			Name:     "Demo",
			SavePath: curSave,
			Files: []filematcher.File{
				{Index: 0, Path: "Show/ep.mkv", Size: 100},
			},
		},
		Matches: map[int]*filematcher.FileMatch{
			0: {Candidates: []string{selected}, SelectedIndex: 0},
		},
	}
	plan := &filematcher.SavePlan{
		SavePath:     newSave,
		MappedFiles:  []string{"Show/ep.mkv"},
		IsIncomplete: false,
	}

	logMatchSave(tor, plan, true)
	got := buf.String()
	for _, want := range []string{
		" [INFO ] save: Demo (aabbccddeeff00112233445566778899aabbccdd)",
		"status: all_matches",
		"location: changed",
		"allow_incomplete: true",
		"current_save_path: " + curSave,
		"current[0]: Show/ep.mkv -> " + selected,
		"plan_save_path: " + newSave,
		"plan_incomplete: false",
		"plan[0]: Show/ep.mkv",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("log missing %q in:\n%s", want, got)
		}
	}
}
