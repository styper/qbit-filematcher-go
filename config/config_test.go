package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidatePaths(t *testing.T) {
	root := t.TempDir()
	bt := filepath.Join(root, "BT_backup")
	search := filepath.Join(root, "media")
	if err := os.MkdirAll(bt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(search, 0o755); err != nil {
		t.Fatal(err)
	}

	s := Settings{
		BTBackupLocation: bt,
		SearchPaths:      []string{search},
		Host:             "localhost",
		Port:             8080,
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("expected valid: %v", err)
	}

	s.SearchPaths = nil
	if err := s.ValidatePaths(); err == nil {
		t.Fatal("expected empty search paths to fail")
	}

	s.SearchPaths = []string{filepath.Join(root, "missing")}
	if err := s.ValidatePaths(); err == nil {
		t.Fatal("expected missing search path to fail")
	}
}

func TestValidateListen(t *testing.T) {
	cases := []struct {
		host string
		port int
		ok   bool
	}{
		{"localhost", 8080, true},
		{"0.0.0.0", 1024, true},
		{"::", 65535, true},
		{"", 8080, false},
		{"localhost:8080", 8080, false},
		{"bad host", 8080, false},
		{"localhost", 80, false},
		{"localhost", 70000, false},
	}
	for _, tc := range cases {
		s := Settings{Host: tc.host, Port: tc.port}
		err := s.ValidateListen()
		if tc.ok && err != nil {
			t.Errorf("host=%q port=%d: unexpected error %v", tc.host, tc.port, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("host=%q port=%d: expected error", tc.host, tc.port)
		}
	}
}

func TestValidateLogging(t *testing.T) {
	ok := []Settings{
		{},
		{LogFormat: "text", LogLevel: "info"},
		{LogFormat: "JSON", LogLevel: "WARN"},
		{LogFormat: "json", LogLevel: "warning"},
		{LogFormat: "line", LogLevel: "debug"}, // alias for text
		{LogFormat: "json", LogLevel: "error"},
	}
	for _, s := range ok {
		if err := s.ValidateLogging(); err != nil {
			t.Errorf("%+v: unexpected error %v", s, err)
		}
	}
	bad := []Settings{
		{LogFormat: "xml"},
		{LogLevel: "trace"},
	}
	for _, s := range bad {
		if err := s.ValidateLogging(); err == nil {
			t.Errorf("%+v: expected error", s)
		}
	}
	if got := (Settings{}).EffectiveLogFormat(); got != LogFormatText {
		t.Fatalf("EffectiveLogFormat=%q", got)
	}
	if got := (Settings{LogFormat: "line"}).EffectiveLogFormat(); got != LogFormatText {
		t.Fatalf("line alias EffectiveLogFormat=%q", got)
	}
	if got := (Settings{}).EffectiveLogLevel(); got != LogLevelInfo {
		t.Fatalf("EffectiveLogLevel=%q", got)
	}
}

func TestUserPath(t *testing.T) {
	p, err := UserPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != DefaultFileName {
		t.Fatalf("basename=%q want %q", filepath.Base(p), DefaultFileName)
	}
	if filepath.Base(filepath.Dir(p)) != AppConfigDirName {
		t.Fatalf("parent=%q want %q", filepath.Base(filepath.Dir(p)), AppConfigDirName)
	}
}

func TestPathExplicit(t *testing.T) {
	want := filepath.Join(t.TempDir(), "custom.yaml")
	if got := Path(want); got != want {
		t.Fatalf("Path(%q)=%q", want, got)
	}
}

func TestPathSearchOrder(t *testing.T) {
	root := t.TempDir()
	isolateUserConfig(t, root)

	exeDir := t.TempDir()
	testExeDir = exeDir
	t.Cleanup(func() { testExeDir = "" })

	user, err := UserPath()
	if err != nil {
		t.Fatal(err)
	}
	if got := Path(""); got != user {
		t.Fatalf("no files: Path()=%q want user %q", got, user)
	}

	if err := os.MkdirAll(filepath.Dir(user), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(user, []byte("host: userhost\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Path(""); got != user {
		t.Fatalf("user only: Path()=%q want %q", got, user)
	}

	adj := filepath.Join(exeDir, DefaultFileName)
	if err := os.WriteFile(adj, []byte("host: adjhost\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Path(""); got != adj {
		t.Fatalf("adjacent+user: Path()=%q want %q", got, adj)
	}
	s, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if s.Host != "adjhost" {
		t.Fatalf("Load host=%q want adjhost (binary-adjacent wins)", s.Host)
	}
}

func isolateUserConfig(t *testing.T, root string) {
	t.Helper()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("AppData", root)
	case "darwin":
		t.Setenv("HOME", root)
	default:
		t.Setenv("XDG_CONFIG_HOME", root)
	}
}
