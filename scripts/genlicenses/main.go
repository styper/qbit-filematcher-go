// Command genlicenses regenerates third_party/ license materials.
//
// Requires go-licenses on PATH (or $(go env GOPATH)/bin). See CONTRIBUTING.md.
//
//	go run ./scripts/genlicenses
//	go run ./scripts/genlicenses -check
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/styper/qbit-filematcher-go/scripts/internal/assetver"
)

const (
	cmdPkg    = "./cmd/qbit-filematcher"
	userAgent = "qbit-filematcher-go"
)

func main() {
	htmxFlag := flag.String("htmx", "", "htmx.org version (default: scripts/versions.json)")
	alpineFlag := flag.String("alpine", "", "alpinejs version (default: scripts/versions.json)")
	outDir := flag.String("out", "", "output third_party directory (default: <module>/third_party)")
	check := flag.Bool("check", false, "fail if committed third_party/ differs from a fresh generation")
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

	module, err := modulePath(root)
	if err != nil {
		fail(err)
	}
	goLicenses, err := findGoLicenses()
	if err != nil {
		fail(err)
	}

	committed := filepath.Join(root, "third_party")
	dest := *outDir
	if dest == "" {
		dest = committed
	}
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(root, dest)
	}

	if *check {
		tmp, err := os.MkdirTemp("", "qbit-licenses-*")
		if err != nil {
			fail(err)
		}
		defer os.RemoveAll(tmp)
		genDir := filepath.Join(tmp, "third_party")
		if err := generate(root, module, goLicenses, genDir, htmxVer, alpineVer); err != nil {
			fail(err)
		}
		if err := diffTrees(committed, genDir); err != nil {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "third_party/ is out of date. Run: go run ./scripts/genlicenses && commit the result.")
			fail(err)
		}
		fmt.Println("third_party/ is up to date")
		return
	}

	if err := generate(root, module, goLicenses, dest, htmxVer, alpineVer); err != nil {
		fail(err)
	}
	fmt.Println("done:", dest)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func generate(root, module, goLicenses, dest, htmxVer, alpineVer string) error {
	outGo := filepath.Join(dest, "go")
	outWeb := filepath.Join(dest, "web")
	csvPath := filepath.Join(dest, "licenses.csv")

	fmt.Println("→ collecting Go dependency licenses into", dest)
	if err := os.RemoveAll(outGo); err != nil {
		return err
	}
	if err := os.MkdirAll(outGo, 0o755); err != nil {
		return err
	}

	hostGOOS := runtime.GOOS
	if err := mergeSave(root, goLicenses, module, hostGOOS, outGo); err != nil {
		return err
	}
	if err := mergeSave(root, goLicenses, module, "windows", outGo); err != nil {
		return err
	}

	fmt.Println("→ writing licenses.csv")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	lines := map[string]struct{}{}
	for _, goos := range []string{hostGOOS, "windows"} {
		out, err := reportLicenses(root, goLicenses, goos)
		if err != nil {
			return err
		}
		sc := bufio.NewScanner(bytes.NewReader(out))
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, module+",") {
				continue
			}
			lines[line] = struct{}{}
		}
		if err := sc.Err(); err != nil {
			return err
		}
	}
	sorted := make([]string, 0, len(lines))
	for line := range lines {
		sorted = append(sorted, line)
	}
	sort.Strings(sorted) // Go sort is bytewise (C locale–like)
	var csv bytes.Buffer
	for _, line := range sorted {
		csv.WriteString(line)
		csv.WriteByte('\n')
	}
	if err := os.WriteFile(csvPath, csv.Bytes(), 0o644); err != nil {
		return err
	}

	fmt.Println("→ fetching embedded web asset licenses")
	if err := os.MkdirAll(outWeb, 0o755); err != nil {
		return err
	}
	htmxName := fmt.Sprintf("htmx-v%s.LICENSE", htmxVer)
	alpineName := fmt.Sprintf("alpine-v%s.LICENSE.md", alpineVer)
	htmxURL := fmt.Sprintf("https://raw.githubusercontent.com/bigskysoftware/htmx/v%s/LICENSE", htmxVer)
	alpineURL := fmt.Sprintf("https://raw.githubusercontent.com/alpinejs/alpine/v%s/LICENSE.md", alpineVer)
	if err := downloadFile(htmxURL, filepath.Join(outWeb, htmxName)); err != nil {
		return fmt.Errorf("htmx license: %w", err)
	}
	if err := downloadFile(alpineURL, filepath.Join(outWeb, alpineName)); err != nil {
		return fmt.Errorf("alpine license: %w", err)
	}
	return pruneOtherVersions(outWeb, htmxName, alpineName)
}

