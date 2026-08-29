#!/usr/bin/env bash
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

# Build a browser (WebAssembly) bundle for every example that can run in one,
# laid out for publication through the Cloudflare R2 bucket served at
# downloads.pallier.org.
#
# Run from the repo root OR from examples/installers/:
#   bash examples/installers/build-wasm-apps.sh
#
# Outputs (relative to the repo root):
#   _build/r2/wasm/_runtime/{sdl.js,sdl.wasm,wasm_exec.js}   shared, ~5.3 MB
#   _build/r2/wasm/<app>/index.html                          launcher page
#   _build/r2/wasm/<app>/main.wasm                           the experiment
#
# The bundler emits all five files for every app, but three of them are
# byte-identical everywhere (sdl.js and sdl.wasm are //go:embed constants in
# wasmsdl; wasm_exec.js comes from GOROOT). Keeping one copy and pointing every
# page at it with Emscripten's Module.locateFile saves ~5.3 MB per app -- over
# 400 MB across the collection.
#
# Examples listed in wasm-skip.txt are not bundled; see that file for why.
#
# Optional environment:
#   ONLY=<name>    build only that example, for quick local checks
#   KEEP_GOING=0   stop at the first failure (default: build them all, then
#                  report every failure at the end)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES_DIR="${SCRIPT_DIR%/installers}"
REPO_ROOT="$(cd "${EXAMPLES_DIR}/.." && pwd)"
OUT_DIR="${R2_DIR:-${REPO_ROOT}/_build/r2}/wasm"
RUNTIME_DIR="${OUT_DIR}/_runtime"
SKIP_FILE="${SCRIPT_DIR}/wasm-skip.txt"

ONLY="${ONLY:-}"
KEEP_GOING="${KEEP_GOING:-1}"

# The three files that are identical in every bundle.
SHARED=(sdl.js sdl.wasm wasm_exec.js)

# wasmsdl lives in the pinned go-sdl3 fork. `go mod vendor` prunes cmd/, so it
# is resolved through the workspace rather than vendor/ -- which is also why no
# -mod flag may be passed here (workspace mode rejects -mod=mod).
WASMSDL=(go run github.com/Zyko0/go-sdl3/cmd/wasmsdl)

cd "${REPO_ROOT}"

# --- eligibility -------------------------------------------------------------

# skipped_apps prints the names listed in wasm-skip.txt, comments stripped.
skipped_apps() {
  [[ -f "${SKIP_FILE}" ]] || return 0
  sed 's/#.*//' "${SKIP_FILE}" | tr -d '[:blank:]' | grep -v '^$' || true
}

mapfile -t SKIP_LIST < <(skipped_apps)

is_skipped() {
  local name="$1" s
  [[ ${#SKIP_LIST[@]} -eq 0 ]] && return 1
  for s in "${SKIP_LIST[@]}"; do
    [[ "$name" == "$s" ]] && return 0
  done
  return 1
}

# eligible_apps prints every example directory that has a main.go and is not
# skipped. Mirrors example_dirs() in build-all-platforms.sh.
eligible_apps() {
  local dir name
  for dir in "${EXAMPLES_DIR}"/*/; do
    name="$(basename "$dir")"
    [[ -f "${dir}/main.go" ]] || continue
    is_skipped "$name" && continue
    [[ -n "${ONLY}" && "$name" != "${ONLY}" ]] && continue
    echo "$name"
  done
}

# --- build -------------------------------------------------------------------

rm -rf "${OUT_DIR}"
mkdir -p "${RUNTIME_DIR}"

declare -a FAILED=()
built=0

mapfile -t APPS < <(eligible_apps)
if [[ ${#APPS[@]} -eq 0 ]]; then
  echo "error: no eligible examples${ONLY:+ matching ONLY=${ONLY}}" >&2
  exit 1
fi

echo "=== Building ${#APPS[@]} browser bundle(s) into ${OUT_DIR} ==="

for name in "${APPS[@]}"; do
  dest="${OUT_DIR}/${name}"
  stage="$(mktemp -d)"

  # An example may ship its own launcher page (Memory_span, Reading-1). The
  # generator takes it as a base so those keep their bespoke instructions,
  # while still being rewritten to load the shared runtime.
  custom_page="${EXAMPLES_DIR}/${name}/web/index.html"
  page=()
  [[ -f "${custom_page}" ]] && page=(-page "${custom_page}")

  if ! "${WASMSDL[@]}" build -out "${stage}" "./examples/${name}" \
       >"${stage}/.log" 2>&1; then
    echo "  FAIL  ${name}"
    sed 's/^/          /' "${stage}/.log" | tail -5 >&2
    FAILED+=("${name}")
    rm -rf "${stage}"
    [[ "${KEEP_GOING}" == "1" ]] && continue
    exit 1
  fi

  mkdir -p "${dest}"
  mv "${stage}/main.wasm" "${dest}/main.wasm"

  # The shared runtime is written once, from whichever app builds first; every
  # later bundle's copies are identical and discarded.
  for f in "${SHARED[@]}"; do
    if [[ ! -f "${RUNTIME_DIR}/${f}" ]]; then
      mv "${stage}/${f}" "${RUNTIME_DIR}/${f}"
    fi
  done

  # Launcher page: the example's own if it has one, else generated from its
  # meta.yaml. Either way it must load the shared runtime, not a local copy.
  go run ./cmd/gen-wasm-launcher \
    -app "${name}" \
    "${page[@]}" \
    -out "${dest}/index.html"

  rm -rf "${stage}"
  built=$((built + 1))
  printf '  %-46s %s\n' "${name}" "$(du -h "${dest}/main.wasm" | cut -f1)"
done

# --- report ------------------------------------------------------------------

echo ""
echo "Built ${built} bundle(s); shared runtime $(du -sh "${RUNTIME_DIR}" | cut -f1)."
echo "Total: $(du -sh "${OUT_DIR}" | cut -f1)"

if [[ ${#FAILED[@]} -gt 0 ]]; then
  echo ""
  echo "${#FAILED[@]} example(s) failed to build for the browser:" >&2
  printf '  - %s\n' "${FAILED[@]}" >&2
  echo "" >&2
  echo "Fix them, or add them to ${SKIP_FILE#"${REPO_ROOT}"/} with a reason." >&2
  exit 1
fi
