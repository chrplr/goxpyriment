.PHONY: all examples update-examples-gallery tests pdfs docs serve deploy clean help

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
	@echo "  run-NAME       Build and run a single example (e.g. make run-parity_decision)"
	@echo "  tests     Build test binaries"
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

# Build and run a single example: make run-hello_world
run-%:
	@(cd examples/$* && CGO_ENABLED=0 go run .)

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
#   make wasm-hello_world          build hello_world as WASM
#   make wasm-hello_world-serve    build + serve on http://localhost:8080
#
# Prerequisite: the Emscripten-compiled SDL3 bundle must exist at
#   examples/hello_world/SDL3.js and examples/hello_world/SDL3.wasm
# See docs/WASM.md for how to build it.

WASM_EXEC_JS := $(shell go env GOROOT)/lib/wasm/wasm_exec.js

wasm-%-serve: wasm-%
	cd examples/$* && python3 -m http.server 8080

wasm-%:
	GOOS=js GOARCH=wasm go build -o examples/$*/main.wasm ./examples/$*/
	@if [ ! -f examples/$*/wasm_exec.js ]; then \
	  cp $(WASM_EXEC_JS) examples/$*/wasm_exec.js; \
	fi
	@echo "Built examples/$*/main.wasm"
	@echo "Serve with: make wasm-$*-serve  (requires SDL3.js + SDL3.wasm in examples/$*/)"

