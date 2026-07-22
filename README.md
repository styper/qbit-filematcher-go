# qBit Filematcher

Rematch on-disk files to qBittorrent torrents after content has been moved.

It works directly on qBittorrent's `BT_backup` directory (paired `.torrent` / `.fastresume` files). There is **no** qBittorrent Web API dependency.

One binary, two modes:

| Mode | Command | Stack |
|------|---------|-------|
| CLI | `qbit-filematcher match` / `config` | flags / prompts |
| WEB | `qbit-filematcher web` | HTMX + Alpine.js + Tailwind CSS |

![WEB home — rematch moved torrent files](assets/screenshots/web-home.png)

## What it does

1. Load torrents from `BT_backup`
2. Optionally filter by hash / name / tags
3. Scan configured search paths for candidates matched by **extension + size**
4. Select among candidates (interactive, `--auto`, or WEB UI)
5. Compute a NoSubfolder-style `save_path` + `mapped_files`
6. Rewrite `.fastresume` (with timestamped backup), preferably while qBittorrent is closed

## Configuration

Config file: `qbit-filematcher.yaml`. Lookup order:

1. `--config` path (if set)
2. `<binary-dir>/qbit-filematcher.yaml` (portable / next to the executable)
3. OS user config dir:
   - Linux: `$XDG_CONFIG_HOME/qbit-filematcher/qbit-filematcher.yaml` (default `~/.config/…`)
   - macOS: `~/Library/Application Support/qbit-filematcher/qbit-filematcher.yaml`
   - Windows: `%APPDATA%\qbit-filematcher\qbit-filematcher.yaml`

New saves go to the user config path when no file exists yet.

Persisted keys:

- `bt_backup_location`
- `search_paths`
- `exclude_dirs`
- `host` / `port` (WEB)

## CLI

```bash
qbit-filematcher match \
  -b ~/.local/share/data/qBittorrent/BT_backup \
  -s /data/media \
  -e .trash -e .git \
  --auto --dry-run

qbit-filematcher config view
qbit-filematcher config edit
```

Useful flags: `-a`/`--hash`, `-t`/`--tag`, `-n`/`--name`, `--incomplete`, `--dry-run`, `--auto`, `--config`.

## WEB

```bash
qbit-filematcher web --host localhost --port 8080
```

Pages: `/` home, `/config`, `/match` (scan + list), `/match/{hash}` (select candidates + save).

## Safety notes

- Every fastresume update writes a timestamped `.bak` beside the original.
- Saves refuse to run while qBittorrent appears to be running (CLI/WEB and library default).
- Incomplete matches (when allowed) clear piece data so qBittorrent rechecks on next start.

---

## Development

### Prerequisites

- Go 1.22+
- `curl` (to fetch frontend assets)
- qBittorrent installed locally (for a real `BT_backup`)
- [GoReleaser](https://goreleaser.com/) v2 (optional; for release builds):

```bash
go install github.com/goreleaser/goreleaser/v2@latest
```

- [go-licenses](https://github.com/google/go-licenses) (for regenerating `third_party/`):

```bash
go install github.com/google/go-licenses@latest
```

### Build

Frontend vendor sources are **not** committed. Download and build them first:

```bash
make assets
make build
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

### Licensing

Project license: [Unlicense](LICENSE) (public domain dedication). See [NOTICE](NOTICE) for third-party overview.

License texts for Go modules and embedded HTMX/Alpine live under `third_party/` (included in release archives):

```bash
make licenses        # regenerate third_party/ after dependency or asset version changes
make licenses-check  # CI gate (also part of make check)
```

### Layout

```text
filematcher/                 Standalone core library (importable)
config/                      Shared YAML settings
cmd/qbit-filematcher/        Single binary entrypoint (CLI + WEB)
cli/                         CLI commands (match, config, web)
web/                         WEB application (handlers, templates)
web/styles/                  Tailwind input CSS (app-owned)
web/static/                  Downloaded HTMX / Alpine / built CSS (not in git)
third_party/                 Dependency license texts (Go + HTMX/Alpine)
scripts/gen-licenses.sh      Regenerates third_party/
LICENSE                      Unlicense
NOTICE                       Third-party overview (hand-maintained)
web/templates/               HTML templates
web/icon.svg                 App icon (embedded into the binary)
assets/icon/                 PNG exports of the app icon (512 / 1024)
assets/screenshots/          README screenshots
```

### Library usage

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

### Quality checks

```bash
make fmt         # rewrite Go sources with gofmt
make check       # fmt-check → vet → gosec → govulncheck → test
make check-race  # same, with race detector for tests
make gosec       # static security analysis only
make govulncheck # dependency vulnerability scan only
make build-all   # cross-compile → dist/
make tidy
make clean       # removes bin/, dist/, .tools/, and downloaded static assets
make icon-pngs   # regenerate assets/icon PNGs from web/icon.svg
```

## Roadmap

- **Packaging**:
  - Windows
  - macOS
  - Linux
- **Signed releases**
