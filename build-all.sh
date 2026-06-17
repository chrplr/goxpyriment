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

# Build every directory that contains a main.go directly under examples/ or
# tests/. We use shell globbing rather than `find` on purpose: on Windows with
# Git Bash, `find` (and `sort`) are frequently shadowed by the DOS find.exe /
# sort.exe on PATH, which have completely different semantics and break this
# script. Globbing is a bash builtin and has no such conflict.
for dir in examples/*/ tests/*/; do
    [ -f "${dir}main.go" ] || continue
    name=${dir%/}; name=${name##*/}
    echo "Building $name..."
    ( cd "$dir" && CGO_ENABLED=0 go build -o "$ROOT/_build/$name" . ) \
      || echo "  !! failed to build $name"
done

echo "Done. Binaries are in $ROOT/_build/"
