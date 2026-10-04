#!/usr/bin/env bash
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

# Build a JATOS study archive (.jzip) for one example, ready for Import Study in
# the JATOS GUI (https://www.jatos.org). Under JATOS the results are sent to the
# server instead of being downloaded; see "Running on JATOS" in docs/WASM.md.
#
# Run from the repo root OR from examples/installers/:
#   bash examples/installers/build-jatos-study.sh Stroop_task
# or:
#   make jatos-Stroop_task
#
# Output (relative to the repo root):
#   _build/jatos/<app>.jzip      the study archive to import
#   _build/jatos/<app>/          the unpacked bundle it was made from
#
# The bundle is self-contained: unlike the R2 publication (build-wasm-apps.sh),
# sdl.js, sdl.wasm and wasm_exec.js sit next to the page, because each JATOS
# study has its own assets folder. That costs ~5.3 MB per study.
#
# The launcher is always the generated one (gen-wasm-launcher -jatos), even for
# an example that ships its own web/index.html: hand-written pages are not
# adapted to JATOS.

set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <example-name>" >&2
  exit 2
fi
name="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES_DIR="${SCRIPT_DIR%/installers}"
REPO_ROOT="$(cd "${EXAMPLES_DIR}/.." && pwd)"
OUT_DIR="${REPO_ROOT}/_build/jatos"
SKIP_FILE="${SCRIPT_DIR}/wasm-skip.txt"

cd "${REPO_ROOT}"

if [[ ! -f "examples/${name}/main.go" ]]; then
  echo "error: examples/${name}/main.go not found" >&2
  exit 1
fi
if [[ -f "${SKIP_FILE}" ]] && sed 's/#.*//' "${SKIP_FILE}" | tr -d '[:blank:]' | grep -qx "${name}"; then
  echo "error: ${name} is listed in ${SKIP_FILE#"${REPO_ROOT}"/} and cannot run in a browser" >&2
  exit 1
fi

dest="${OUT_DIR}/${name}"
rm -rf "${dest}" "${OUT_DIR}/${name}.jzip"
mkdir -p "${dest}"

# wasmsdl lives in the pinned go-sdl3 fork and is resolved through the
# workspace (see build-wasm-apps.sh for why no -mod flag is passed).
go run github.com/Zyko0/go-sdl3/cmd/wasmsdl build -out "${dest}" "./examples/${name}"

# Replace wasmsdl's bare-canvas page with the JATOS launcher.
go run ./cmd/gen-wasm-launcher -app "${name}" -jatos -out "${dest}/index.html"

go run ./cmd/gen-jatos-jzip -app "${name}" -bundle "${dest}" -out "${OUT_DIR}/${name}.jzip"

echo "Study archive: _build/jatos/${name}.jzip ($(du -h "${OUT_DIR}/${name}.jzip" | cut -f1))"
echo "Import it in JATOS (Studies > Import study), then create a study link."
