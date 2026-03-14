#!/usr/bin/env bash

# Build AppImages for all goxpyriment examples (Linux).
# This script is intended to be run locally on a Linux machine with:
#   - Go installed
#   - FUSE available (to run appimagetool itself)
#
# Usage:
#   cd examples/installers
#   bash build-linux-appimages.sh

set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
EXAMPLES_DIR="${SCRIPT_DIR%/installers}"
APPIMAGES_ROOT="${SCRIPT_DIR}/AppImages"
APPIMAGETOOL="${SCRIPT_DIR}/appimagetool"

mkdir -p "${APPIMAGES_ROOT}"

echo "Building all examples..."
(
  cd "${EXAMPLES_DIR}"
  bash build.sh
)

if [[ ! -x "${APPIMAGETOOL}" ]]; then
  echo "Downloading appimagetool..."
  wget -q https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage -O "${APPIMAGETOOL}"
  chmod +x "${APPIMAGETOOL}"
fi

echo "Packaging examples as AppImages into ${APPIMAGES_ROOT} ..."

cd "${EXAMPLES_DIR}"

for dir in */; do
  name="${dir%/}"
  # Skip non-example directories
  if [[ "${name}" == "assets" || "${name}" == "installers" || "${name}" == "xpd_results" ]]; then
    continue
  fi
  if [[ ! -f "${dir}/main.go" ]]; then
    continue
  fi

  echo "  - ${name}"

  appdir="${APPIMAGES_ROOT}/${name}.AppDir"
  mkdir -p "${appdir}/usr/bin" "${appdir}/usr/share/applications"

  # Binary: reuse the one built by build.sh if present, otherwise build just this example.
  if [[ -f "${dir%/}/${name}" ]]; then
    cp "${dir%/}/${name}" "${appdir}/usr/bin/${name}"
  else
    echo "    (rebuilding ${name})"
    (cd "${dir}" && go build -o "../${appdir}/usr/bin/${name}" .)
  fi

  # AppRun
  echo '#!/bin/sh' > "${appdir}/AppRun"
  echo "exec ./usr/bin/${name} \"\$@\"" >> "${appdir}/AppRun"
  chmod +x "${appdir}/AppRun"

  # .desktop file
  desktop_dir="${appdir}/usr/share/applications"
  desktop="${desktop_dir}/${name}.desktop"
  mkdir -p "${desktop_dir}"
  {
    echo "[Desktop Entry]"
    echo "Type=Application"
    echo "Name=${name}"
    echo "Exec=${name}"
    echo "Icon=application-default-icon"
    echo "Categories=Education;"
  } > "${desktop}"
  # AppImageKit expects a .desktop file at the root of the AppDir as well.
  cp "${desktop}" "${appdir}/${name}.desktop"

  # Provide a dummy icon so appimagetool is satisfied.
  touch "${appdir}/application-default-icon.png"

  # Build the AppImage
  "${APPIMAGETOOL}" "${appdir}" "${APPIMAGES_ROOT}/${name}.AppImage"
done

echo "Done. AppImages are in: ${APPIMAGES_ROOT}"