func mergeSave(root, goLicenses, module, goos, dest string) error {
	fmt.Printf("→ go-licenses save (GOOS=%s)\n", goos)
	stage, err := os.MkdirTemp("", "go-licenses-save-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	cmd := exec.CommandContext(context.Background(), goLicenses, "save", cmdPkg,
		"--save_path="+stage,
		"--force",
		"--ignore="+module,
	)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS="+goos)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go-licenses save GOOS=%s: %w", goos, err)
	}
	chmodWritable(stage)
	chmodWritable(dest)
	return copyMerge(stage, dest)
}

func reportLicenses(root, goLicenses, goos string) ([]byte, error) {
	cmd := exec.CommandContext(context.Background(), goLicenses, "report", cmdPkg)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS="+goos)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(out) == 0 {
			return nil, fmt.Errorf("go-licenses report GOOS=%s: %w\n%s", goos, err, ee.Stderr)
		}
	}
	return out, nil
}

func findGoLicenses() (string, error) {
	if p, err := exec.LookPath("go-licenses"); err == nil {
		return p, nil
	}
	out, err := exec.CommandContext(context.Background(), "go", "env", "GOPATH").Output()
	if err != nil {
		return "", fmt.Errorf("go-licenses not found (and go env GOPATH failed: %w)", err)
	}
	gopath := strings.TrimSpace(string(out))
	candidate := filepath.Join(gopath, "bin", "go-licenses")
	if runtime.GOOS == "windows" {
		candidate += ".exe"
	}
	if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
		return candidate, nil
	}
	return "", fmt.Errorf("go-licenses not found\nInstall it, then re-run:\n  go install github.com/google/go-licenses@latest")
}

func modulePath(root string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "go", "list", "-m")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
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

func fail(err error) {
	fmt.Fprintf(os.Stderr, "genlicenses: %v\n", err)
	os.Exit(1)
}

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 2 * time.Minute}
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

func pruneOtherVersions(webDir, keepHTMX, keepAlpine string) error {
	entries, err := os.ReadDir(webDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		switch {
		case strings.HasPrefix(name, "htmx-v") && strings.HasSuffix(name, ".LICENSE") && name != keepHTMX:
			_ = os.Remove(filepath.Join(webDir, name))
		case strings.HasPrefix(name, "alpine-v") && strings.HasSuffix(name, ".LICENSE.md") && name != keepAlpine:
			_ = os.Remove(filepath.Join(webDir, name))
		}
	}
	return nil
}

func chmodWritable(root string) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // best-effort chmod of module-cache copies
		}
		info, err := d.Info()
		if err != nil {
			return nil //nolint:nilerr // best-effort
		}
		mode := info.Mode()
		_ = os.Chmod(path, mode|0o200) // u+w
		if d.IsDir() {
			_ = os.Chmod(path, mode|0o700)
		}
		return nil
	})
}

func copyMerge(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func diffTrees(a, b string) error {
	var mismatches []string
	err := filepath.WalkDir(a, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(a, path)
		if err != nil {
			return err
		}
		other := filepath.Join(b, rel)
		if d.IsDir() {
			st, err := os.Stat(other)
			if err != nil || !st.IsDir() {
				mismatches = append(mismatches, "only in committed: "+rel+"/")
			}
			return nil
		}
		ab, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		bb, err := os.ReadFile(other)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				mismatches = append(mismatches, "only in committed: "+rel)
				return nil
			}
			return err
		}
		if !bytes.Equal(ab, bb) {
			mismatches = append(mismatches, "differ: "+rel)
		}
		return nil
	})
	if err != nil {
		return err
	}
	err = filepath.WalkDir(b, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(b, path)
		if err != nil {
			return err
		}
		other := filepath.Join(a, rel)
		if _, err := os.Stat(other); err != nil {
			mismatches = append(mismatches, "only in generated: "+rel)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(mismatches) == 0 {
		return nil
	}
	for _, m := range mismatches {
		fmt.Fprintln(os.Stderr, m)
	}
	return fmt.Errorf("%d path(s) differ", len(mismatches))
}
