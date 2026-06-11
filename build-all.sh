#!/bin/bash
#
# build-all.sh — build every example and test into _build/
#
# This is a no-make equivalent of `make all` for users who do not have
# `make` installed (notably on Windows running this from Git Bash).
# It builds the same binaries into the same _build/ folder.
#
# Usage (from the repository root):
#   bash build-all.sh

set -e

# Resolve the repo root (directory of this script) so it works from anywhere.
ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$ROOT"

mkdir -p _build

# Build every directory that contains a main.go under examples/ and tests/.
find examples tests -maxdepth 2 -name main.go 2>/dev/null \
  | xargs -I{} dirname {} | sort | while read -r dir; do
    name=$(basename "$dir")
    echo "Building $name..."
    ( cd "$dir" && CGO_ENABLED=0 go build -o "$ROOT/_build/$name" . ) \
      || echo "  !! failed to build $name"
done

echo "Done. Binaries are in $ROOT/_build/"
