# Contributing

Build, layout, library usage, and quality checks for [qBit Filematcher](README.md).

## Prerequisites

- Go 1.22+ (and network access to fetch frontend assets / license texts when generating)
- qBittorrent installed locally (for a real `BT_backup`)
- [GoReleaser](https://goreleaser.com/) v2 (optional; for release builds):

```bash
go install github.com/goreleaser/goreleaser/v2@latest
```

- [go-licenses](https://github.com/google/go-licenses) (for regenerating `third_party/`):

```bash
go install github.com/google/go-licenses@latest
```

- [golangci-lint](https://golangci-lint.run/) v2 (for `make lint` / `make check`):

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
```

`make` is optional — it wraps the same `go run` / `go generate` steps.

Pinned HTMX / Alpine / Tailwind versions live in one place: [`scripts/versions.json`](scripts/versions.json). To bump: edit that file, then `go run ./scripts/fetchassets -force` (and `go run ./scripts/genlicenses` if HTMX/Alpine changed).

## Build

Frontend vendor sources are **not** committed. Generate them first (Go-only):

```bash
go generate ./web
# equivalent: go run ./scripts/fetchassets
# or:        make assets

go build -o bin/qbit-filematcher ./cmd/qbit-filematcher
# or: make build
```

Outputs a single binary at `bin/qbit-filematcher`.

```bash
make help                  # list all Makefile targets
make build VERSION=1.0.0   # stamp a release version into the binary
```

Cross-compile with Make:

```bash
make build-windows-amd64   # → dist/qbit-filematcher-windows-amd64.exe
make build-linux-arm64     # → dist/qbit-filematcher-linux-arm64
make build-all             # all OS/arch combos → dist/
```

Or with GoReleaser (same OS/arch targets as `make build-all`, including `make assets`):

```bash
goreleaser release --snapshot --clean   # local dry-run into dist/
goreleaser release --clean              # real release (needs a v* tag + GITHUB_TOKEN)
```

Release artifacts are versioned archives (`qbit-filematcher_<version>_<os>_<arch>.tar.gz`, `.zip` on Windows). Inside each archive: the binary (`qbit-filematcher` / `.exe`), `LICENSE`, `NOTICE`, `README.md`, and `third_party/` (dependency license texts). `make build-all` still writes bare binaries under `dist/` for local/CI use.

Tagged `v*` pushes run GoReleaser in GitHub Actions (see `.github/workflows/release.yml`).

## Licensing

Project license: [Unlicense](LICENSE) (public domain dedication). See [NOTICE](NOTICE) for third-party overview.

License texts for Go modules and embedded HTMX/Alpine live under `third_party/` (included in release archives):

```bash
go run ./scripts/genlicenses        # regenerate third_party/
go run ./scripts/genlicenses -check # CI gate (also: make licenses-check)
# or: make licenses / make licenses-check
```

## Layout

```text
filematcher/                 Standalone core library (importable)
config/                      Shared YAML settings
cmd/qbit-filematcher/        Single binary entrypoint (CLI + WEB)
cli/                         CLI commands (match, config, web)
web/                         WEB application (handlers, templates)
web/styles/                  Tailwind input CSS (app-owned)
web/static/                  Downloaded HTMX / Alpine / built CSS (not in git)
third_party/                 Dependency license texts (Go + HTMX/Alpine)
scripts/versions.json        Pinned HTMX / Alpine / Tailwind versions
scripts/fetchassets/         Downloads vendors + builds CSS (go:generate ./web)
scripts/genlicenses/         Regenerates third_party/
scripts/hashassets/          Writes web/static/assets.json hashes only
LICENSE                      Unlicense
NOTICE                       Third-party overview (hand-maintained)
web/templates/               HTML templates
web/icon.svg                 App icon (embedded into the binary)
assets/icon/                 PNG exports of the app icon (512 / 1024)
assets/screenshots/          README screenshots
```

## Library usage

```go
import "github.com/styper/qbit-filematcher-go/filematcher"

lib, err := filematcher.LoadLibrary(btBackup, filematcher.LoadOptions{})
if err != nil {
    return err
}

filtered := lib.Filter(filematcher.FilterOptions{Tags: []string{"linux"}, MatchAllTags: true})
torrents := filtered.List()

if _, err := filematcher.Scan(context.Background(), torrents, filematcher.ScanOptions{
    SearchPaths: []string{"/data/media"},
    ExcludeDirs: []string{".git", ".trash"},
    SelectBest:  true,
}); err != nil {
    return err
}

for _, t := range torrents {
    plan, err := t.MakePlan(filematcher.PlanOptions{})
    if err != nil {
        return err
    }
    if _, err := t.Save(plan, filematcher.DefaultSaveOptions()); err != nil {
        return err
    }
}
```

## Quality checks

```bash
make fmt         # rewrite Go sources (gofumpt + goimports via golangci-lint)
make lint        # golangci-lint (format check + linters; see .golangci.yml)
make check       # lint → govulncheck → licenses-check → test
make check-race  # same, with race detector for tests
make govulncheck # dependency vulnerability scan only
make build-all   # cross-compile → dist/
make tidy
make clean       # removes bin/, dist/, .tools/, and downloaded static assets
make icon-pngs   # regenerate assets/icon PNGs from web/icon.svg
```
