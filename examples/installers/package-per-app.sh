#!/usr/bin/env bash
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

# Package every example and test as its own zip, one per OS/arch, for
# distribution through the Cloudflare R2 bucket served at downloads.pallier.org.
#
# Reads the staging directories left behind by:
#   KEEP_STAGE=1 bash examples/installers/build-all-platforms.sh
#
# Run from the repo root OR from examples/installers/:
#   bash examples/installers/package-per-app.sh
#
# Only the individual apps go to R2. The four whole-collection archives are
# served from the GitHub release instead -- they are the same bytes, GitHub
# keeps them indefinitely, and mirroring them here would double the bucket.
#
# Outputs (relative to the repo root):
#   _build/r2/Windows_x86_64/<app>.zip     containing <app>.exe
#   _build/r2/MacOS_arm64/<app>.zip        containing <app>.app/ (a bundle)
#   _build/r2/Linux_x86_64/<app>.zip       containing <app>
#   _build/r2/Linux_arm64/<app>.zip        containing <app>
#
# Each zip holds exactly one top-level entry, so unzipping it yields the app
# itself rather than a wrapper directory.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES_DIR="${SCRIPT_DIR%/installers}"
REPO_ROOT="$(cd "${EXAMPLES_DIR}/.." && pwd)"
OUT_DIR="${R2_DIR:-${REPO_ROOT}/_build/r2}"

# stage directory : R2 folder name
TARGETS=(
  "win-stage:Windows_x86_64"
  "mac-stage:MacOS_arm64"
  "x86-stage:Linux_x86_64"
  "arm64-stage:Linux_arm64"
)

command -v zip >/dev/null 2>&1 || { echo "error: 'zip' is not installed" >&2; exit 1; }

# --- helpers -----------------------------------------------------------------

# app_name strips the platform-specific suffix from a staged entry:
#   Stroop_task.exe -> Stroop_task     Stroop_task.app -> Stroop_task
app_name() {
  local entry="$1"
  entry="${entry%.exe}"
  entry="${entry%.app}"
  echo "$entry"
}

# package_stage zips every app in one staging directory into one R2 folder.
# Examples sit at the stage root and tests in its tests/ subdirectory; both
# flatten into the same R2 folder, so a name used by both is a hard error
# rather than a silent overwrite.
package_stage() {
  local stage="$1" osarch="$2"
  local dest="${OUT_DIR}/${osarch}"
  mkdir -p "${dest}"

  local n=0
  local sub src path entry name zip
  for sub in "" "tests"; do
    src="${stage}${sub:+/$sub}"
    [[ -d "${src}" ]] || continue
    for path in "${src}"/*; do
      [[ -e "${path}" ]] || continue
      entry="$(basename "${path}")"
      # The tests/ subdirectory is walked separately, not packaged as an app.
      [[ -z "${sub}" && "${entry}" == "tests" ]] && continue
      name="$(app_name "${entry}")"
      zip="${dest}/${name}.zip"
      if [[ -e "${zip}" ]]; then
        echo "error: ${osarch}/${name}.zip already exists -- the name '${name}'" >&2
        echo "       is used by both an example and a test. Rename one of them." >&2
        exit 1
      fi
      # -X drops extra file attributes; -y keeps symlinks inside .app bundles.
      (cd "${src}" && zip -q -r -X -y "${zip}" "${entry}")
      n=$((n + 1))
    done
  done
  echo "  ${osarch}: ${n} app(s)"
}

# --- main --------------------------------------------------------------------

rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}"

echo "=== Packaging each app individually into ${OUT_DIR} ==="
for target in "${TARGETS[@]}"; do
  stage_name="${target%%:*}"
  osarch="${target##*:}"
  stage="${SCRIPT_DIR}/${stage_name}"
  if [[ ! -d "${stage}" ]]; then
    echo "error: staging directory ${stage} not found." >&2
    echo "       Run: KEEP_STAGE=1 bash examples/installers/build-all-platforms.sh" >&2
    exit 1
  fi
  package_stage "${stage}" "${osarch}"
done

echo ""
echo "Done. Total size:"
du -sh "${OUT_DIR}"
