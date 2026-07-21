package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestStaticURL(t *testing.T) {
	hashes := map[string]string{"app.css": "abc123def456"}
	if got := staticURL(hashes, "app.css"); got != "/static/app.css?v=abc123def456" {
		t.Fatalf("got %q", got)
	}
	if got := staticURL(hashes, "missing.js"); got != "/static/missing.js" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadAssetHashes(t *testing.T) {
	fsys := fstest.MapFS{
		"assets.json": &fstest.MapFile{
			Data: []byte(`{"app.css":"aabbccddeeff","htmx.min.js":"112233445566"}`),
		},
	}
	got, err := loadAssetHashes(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if got["app.css"] != "aabbccddeeff" || got["htmx.min.js"] != "112233445566" {
		t.Fatalf("got %#v", got)
	}

	empty, err := loadAssetHashes(fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected empty map, got %#v", empty)
	}
}

func TestShortContentHash(t *testing.T) {
	sum := sha256.Sum256([]byte("hello"))
	want := hex.EncodeToString(sum[:])[:12]
	if got := shortContentHash([]byte("hello")); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLoadAssetHashesInvalidJSON(t *testing.T) {
	_, err := loadAssetHashes(fstest.MapFS{
		"assets.json": &fstest.MapFile{Data: []byte(`{`)},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

// Ensure fs.ErrNotExist path is exercised via a real sub-FS miss.
func TestLoadAssetHashesMissingUsesEmpty(t *testing.T) {
	sub, err := fs.Sub(fstest.MapFS{"other.txt": &fstest.MapFile{Data: []byte("x")}}, ".")
	if err != nil {
		t.Fatal(err)
	}
	got, err := loadAssetHashes(sub)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("%#v", got)
	}
}

func TestAssetsJSONRoundTrip(t *testing.T) {
	raw, err := json.Marshal(map[string]string{
		"app.css":       "deadbeefcafe",
		"htmx.min.js":   "cafebabef00d",
		"alpine.min.js": "0123456789ab",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := loadAssetHashes(fstest.MapFS{"assets.json": &fstest.MapFile{Data: raw}})
	if err != nil {
		t.Fatal(err)
	}
	if staticURL(got, "alpine.min.js") != "/static/alpine.min.js?v=0123456789ab" {
		t.Fatalf("%q", staticURL(got, "alpine.min.js"))
	}
}
