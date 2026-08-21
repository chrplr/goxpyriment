#!/usr/bin/env bash
# Build the whole documentation as one PDF, each docs/ page a chapter.
#
# The output lands in docs/ beside the per-page PDFs, so Zensical copies it into
# the published site and docs/index.md can link to it. Like those, it is tracked
# in git: rebuild and commit it when the Markdown changes.
#
# Run from the repo root via: make book
#
# Requirements are the same as make_pdfs.sh: pandoc, latexmk, xelatex, DejaVu.
#   Ubuntu/Debian: sudo apt install pandoc latexmk texlive-xetex fonts-dejavu
#   macOS:         brew install pandoc && brew install --cask mactex
#
# cmd/gen-book assembles _build/book.md first: it takes the chapter order from
# zensical.toml's nav, gives every heading a file-scoped identifier, and
# rewrites cross-page links into internal ones. See its comment for why each of
# those is necessary.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

OUT="${1:-docs/goxpyriment-docs.pdf}"

echo "Assembling chapters ..."
go run ./cmd/gen-book/

# Mostly make_pdfs.sh's options. The differences are the ones that make a book
# out of a document: report class so pages become numbered chapters, a deeper
# table of contents to navigate 27 of them, and --resource-path so images still
# resolve now that the source lives in _build/.
echo "Typesetting $OUT ..."
pandoc _build/book.md \
  --from=markdown+gfm_auto_identifiers+autolink_bare_uris \
  --pdf-engine=latexmk \
  --pdf-engine-opt=-xelatex \
  --pdf-engine-opt=-interaction=nonstopmode \
  --pdf-engine-opt=-halt-on-error \
  --top-level-division=chapter \
  --toc \
  --toc-depth=3 \
  --number-sections \
  --resource-path="docs:_build" \
  -V documentclass=report \
  -V geometry:margin=25mm \
  -V colorlinks=true \
  -V linkcolor=blue \
  -V urlcolor=blue \
  -V toccolor=black \
  --highlight-style=tango \
  -V fontsize=11pt \
  -V mainfont="DejaVu Serif" \
  -V monofont="DejaVu Sans Mono" \
  -V title="goxpyriment" \
  -V subtitle="Complete Documentation" \
  -V author="Christophe Pallier" \
  --include-in-header=docs/unicode-fixes.tex \
  -o "$OUT"

echo "  ✓ $OUT  ($(du -h "$OUT" | cut -f1))"
