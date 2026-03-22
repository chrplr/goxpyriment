#!/usr/bin/env bash
# Generate PDF versions of the documentation using pandoc + xelatex.
# Run this before "mkdocs gh-deploy" to include PDFs on the GitHub Pages site.
#
# Requirements: pandoc, xelatex (texlive-xetex)
#   Ubuntu/Debian: sudo apt install pandoc texlive-xetex
#   macOS:         brew install pandoc && brew install --cask mactex

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

PANDOC_OPTS=(
  --pdf-engine=xelatex
  --toc
  --toc-depth=2
  -V geometry:margin=25mm
  -V colorlinks=true
  -V linkcolor=blue
  -V urlcolor=blue
  -V toccolor=black
  --highlight-style=tango
  -V fontsize=11pt
)

cd "$SCRIPT_DIR"

echo "Generating PDFs in docs/ ..."

pandoc GettingStarted.md "${PANDOC_OPTS[@]}" \
  -V title="goxpyriment — Getting Started" \
  -o GettingStarted.pdf
echo "  ✓ GettingStarted.pdf"

pandoc UserManual.md "${PANDOC_OPTS[@]}" \
  -V title="goxpyriment — User Manual" \
  -o UserManual.pdf
echo "  ✓ UserManual.pdf"

pandoc API.md "${PANDOC_OPTS[@]}" \
  -V title="goxpyriment — API Reference" \
  -o API.pdf
echo "  ✓ API.pdf"

echo "Done. Run 'mkdocs gh-deploy' to publish."
