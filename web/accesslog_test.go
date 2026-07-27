package web

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/styper/qbit-filematcher-go/config"
)

func TestAccessLog(t *testing.T) {
	var buf bytes.Buffer
	Setup(&buf)
	t.Cleanup(func() { Setup(os.Stdout) })

	h := accessLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz?x=1", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d", rr.Code)
	}
	line := strings.TrimSpace(buf.String())
	for _, want := range []string{
		" [INFO ] access: ",
		"127.0.0.1:1234",
		"GET",
		"/healthz?x=1",
		"201",
		"2",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("access log missing %q in %q", want, line)
		}
	}
	fields := strings.SplitN(line, " ", 2)
	if len(fields) < 1 {
		t.Fatalf("empty line")
	}
	if _, err := time.Parse("2006-01-02T15:04:05.000Z", fields[0]); err != nil {
		t.Errorf("timestamp: %v in %q", err, line)
	}
}

func TestAccessLogFollowsSetupFormatChange(t *testing.T) {
	var buf bytes.Buffer
	SetupWith(&buf, slog.LevelInfo, config.LogFormatText)
	t.Cleanup(func() { SetupWith(os.Stdout, slog.LevelInfo, config.LogFormatText) })

	h := accessLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/a", nil)
	req.RemoteAddr = "127.0.0.1:1"
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !strings.Contains(buf.String(), "[INFO ] access:") {
		t.Fatalf("expected text access log: %q", buf.String())
	}

	buf.Reset()
	SetupWith(&buf, slog.LevelInfo, config.LogFormatJSON)
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/b", nil)
	req.RemoteAddr = "127.0.0.1:1"
	h.ServeHTTP(httptest.NewRecorder(), req)
	got := buf.String()
	if strings.Contains(got, "[INFO ]") {
		t.Fatalf("access log should use new JSON handler after SetupWith: %q", got)
	}
	for _, want := range []string{`"logger":"access"`, `"path":"/b"`, `"level":"INFO"`, `"duration_ms":`} {
		if !strings.Contains(got, want) {
			t.Errorf("JSON access log missing %s in %q", want, got)
		}
	}
	if strings.Contains(got, `"duration":`) {
		t.Errorf("JSON access log should use duration_ms, not duration: %q", got)
	}
}

func TestGetLogger(t *testing.T) {
	var buf bytes.Buffer
	SetupWith(&buf, slog.LevelInfo, config.LogFormatText)
	t.Cleanup(func() { SetupWith(os.Stdout, slog.LevelInfo, config.LogFormatText) })

	logAt(GetLogger("config"), slog.LevelInfo,
		time.Date(2026, 7, 27, 8, 25, 32, 123*int(time.Millisecond), time.UTC),
		"saved /tmp/x.yaml",
		"path", "/tmp/x.yaml",
	)
	got := strings.TrimSpace(buf.String())
	want := "2026-07-27T08:25:32.123Z [INFO ] config: saved /tmp/x.yaml"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestGetLoggerMultilineAtomic(t *testing.T) {
	var buf bytes.Buffer
	SetupWith(&buf, slog.LevelInfo, config.LogFormatText)
	t.Cleanup(func() { SetupWith(os.Stdout, slog.LevelInfo, config.LogFormatText) })

	logAt(GetLogger("scan"), slog.LevelInfo,
		time.Date(2026, 7, 27, 8, 25, 32, 0, time.UTC),
		"starting\n  bt_backup: /data\n  select_best: true",
	)
	got := buf.String()
	want := "2026-07-27T08:25:32.000Z [INFO ] scan: starting\n  bt_backup: /data\n  select_best: true\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLogMinLevelFiltersDebug(t *testing.T) {
	var buf bytes.Buffer
	SetupWith(&buf, slog.LevelInfo, config.LogFormatText)
	t.Cleanup(func() { SetupWith(os.Stdout, slog.LevelInfo, config.LogFormatText) })

	GetLogger("scan").Debug("hidden")
	GetLogger("scan").Info("visible")
	got := buf.String()
	if strings.Contains(got, "hidden") {
		t.Fatalf("DEBUG should be filtered: %q", got)
	}
	if !strings.Contains(got, "[INFO ] scan: visible") {
		t.Fatalf("INFO missing: %q", got)
	}
}

func TestSetupJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	SetupWith(&buf, slog.LevelInfo, config.LogFormatJSON)
	t.Cleanup(func() { SetupWith(os.Stdout, slog.LevelInfo, config.LogFormatText) })

	GetLogger("scan").Info("done", "torrents", 3)
	got := buf.String()
	for _, want := range []string{
		`"msg":"done"`,
		`"logger":"scan"`,
		`"torrents":3`,
		`"level":"INFO"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("JSON log missing %s in %q", want, got)
		}
	}
	if strings.Contains(got, "[INFO ]") {
		t.Errorf("JSON log should not use line format: %q", got)
	}
}

func TestLevelBracketPadding(t *testing.T) {
	if got := levelBracket(slog.LevelInfo); got != "[INFO ]" {
		t.Errorf("INFO: got %q", got)
	}
	if got := levelBracket(slog.LevelWarn); got != "[WARN ]" {
		t.Errorf("WARN: got %q", got)
	}
	if got := levelBracket(slog.LevelError); got != "[ERROR]" {
		t.Errorf("ERROR: got %q", got)
	}
	if got := levelBracket(slog.LevelDebug); got != "[DEBUG]" {
		t.Errorf("DEBUG: got %q", got)
	}
}
