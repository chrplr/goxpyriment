# Development workflow

## Code

```bash
# Build and run a single example (windowed, subject 1)
go run examples/parity_decision/main.go -w -s 1

# Build all examples and tests
make all

# Run unit tests
go test ./control/...
```

## Documentation

### Editing

- Edit markdown files in `docs/`.
- To regenerate the Gallery of Examples table from `examples/*/meta.yaml`:

```bash
make update-examples-gallery
```

### Local preview

```bash
make docs-serve   # live-reload at http://127.0.0.1:8000
```

This does **not** need to be committed — `site/` is gitignored.

### Before committing

If you edited any doc file, regenerate the PDFs:

```bash
make pdfs    # requires pandoc + xelatex
```

The PDFs (`docs/*.pdf`) **are** tracked in git and must be committed alongside
the markdown sources so that the GitHub Pages site can serve them as downloads.

> **Note:** you do **not** need to run `make docs` before committing.
> The HTML site is built automatically by GitHub Actions on every push to `main`.

### Typical docs commit sequence

```bash
# 1. Edit docs/...
make pdfs
git add docs/
git commit -m "docs: update ..."
git push
# GitHub Actions builds and deploys the site automatically.
```

## Releases

```bash
git tag -a vX.Y.Z -m "Version X.Y.Z"
git push origin vX.Y.Z
```

Then update the version number in the Zenodo citation in `docs/index.md` and `README.md`.
