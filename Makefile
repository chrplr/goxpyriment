.PHONY: all examples tests pdfs docs serve deploy clean

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
