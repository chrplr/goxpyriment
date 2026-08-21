#!/usr/bin/env bash
# Generate PDF versions of the documentation using pandoc + latexmk (xelatex).
# Run this before building the Zensical site (or `make deploy`) to include the
# PDFs on the GitHub Pages site.
#
# Requirements: pandoc, latexmk, xelatex, DejaVu fonts
#   Ubuntu/Debian: sudo apt install pandoc latexmk texlive-xetex fonts-dejavu
#   macOS:         brew install pandoc && brew install --cask mactex
#                  (latexmk and DejaVu fonts are included with MacTeX)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# DejaVu fonts have broad Unicode coverage (Greek, math operators, check mark).
# unicode-fixes.tex is a safety net for any remaining missing characters.
# latexmk drives xelatex through as many passes as needed so the table of
# contents and intra-document hyperlinks (\ref/hyperref anchors) resolve
# instead of emitting "undefined reference" warnings from a single pass.
PANDOC_OPTS=(
  # GitHub-style header IDs keep section numbers (e.g. "6-timing-architecture")
  # so the hand-written tables of contents — and Zensical/python-markdown, which
  # uses the same scheme — resolve. Pandoc's default strips leading numbers,
  # which silently breaks every numbered TOC link.
  # autolink_bare_uris: a bare URL in the text becomes a real link, which
  # makes it clickable and, with xurl, breakable — otherwise a long DOI runs
  # off the page (it hid half the citation on the landing page).
  --from=markdown+gfm_auto_identifiers+autolink_bare_uris
  --pdf-engine=latexmk
  --pdf-engine-opt=-xelatex
  --toc
  --toc-depth=2
  -V geometry:margin=25mm
  -V colorlinks=true
  -V linkcolor=blue
  -V urlcolor=blue
  -V toccolor=black
  --highlight-style=tango
  -V fontsize=11pt
  -V mainfont="DejaVu Serif"
  -V monofont="DejaVu Sans Mono"
  --include-in-header=unicode-fixes.tex
)

cd "$SCRIPT_DIR"

echo "Generating PDFs in docs/ ..."

pandoc Installation.md "${PANDOC_OPTS[@]}" \
  -V title="goxpyriment — Installation" \
  -o Installation.pdf
echo "  ✓ Installation.pdf"

pandoc GettingStarted.md "${PANDOC_OPTS[@]}" \
  -V title="goxpyriment — Getting Started" \
  -o GettingStarted.pdf
echo "  ✓ GettingStarted.pdf"

pandoc UserManual.md "${PANDOC_OPTS[@]}" \
  -V title="goxpyriment — User Manual" \
  -o UserManual.pdf
echo "  ✓ UserManual.pdf"

pandoc MigrationGuide.md "${PANDOC_OPTS[@]}" \
  -V title="goxpyriment — Migration Guide" \
  -o MigrationGuide.pdf
echo "  ✓ MigrationGuide.pdf"

pandoc API.md "${PANDOC_OPTS[@]}" \
  -V title="goxpyriment — API Reference" \
  -o API.pdf
echo "  ✓ API.pdf"

pandoc TimingTests.md "${PANDOC_OPTS[@]}" \
  -V title="goxpyriment — Timing Tests" \
  -o TimingTests.pdf


echo "Done. Commit the PDFs and push — they will be published via GitHub Actions."
