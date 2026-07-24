// Command fetchassets downloads HTMX/Alpine, fetches the Tailwind standalone
// CLI, builds app.css, and writes assets.json content hashes.
//
// Run from the module root or via go:generate in package web:
//
//	go run ./scripts/fetchassets
//	go generate ./web
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/styper/qbit-filematcher-go/scripts/internal/assetver"
)

const userAgent = "qbit-filematcher-go"

var assetFiles = []string{
	"app.css",
	"htmx.min.js",
	"alpine.min.js",
}

func main() {
	htmxFlag := flag.String("htmx", "", "htmx.org version (default: scripts/versions.json)")
	alpineFlag := flag.String("alpine", "", "alpinejs version (default: scripts/versions.json)")
	tailwindFlag := flag.String("tailwind", "", "tailwindcss version tag (default: scripts/versions.json)")
	force := flag.Bool("force", false, "re-download/rebuild even if outputs exist")
	flag.Parse()

	root, err := findModuleRoot()
	if err != nil {
		fail(err)
	}
	vers, err := assetver.Load(root)
	if err != nil {
		fail(err)
	}
	htmxVer := firstNonEmpty(*htmxFlag, os.Getenv("HTMX_VERSION"), vers.HTMX)
	alpineVer := firstNonEmpty(*alpineFlag, os.Getenv("ALPINE_VERSION"), vers.Alpine)
	tailwindVer := firstNonEmpty(*tailwindFlag, os.Getenv("TAILWIND_VERSION"), vers.Tailwind)

	staticDir := filepath.Join(root, "web", "static")
	stylesDir := filepath.Join(root, "web", "styles")
	toolsDir := filepath.Join(root, ".tools")
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		fail(err)
	}
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		fail(err)
	}

	htmxPath := filepath.Join(staticDir, "htmx.min.js")
	alpinePath := filepath.Join(staticDir, "alpine.min.js")
	cssPath := filepath.Join(staticDir, "app.css")
	assetsJSON := filepath.Join(staticDir, "assets.json")

	htmxURL := fmt.Sprintf("https://cdn.jsdelivr.net/npm/htmx.org@%s/dist/htmx.min.js", htmxVer)
	alpineURL := fmt.Sprintf("https://cdn.jsdelivr.net/npm/alpinejs@%s/dist/cdn.min.js", alpineVer)

	if err := downloadIfNeeded(htmxURL, htmxPath, *force); err != nil {
		fail(fmt.Errorf("htmx: %w", err))
	}
	if err := downloadIfNeeded(alpineURL, alpinePath, *force); err != nil {
		fail(fmt.Errorf("alpine: %w", err))
	}

	twName, twURL := tailwindArtifact(tailwindVer)
	twBin := filepath.Join(toolsDir, twName)
	if err := downloadIfNeeded(twURL, twBin, *force); err != nil {
		fail(fmt.Errorf("tailwind cli: %w", err))
	}
	if err := os.Chmod(twBin, 0o755); err != nil {
		fail(err)
	}

	inputCSS := filepath.Join(stylesDir, "input.css")
	templatesGlob := filepath.Join(root, "web", "templates", "*.html")
	needCSS := *force
	if !needCSS {
		needCSS = !fileExists(cssPath) || isNewer(inputCSS, cssPath)
	}
	if !needCSS {
		if matches, _ := filepath.Glob(templatesGlob); len(matches) > 0 {
			for _, m := range matches {
				if isNewer(m, cssPath) {
					needCSS = true
					break
				}
			}
		}
	}
	if needCSS {
		fmt.Println("→ building app.css with Tailwind")
		_ = os.Remove(cssPath)
		cmd := exec.CommandContext(context.Background(), twBin, "-i", inputCSS, "-o", cssPath, "--minify")
		cmd.Dir = root
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fail(fmt.Errorf("tailwind: %w", err))
		}
	} else {
		fmt.Println("→ app.css up to date")
	}

	if err := writeAssetHashes(staticDir, assetsJSON); err != nil {
		fail(err)
	}
	fmt.Println("→ wrote", assetsJSON)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "fetchassets: %v\n", err)
	os.Exit(1)
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}

func downloadIfNeeded(url, dest string, force bool) error {
	if !force {
		if st, err := os.Stat(dest); err == nil && st.Size() > 0 {
			fmt.Println("→ exists", dest)
			return nil
		}
	}
	fmt.Println("→ downloading", url)
	return downloadFile(url, dest)
}

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%s: HTTP %s", url, res.Status)
	}

	tmp := dest + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, res.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, dest)
}

func tailwindArtifact(version string) (name, url string) {
	var osName string
	switch runtime.GOOS {
	case "darwin":
		osName = "macos"
	case "windows":
		osName = "windows"
	case "linux":
		osName = "linux"
	default:
		osName = runtime.GOOS
	}
	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	default:
		arch = runtime.GOARCH
	}
	name = fmt.Sprintf("tailwindcss-%s-%s", osName, arch)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	url = fmt.Sprintf(
		"https://github.com/tailwindlabs/tailwindcss/releases/download/%s/%s",
		version, name,
	)
	return name, url
}

func writeAssetHashes(staticDir, outPath string) error {
	hashes := make(map[string]string, len(assetFiles))
	for _, name := range assetFiles {
		path := filepath.Join(staticDir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("hash %s: %w", path, err)
		}
		sum := sha256.Sum256(b)
		hashes[name] = hex.EncodeToString(sum[:])[:12]
	}
	encoded, err := json.MarshalIndent(hashes, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(outPath, encoded, 0o644)
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Size() > 0
}

func isNewer(src, dst string) bool {
	s, err := os.Stat(src)
	if err != nil {
		return false
	}
	d, err := os.Stat(dst)
	if err != nil {
		return true
	}
	return s.ModTime().After(d.ModTime())
}
