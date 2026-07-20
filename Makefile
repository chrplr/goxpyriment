.PHONY: all examples update-examples-gallery readme tests pdfs docs serve deploy clean help

# Discover every directory holding a main.go directly under examples/ or
# tests/. We use make's built-in $(wildcard)/$(patsubst) rather than `find`
# (piped through `sort`) on purpose: on Windows with Git Bash, `find` and
# `sort` are frequently shadowed by the DOS find.exe / sort.exe on PATH, which
# have completely different semantics and break these shell-outs. The wildcard
# functions are pure make and have no such conflict. (Mirrors build-all.sh.)
EXAMPLES := $(patsubst %/main.go,%,$(wildcard examples/*/main.go))

TESTS := $(patsubst %/main.go,%,$(wildcard tests/*/main.go))


# ---------------------------------------------------------------------------
# Help
# ---------------------------------------------------------------------------

help:
	@echo "Available targets:"
	@echo "  help      Show this message"
	@echo "  all       Build all examples and tests to _build / (default)"
	@echo "  examples       Same as target "all"<Down>Build all examples to _build/"
	@echo "  update-examples-gallery  Regenerate docs/GalleryOfExamples.md tables from meta.yaml files"
	@echo "  readme    Regenerate README.md from docs/index.md (single source of truth)"
	@echo "  run-NAME       Build and run a single example (e.g. make run-parity_decision)"
	@echo "  share-NAME     Export one example as a standalone module to _build/share/NAME/"
	@echo "  tests     Build test binaries"
	@echo "  wasm-NAME        Build a browser (WASM) bundle of an example to _build/wasm/NAME/"
	@echo "  wasm-NAME-serve  Build + serve a browser bundle at http://localhost:8080/?s=1"
	@echo "  pdfs      Generate PDF docs via pandoc + xelatex"
	@echo "  docs      Build Zensical HTML site to site/"
	@echo "  serve     Live-reload docs preview at http://127.0.0.1:8000"
	@echo "  deploy    Generate PDFs and build docs (GitHub Actions pushes to Pages)"
	@echo "  clean     Remove _build/ and site/"



all: examples tests

# ---------------------------------------------------------------------------
# Examples
# ---------------------------------------------------------------------------

# Build all examples; binaries go to _build/
examples:
	@mkdir -p _build
	@for dir in $(EXAMPLES); do \
	  name=$$(basename $$dir); \
	  echo "Building $$name..."; \
	  (cd $$dir && CGO_ENABLED=0 go build -o "$(CURDIR)/_build/$$name" .); \
	done

# Regenerate docs/GalleryOfExamples.md tables from per-example meta.yaml files.
update-examples-gallery:
	@go run ./cmd/gen-gallery/

# Regenerate README.md from docs/index.md (single source of truth for the
# landing page). Rewrites relative links to reach files through docs/.
readme:
	@go run ./cmd/gen-readme/

# Build and run a single example: make run-hello_world
run-%:
	@(cd examples/$* && CGO_ENABLED=0 go run .)

# Export one example as a self-contained, standalone module under
# _build/share/NAME/ (generated go.mod requiring the published goxpyriment, so a
# colleague can build it without go.work/replace/`go mod init`).
#   make share-demo_hello_world              # uses the latest git tag
#   make share-demo_hello_world VERSION=v0.12.3
share-%:
	@bash examples/share.sh $* "$(VERSION)"

# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

# Build all tests; binaries go to _build/
tests:
	@mkdir -p _build
	@for dir in $(TESTS); do \
	  name=$$(basename $$dir); \
	  echo "Building $$name..."; \
	  (cd $$dir && CGO_ENABLED=0 go build -o "$(CURDIR)/_build/$$name" .); \
	done

# ---------------------------------------------------------------------------
# Documentation
# ---------------------------------------------------------------------------

# Generate PDF versions of the documentation.
# Requires: pandoc, xelatex  (sudo apt install pandoc texlive-xetex)
pdfs:
	bash docs/make_pdfs.sh

# Build the Zensical HTML site locally (output → site/).
docs:
	zensical build --clean

# Live-reload preview at http://127.0.0.1:8000
serve:
	zensical serve

# Generate PDFs and build docs locally.
# GitHub Actions (.github/workflows/docs.yml) handles the push to GitHub Pages.
deploy: pdfs docs

# ---------------------------------------------------------------------------
# Clean
# ---------------------------------------------------------------------------

clean:
	rm -rf _build/ site/

# ---------------------------------------------------------------------------
# WASM / browser builds
# ---------------------------------------------------------------------------
# Usage:
#   make wasm-parity_decision        build a self-contained browser bundle
#                                    into _build/wasm/parity_decision/
#   make wasm-parity_decision-serve  build + serve on http://localhost:8080
#                                    (open http://localhost:8080/?s=1)
#
# The wasmsdl bundler ships inside the pinned go-sdl3 fork (see the replace
# directive in go.mod), so `go run` resolves it from the module graph — no
# local clone or Emscripten install needed. It bundles main.wasm together
# with wasm_exec.js and the prebuilt SDL3 blob (sdl.js/sdl.wasm), and the
# serve command sends the COOP/COEP headers that give experiments the
# high-resolution (~5 µs) browser clock. See docs/WASM.md for details.

WASMSDL := go run github.com/Zyko0/go-sdl3/cmd/wasmsdl

wasm-%-serve:
	$(WASMSDL) serve ./examples/$*

wasm-%:
	$(WASMSDL) build -out _build/wasm/$* ./examples/$*
	@echo "Bundle in _build/wasm/$* — serve it with: make wasm-$*-serve"

