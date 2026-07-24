package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestMatchHelpGroupsFlags(t *testing.T) {
	var out bytes.Buffer
	app := &App{Stdout: &out, Stderr: &out, Stdin: strings.NewReader("")}
	if err := app.Run(t.Context(), []string{"match", "--help"}); err != nil {
		t.Fatalf("match --help: %v", err)
	}
	got := out.String()

	for _, want := range []string{
		"After merge, --bt-backup and at least one --search path are required.",
		"Config Overrides:",
		"--bt-backup",
		"--search",
		"--exclude",
		"Flags:",
		"--auto",
		"--dry-run",
		"Filters:",
		"--hash",
		"--tag",
		"--name",
		"Global Flags:",
		"--config",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("help missing %q\n---\n%s", want, got)
		}
	}

	cfgIdx := strings.Index(got, "Config Overrides:")
	flagsIdx := strings.Index(got, "\nFlags:")
	filtersIdx := strings.Index(got, "\nFilters:")
	globalIdx := strings.Index(got, "Global Flags:")
	if cfgIdx < 0 || flagsIdx < 0 || filtersIdx < 0 || globalIdx < 0 {
		t.Fatalf("missing section headers\n---\n%s", got)
	}
	if cfgIdx >= flagsIdx || flagsIdx >= filtersIdx || filtersIdx >= globalIdx {
		t.Fatalf("expected Config Overrides → Flags → Filters → Global; indices %d %d %d %d",
			cfgIdx, flagsIdx, filtersIdx, globalIdx)
	}

	cfgSection := got[cfgIdx:flagsIdx]
	cliSection := got[flagsIdx:filtersIdx]
	filterSection := got[filtersIdx:globalIdx]
	for _, name := range []string{"--auto", "--incomplete", "--dry-run", "--hash", "--tag", "--name"} {
		if strings.Contains(cfgSection, name) {
			t.Errorf("config overrides section unexpectedly contains %s", name)
		}
	}
	for _, name := range []string{"--bt-backup", "--search", "--exclude", "--hash", "--tag", "--name"} {
		if strings.Contains(cliSection, name) {
			t.Errorf("Flags section unexpectedly contains %s", name)
		}
	}
	for _, name := range []string{"--bt-backup", "--search", "--exclude", "--auto", "--dry-run"} {
		if strings.Contains(filterSection, name) {
			t.Errorf("Filters section unexpectedly contains %s", name)
		}
	}
}
