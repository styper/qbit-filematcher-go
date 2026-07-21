// Command hashassets writes web/static/assets.json with short content hashes
// for cache-busting query strings.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var assetFiles = []string{
	"app.css",
	"htmx.min.js",
	"alpine.min.js",
}

func main() {
	dir := flag.String("dir", "web/static", "directory containing static assets")
	out := flag.String("o", "", "output assets.json path (default: <dir>/assets.json)")
	flag.Parse()
	if *out == "" {
		*out = filepath.Join(*dir, "assets.json")
	}

	hashes := make(map[string]string, len(assetFiles))
	for _, name := range assetFiles {
		path := filepath.Join(*dir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hashassets: read %s: %v\n", path, err)
			os.Exit(1)
		}
		sum := sha256.Sum256(b)
		hashes[name] = hex.EncodeToString(sum[:])[:12]
	}

	encoded, err := json.MarshalIndent(hashes, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "hashassets: encode: %v\n", err)
		os.Exit(1)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(*out, encoded, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "hashassets: write %s: %v\n", *out, err)
		os.Exit(1)
	}
}
