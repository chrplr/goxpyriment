#!/usr/bin/env bash
# share.sh — export one example as a self-contained, standalone Go module.
#
# Usage:
#   examples/share.sh <example-dir> [version] [outdir]
#
# Copies <example-dir> to <outdir>/<example-dir>/ (default outdir:
# _build/share/) and generates a go.mod/go.sum that require the *published*
# goxpyriment module instead of relying on this repo's shared examples/go.mod
# and go.work. The result builds on its own — a colleague can unzip it and run
# `go run .` without `go mod init`, a replace directive, or the workspace.
#
# The goxpyriment version defaults to the most recent git tag; pass one
# explicitly as the second argument to override.
#
# Note: `go mod tidy` (below) fetches the published library and its
# dependencies, so the first export of a given version needs network access
# (afterwards they are served from the local module cache).
set -euo pipefail

name="${1:-}"
if [ -z "$name" ]; then
  echo "usage: examples/share.sh <example-dir> [version] [outdir]" >&2
  exit 2
fi
name="${name%/}"                                        # tolerate a trailing slash

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"    # examples/
repo="$(cd "$here/.." && pwd)"                          # repo root
src="$here/$name"

if [ ! -f "$src/main.go" ]; then
  echo "error: '$name' is not an example directory (no $src/main.go)" >&2
  exit 1
fi

version="${2:-}"
if [ -z "$version" ]; then
  version="$(git -C "$repo" describe --tags --abbrev=0 2>/dev/null || true)"
fi
if [ -z "$version" ]; then
  echo "error: could not determine a goxpyriment version; pass one explicitly, e.g." >&2
  echo "       examples/share.sh $name v0.12.3" >&2
  exit 1
fi

out="${3:-$repo/_build/share}"
dest="$out/$name"

echo ">> exporting '$name' as a standalone module (goxpyriment $version)"
rm -rf "$dest"
mkdir -p "$dest"
cp -R "$src/." "$dest/"

# Drop anything that would confuse a fresh module: a stale go.mod/go.sum, or a
# compiled binary named after the directory.
rm -f "$dest/go.mod" "$dest/go.sum" "$dest/$name" "$dest/$name.exe"

# Warn if the example reads sibling files: it depends on content outside its own
# folder and so is not self-contained on its own (e.g. shared .gv assets).
if grep -Eq '"\.\./' "$dest"/*.go 2>/dev/null; then
  echo "!! warning: '$name' references sibling paths (\"../\") in its sources — it"
  echo "!!          depends on files outside its own folder and may not build or run"
  echo "!!          in isolation. Bundle the referenced files manually if needed."
fi

modname="$(printf '%s' "$name" | tr '[:upper:]' '[:lower:]')"
cat > "$dest/go.mod" <<EOF
module $modname

go 1.25

require github.com/chrplr/goxpyriment $version
EOF

echo ">> resolving dependencies (go mod tidy)…"
# GOWORK=off so this behaves like a fresh checkout on a colleague's machine
# (the repo's go.work must not pull the copy back into the workspace).
( cd "$dest" && GOWORK=off go mod tidy )

echo ">> done: $dest"
echo "   zip it:     (cd '$out' && zip -r '$name.zip' '$name')"
echo "   colleague:  unzip, then  cd '$name' && go run ."
