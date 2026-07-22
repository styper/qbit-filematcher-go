#!/usr/bin/env bash
# Regenerate third_party/ license materials for redistribution.
# Go deps: google/go-licenses (host GOOS + windows, so Windows-only imports are covered).
# Web assets: HTMX / Alpine license texts pinned to Makefile versions.
#
# Requires go-licenses on PATH (or $(go env GOPATH)/bin). See README prerequisites.
# Do not use `GOOS=windows go run …go-licenses` — that cross-compiles the tool itself.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

GO="${GO:-go}"
MODULE="$("$GO" list -m)"
CMD_PKG="./cmd/qbit-filematcher"

HTMX_VERSION="${HTMX_VERSION:-2.0.4}"
ALPINE_VERSION="${ALPINE_VERSION:-3.14.8}"

THIRD_PARTY_DIR="${THIRD_PARTY_DIR:-$ROOT/third_party}"
OUT_GO="$THIRD_PARTY_DIR/go"
OUT_WEB="$THIRD_PARTY_DIR/web"
CSV="$THIRD_PARTY_DIR/licenses.csv"

if command -v go-licenses >/dev/null 2>&1; then
	GO_LICENSES=(go-licenses)
elif [[ -x "$("$GO" env GOPATH)/bin/go-licenses" ]]; then
	GO_LICENSES=("$("$GO" env GOPATH)/bin/go-licenses")
else
	echo "error: go-licenses not found" >&2
	echo "Install it, then re-run:" >&2
	echo "  go install github.com/google/go-licenses@latest" >&2
	exit 1
fi

merge_save() {
	local goos="$1"
	local dest="$2"
	local stage
	stage="$(mktemp -d)"
	echo "→ go-licenses save (GOOS=$goos)"
	GOOS="$goos" "${GO_LICENSES[@]}" save "$CMD_PKG" \
		--save_path="$stage" \
		--force \
		--ignore="$MODULE"
	# Module-cache copies are often mode 0444; make writable so merges can overwrite.
	chmod -R u+w "$stage" "$dest" 2>/dev/null || true
	mkdir -p "$dest"
	cp -a "$stage"/. "$dest"/
	rm -rf "$stage"
}

echo "→ collecting Go dependency licenses into $THIRD_PARTY_DIR"
rm -rf "$OUT_GO"
mkdir -p "$OUT_GO"
merge_save "$("$GO" env GOOS)" "$OUT_GO"
merge_save windows "$OUT_GO"

echo "→ writing licenses.csv"
mkdir -p "$THIRD_PARTY_DIR"
{
	"${GO_LICENSES[@]}" report "$CMD_PKG" 2>/dev/null || true
	GOOS=windows "${GO_LICENSES[@]}" report "$CMD_PKG" 2>/dev/null || true
} | grep -v "^${MODULE}," | LC_ALL=C sort -u >"$CSV"

echo "→ fetching embedded web asset licenses"
mkdir -p "$OUT_WEB"
curl -fsSL \
	"https://raw.githubusercontent.com/bigskysoftware/htmx/v${HTMX_VERSION}/LICENSE" \
	-o "$OUT_WEB/htmx-v${HTMX_VERSION}.LICENSE"
curl -fsSL \
	"https://raw.githubusercontent.com/alpinejs/alpine/v${ALPINE_VERSION}/LICENSE.md" \
	-o "$OUT_WEB/alpine-v${ALPINE_VERSION}.LICENSE.md"

find "$OUT_WEB" -maxdepth 1 -type f -name 'htmx-v*.LICENSE' ! -name "htmx-v${HTMX_VERSION}.LICENSE" -delete
find "$OUT_WEB" -maxdepth 1 -type f -name 'alpine-v*.LICENSE.md' ! -name "alpine-v${ALPINE_VERSION}.LICENSE.md" -delete

echo "done: $THIRD_PARTY_DIR updated"
