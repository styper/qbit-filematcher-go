// Package assetver loads scripts/versions.json (pinned frontend vendor versions).
package assetver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// File is the shape of scripts/versions.json.
type File struct {
	HTMX     string `json:"htmx"`
	Alpine   string `json:"alpine"`
	Tailwind string `json:"tailwind"`
}

// Load reads <moduleRoot>/scripts/versions.json.
func Load(moduleRoot string) (File, error) {
	path := filepath.Join(moduleRoot, "scripts", "versions.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("read %s: %w", path, err)
	}
	var v File
	if err := json.Unmarshal(b, &v); err != nil {
		return File{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if v.HTMX == "" || v.Alpine == "" || v.Tailwind == "" {
		return File{}, fmt.Errorf("%s: htmx, alpine, and tailwind are required", path)
	}
	return v, nil
}
