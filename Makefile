# qbit-filematcher — build, assets, and cross-compile targets
#
# Run `make help` to list targets.
# Make is optional: assets/licenses can be driven with `go run` / `go generate`.
# Most build targets depend on `assets` so HTMX/Alpine/CSS are embedded.

# ---------------------------------------------------------------------------
# Tools
# ---------------------------------------------------------------------------
#
# Frontend vendor versions: scripts/versions.json (single source of truth).

# Optional app version stamped into the binary via -ldflags
VERSION ?= dev

STATIC_DIR := web/static
BIN_DIR    := bin
DIST_DIR   := dist
TOOLS_DIR  := .tools
CMD_PKG    := ./cmd/qbit-filematcher

GO       ?= go
GOFLAGS  ?=
MODULE   := $(shell $(GO) list -m)
LDFLAGS  ?= -s -w -X $(MODULE)/cli.Version=$(VERSION)

BIN_NATIVE := $(BIN_DIR)/qbit-filematcher

FETCHASSETS := $(GO) run ./scripts/fetchassets
GENLICENSES := $(GO) run ./scripts/genlicenses
GOLANGCI_LINT ?= golangci-lint

.PHONY: help all assets force-assets \
	build build-race \
	build-linux-amd64 build-linux-arm64 \
	build-windows-amd64 build-windows-arm64 build-windows-386 \
	build-darwin-amd64 build-darwin-arm64 \
	build-all \
	icon-pngs \
	check check-race test test-race fmt lint govulncheck \
	licenses licenses-check tidy \
	clean clean-bin clean-dist clean-assets clean-tools

# ---------------------------------------------------------------------------
# Help / defaults
# ---------------------------------------------------------------------------

## help: Show this help message
help:
	@echo "Targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | awk -F': ' '{printf "  %-22s %s\n", $$1, $$2}'

## all: Fetch frontend assets and build the native binary
all: build

# ---------------------------------------------------------------------------
# Frontend assets (embedded into the binary; not committed)
# ---------------------------------------------------------------------------

## assets: Download HTMX + Alpine, build Tailwind CSS, write assets.json (go run)
assets:
	$(FETCHASSETS)

## force-assets: Re-download/rebuild frontend assets even if files exist
force-assets:
	$(FETCHASSETS) -force

# ---------------------------------------------------------------------------
# Native build
# ---------------------------------------------------------------------------

## build: Build the single binary for the host OS/arch into bin/
build: assets
	mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_NATIVE) $(CMD_PKG)

## build-race: Build the native binary with the race detector enabled
build-race: assets
	mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -race -ldflags "$(LDFLAGS)" -o $(BIN_NATIVE) $(CMD_PKG)

# ---------------------------------------------------------------------------
# Cross-compile (requires assets; CGO is disabled for portable binaries)
# ---------------------------------------------------------------------------

## build-linux-amd64: Cross-compile Linux amd64 → dist/qbit-filematcher-linux-amd64
build-linux-amd64: assets
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(DIST_DIR)/qbit-filematcher-linux-amd64 $(CMD_PKG)

## build-linux-arm64: Cross-compile Linux arm64 → dist/qbit-filematcher-linux-arm64
build-linux-arm64: assets
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(DIST_DIR)/qbit-filematcher-linux-arm64 $(CMD_PKG)

## build-windows-amd64: Cross-compile Windows amd64 → dist/qbit-filematcher-windows-amd64.exe
build-windows-amd64: assets
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
		$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(DIST_DIR)/qbit-filematcher-windows-amd64.exe $(CMD_PKG)

## build-windows-arm64: Cross-compile Windows arm64 → dist/qbit-filematcher-windows-arm64.exe
build-windows-arm64: assets
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 \
		$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(DIST_DIR)/qbit-filematcher-windows-arm64.exe $(CMD_PKG)

## build-windows-386: Cross-compile Windows 386 → dist/qbit-filematcher-windows-386.exe
build-windows-386: assets
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=386 \
		$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(DIST_DIR)/qbit-filematcher-windows-386.exe $(CMD_PKG)

## build-darwin-amd64: Cross-compile macOS amd64 → dist/qbit-filematcher-darwin-amd64
build-darwin-amd64: assets
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
		$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(DIST_DIR)/qbit-filematcher-darwin-amd64 $(CMD_PKG)

## build-darwin-arm64: Cross-compile macOS arm64 → dist/qbit-filematcher-darwin-arm64
build-darwin-arm64: assets
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
		$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(DIST_DIR)/qbit-filematcher-darwin-arm64 $(CMD_PKG)

## build-all: Cross-compile all supported OS/arch combinations into dist/
build-all: build-linux-amd64 build-linux-arm64 \
	build-windows-amd64 build-windows-arm64 build-windows-386 \
	build-darwin-amd64 build-darwin-arm64

## icon-pngs: Rasterize web/icon.svg → assets/icon/icon-{512,1024}.png
icon-pngs:
	mkdir -p assets/icon
	rsvg-convert -w 512 -h 512 web/icon.svg -o assets/icon/icon-512.png
	rsvg-convert -w 1024 -h 1024 web/icon.svg -o assets/icon/icon-1024.png

# ---------------------------------------------------------------------------
# Quality / module maintenance
# ---------------------------------------------------------------------------

## check: Run all quality controls (lint → govulncheck → licenses-check → test)
check: lint govulncheck licenses-check test

## check-race: Same as check, with the race detector enabled for tests
check-race: lint govulncheck licenses-check test-race

## test: Run the full Go test suite
test:
	$(GO) test $(GOFLAGS) ./...

## test-race: Run tests with the race detector
test-race:
	$(GO) test $(GOFLAGS) -race ./...

## fmt: Format Go sources (gofumpt + goimports via golangci-lint)
fmt:
	@if ! command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then \
		echo "error: $(GOLANGCI_LINT) not found" >&2; \
		echo "Install it, then re-run:" >&2; \
		echo "  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest" >&2; \
		exit 1; \
	fi
	$(GOLANGCI_LINT) fmt ./...

## lint: Run golangci-lint (format check + linters; requires golangci-lint on PATH)
lint:
	@if ! command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then \
		echo "error: $(GOLANGCI_LINT) not found" >&2; \
		echo "Install it, then re-run:" >&2; \
		echo "  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest" >&2; \
		exit 1; \
	fi
	$(GOLANGCI_LINT) run ./...

## govulncheck: Scan dependencies for known vulns (go run; not a module dependency)
govulncheck:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest ./...

## licenses: Regenerate third_party/ (Go deps + HTMX/Alpine license texts)
licenses:
	$(GENLICENSES)

## licenses-check: Fail if third_party/ is out of date vs genlicenses
licenses-check:
	$(GENLICENSES) -check

## tidy: Sync go.mod / go.sum with go mod tidy
tidy:
	$(GO) mod tidy

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------

## clean: Remove binaries, dist/, Tailwind CLI cache, and downloaded static assets
clean: clean-bin clean-dist clean-tools clean-assets

## clean-bin: Remove built binaries under bin/
clean-bin:
	rm -rf $(BIN_DIR)

## clean-dist: Remove release artifacts under dist/
clean-dist:
	rm -rf $(DIST_DIR)

## clean-tools: Remove downloaded Tailwind standalone CLI under .tools/
clean-tools:
	rm -rf $(TOOLS_DIR)

## clean-assets: Remove downloaded/built web/static files (keeps .keep)
clean-assets:
	find $(STATIC_DIR) -type f ! -name '.keep' -delete
