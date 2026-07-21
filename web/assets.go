package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
)

const assetHashLen = 12

func loadAssetHashes(fsys fs.FS) (map[string]string, error) {
	b, err := fs.ReadFile(fsys, "assets.json")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("read assets.json: %w", err)
	}
	var hashes map[string]string
	if err := json.Unmarshal(b, &hashes); err != nil {
		return nil, fmt.Errorf("parse assets.json: %w", err)
	}
	if hashes == nil {
		hashes = map[string]string{}
	}
	return hashes, nil
}

// shortContentHash returns a short hex SHA-256 of b for cache-busting.
func shortContentHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:assetHashLen]
}

// staticURL builds /static/<name>?v=<hash> when a content hash is known.
func staticURL(hashes map[string]string, name string) string {
	path := "/static/" + name
	if h := hashes[name]; h != "" {
		return path + "?v=" + h
	}
	return path
}
