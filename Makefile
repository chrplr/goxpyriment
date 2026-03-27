.PHONY: all examples update-readme tests pdfs docs serve deploy clean help

EXAMPLES := $(shell find examples -maxdepth 2 -name main.go \
               | xargs -I{} dirname {} | sort)

# Default: build examples
all: examples

# ---------------------------------------------------------------------------
# Examples
# ---------------------------------------------------------------------------

# Build all examples; binaries go to examples/_build/
examples:
	@mkdir -p examples/_build
	@for dir in $(EXAMPLES); do \
	  name=$$(basename $$dir); \
	  echo "Building $$name..."; \
	  (cd $$dir && CGO_ENABLED=0 go build -o "$(CURDIR)/examples/_build/$$name" .); \
	done

# Regenerate the examples/README.md tables from per-example meta.yaml files.
update-readme:
	@cd examples && go run ./cmd/gen-readme/

# Build and run a single example: make run-hello_world
run-%:
	@(cd examples/$* && CGO_ENABLED=0 go run .)

# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

tests:
	bash tests/build.sh

# ---------------------------------------------------------------------------
# Documentation
# ---------------------------------------------------------------------------

# Generate PDF versions of the documentation.
# Requires: pandoc, xelatex  (sudo apt install pandoc texlive-xetex)
pdfs:
	bash docs/make_pdfs.sh

# Build the MkDocs HTML site locally (output → site/).
docs:
	mkdocs build

# Live-reload preview at http://127.0.0.1:8000
serve:
	mkdocs serve

# Generate PDFs, then build and push to GitHub Pages.
deploy: pdfs
	mkdocs gh-deploy

# ---------------------------------------------------------------------------
# Clean
# ---------------------------------------------------------------------------

clean:
	rm -rf examples/_build/ site/

# ---------------------------------------------------------------------------
# Help
# ---------------------------------------------------------------------------

help:
	@echo "Available targets:"
	@echo "  all       Build all examples (default)"
	@echo "  examples       Build all examples to examples/_build/"
	@echo "  update-readme  Regenerate examples/README.md tables from meta.yaml files"
	@echo "  run-NAME       Build and run a single example (e.g. make run-parity_decision)"
	@echo "  tests     Build test binaries"
	@echo "  pdfs      Generate PDF docs via pandoc + xelatex"
	@echo "  docs      Build MkDocs HTML site to site/"
	@echo "  serve     Live-reload docs preview at http://127.0.0.1:8000"
	@echo "  deploy    Generate PDFs and push docs to GitHub Pages"
	@echo "  clean     Remove examples/_build/ and site/"
	@echo "  help      Show this message"
