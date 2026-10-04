---
name: docs
description: Edit, build, preview or regenerate the goxpyriment documentation (docs/*.md, README.md, the Zensical site, the tracked PDFs and the single-file book). Use when touching anything under docs/ or README.md.
allowed-tools: Bash, Read, Edit, Write
---

# Documentation

All user-facing documentation lives in `docs/`:

| File | Contents |
|---|---|
| `docs/index.md` | Landing page — source of the generated `README.md` |
| `docs/GettingStarted.md` | Tutorial introduction — Python/Expyriment mapping, 3 worked examples |
| `docs/MigrationGuide.md` | Migration reference — concept maps and side-by-side code for Expyriment, PsychoPy, Psychtoolbox |
| `docs/ComparisonWithPsychoPy.md` | Feature-by-feature comparison with PsychoPy — parity, gaps, and how to choose |
| `docs/UserManual.md` | Concept guide — rendering model, timing, input, data, streams, audio, design |
| `docs/API.md` | Complete public API reference organized by package |
| `docs/GalleryOfExamples.md` | Generated tables of examples/tests (`make update-examples-gallery`) |
| `docs/WASM.md` | Browser build status and commands |

## Generated files — never edit by hand

- `README.md` is generated from `docs/index.md` by `cmd/gen-readme`
  (copies the content, rewrites relative links to go through `docs/`). Edit
  `docs/index.md`, then `make readme`. CI (`readme-sync.yml`) fails on drift.
- The tables in `docs/GalleryOfExamples.md` and the `<!-- BEGIN:links -->`
  blocks in `examples/*/README.md` come from `make update-examples-gallery`.
  CI (`gallery-sync.yml`) fails on drift.

## Site and PDFs

The site is built with [Zensical](https://zensical.org/). Makefile targets at
the repo root:

```bash
pip install -r docs/requirements.txt   # install Zensical once

make pdfs      # docs/*.pdf via pandoc + xelatex (one per page)
make book      # docs/goxpyriment-docs.pdf — the whole documentation as ONE PDF
               # (cmd/gen-book takes chapter order from zensical.toml's nav)
make serve     # live-reload preview at http://127.0.0.1:8000
make docs      # static HTML → site/ (zensical build --clean)
make deploy    # PDFs + site locally
make clean     # remove _build/ and site/
```

The generated `docs/*.pdf` files **are tracked in git** — when the underlying
Markdown changes, regenerate with `make pdfs` and `make book` and commit the
result. `site/` is gitignored; GitHub Actions (`docs.yml`) builds and deploys
it to Pages on push to main.
