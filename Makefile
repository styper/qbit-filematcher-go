# qbit-filematcher — build, assets, and cross-compile targets
#
# Run `make help` to list targets.
# Most build targets depend on `assets` so HTMX/Alpine/CSS are embedded.

# ---------------------------------------------------------------------------
# Versions / tools
# ---------------------------------------------------------------------------

# Frontend vendor versions (downloaded; never committed)
HTMX_VERSION     ?= 2.0.4
ALPINE_VERSION   ?= 3.14.8
TAILWIND_VERSION ?= v4.1.11

# Optional app version stamped into the binary via -ldflags
VERSION ?= dev

STATIC_DIR := web/static
STYLES_DIR := web/styles
TOOLS_DIR  := .tools
BIN_DIR    := bin
DIST_DIR   := dist
CMD_PKG    := ./cmd/qbit-filematcher

GO       ?= go
GOFLAGS  ?=
MODULE   := $(shell $(GO) list -m)
LDFLAGS  ?= -s -w -X $(MODULE)/cli.Version=$(VERSION)

# Native binary name (no extension on Unix; .exe added for Windows cross builds)
BIN_NATIVE := $(BIN_DIR)/qbit-filematcher

# Detect host OS/arch for the Tailwind standalone CLI download
UNAME_S := $(shell uname -s | tr '[:upper:]' '[:lower:]')
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_M),x86_64)
  TW_ARCH := x64
else ifeq ($(UNAME_M),amd64)
  TW_ARCH := x64
else ifeq ($(UNAME_M),arm64)
  TW_ARCH := arm64
else ifeq ($(UNAME_M),aarch64)
  TW_ARCH := arm64
else
  TW_ARCH := $(UNAME_M)
endif

ifeq ($(UNAME_S),darwin)
  TW_OS := macos
else ifeq ($(UNAME_S),linux)
  TW_OS := linux
else
  TW_OS := $(UNAME_S)
endif

TAILWIND_BIN := $(TOOLS_DIR)/tailwindcss-$(TW_OS)-$(TW_ARCH)
TAILWIND_URL := https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/tailwindcss-$(TW_OS)-$(TW_ARCH)

.PHONY: help all assets force-assets \
	build build-race \
	build-linux-amd64 build-linux-arm64 \
	build-windows-amd64 build-windows-arm64 build-windows-386 \
	build-darwin-amd64 build-darwin-arm64 \
	build-all \
	icon-pngs \
	check check-race test test-race fmt fmt-check vet gosec govulncheck tidy \
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

## assets: Download HTMX + Alpine, build Tailwind CSS, and write assets.json hashes
assets: $(STATIC_DIR)/assets.json

## force-assets: Re-download/rebuild frontend assets even if files exist
force-assets:
	rm -f $(STATIC_DIR)/htmx.min.js $(STATIC_DIR)/alpine.min.js $(STATIC_DIR)/app.css $(STATIC_DIR)/assets.json
	$(MAKE) assets

$(STATIC_DIR):
	mkdir -p $(STATIC_DIR)

$(TOOLS_DIR):
	mkdir -p $(TOOLS_DIR)

$(STATIC_DIR)/htmx.min.js: | $(STATIC_DIR)
	curl -fsSL -A "qbit-filematcher-go" -o $@ \
		"https://cdn.jsdelivr.net/npm/htmx.org@$(HTMX_VERSION)/dist/htmx.min.js"

$(STATIC_DIR)/alpine.min.js: | $(STATIC_DIR)
	curl -fsSL -A "qbit-filematcher-go" -o $@ \
		"https://cdn.jsdelivr.net/npm/alpinejs@$(ALPINE_VERSION)/dist/cdn.min.js"

$(TAILWIND_BIN): | $(TOOLS_DIR)
	curl -fsSL -A "qbit-filematcher-go" -L -o $@ "$(TAILWIND_URL)"
	chmod +x $@

$(STATIC_DIR)/app.css: $(STYLES_DIR)/input.css $(TAILWIND_BIN) $(wildcard web/templates/*.html) | $(STATIC_DIR)
	$(TAILWIND_BIN) -i $(STYLES_DIR)/input.css -o $@ --minify

$(STATIC_DIR)/assets.json: $(STATIC_DIR)/htmx.min.js $(STATIC_DIR)/alpine.min.js $(STATIC_DIR)/app.css
	$(GO) run ./scripts/hashassets -dir $(STATIC_DIR) -o $@

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
# Artifacts land in dist/ for release packaging.
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

## check: Run all quality controls (fmt-check → vet → gosec → govulncheck → test)
check: fmt-check vet gosec govulncheck test

## check-race: Same as check, with the race detector enabled for tests
check-race: fmt-check vet gosec govulncheck test-race

## test: Run the full Go test suite
test:
	$(GO) test $(GOFLAGS) ./...

## test-race: Run tests with the race detector
test-race:
	$(GO) test $(GOFLAGS) -race ./...

## fmt: Format all Go sources with go fmt
fmt:
	$(GO) fmt ./...

## fmt-check: Fail if any Go file needs gofmt (CI-safe)
fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; \
		echo "$$unformatted"; \
		echo "Run: make fmt"; \
		exit 1; \
	fi

## vet: Run go vet on the module
vet:
	$(GO) vet ./...

## gosec: Static security analysis (go run; not a module dependency)
gosec:
	$(GO) run github.com/securego/gosec/v2/cmd/gosec@latest \
		-exclude=G304,G301,G302,G306,G703,G710 \
		./...

## govulncheck: Scan dependencies for known vulns (go run; not a module dependency)
govulncheck:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest ./...

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
