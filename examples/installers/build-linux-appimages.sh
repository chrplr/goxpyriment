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
  if [[ "${name}" == "assets" || "${name}" == "installers" || "${name}" == "goxpy_data" ]]; then
    continue
  fi
  if [[ ! -f "${dir}/main.go" ]]; then
    continue
  fi

  echo "  - ${name}"

  appdir="${APPIMAGES_ROOT}/${name}.AppDir"
  mkdir -p "${appdir}/usr/bin" "${appdir}/usr/share/applications"

  # Binary: reuse the one built by build.sh if present, otherwise build just this example.
  BINARY_SOURCE_PATH="${dir%/}/${name}"
  TARGET_BINARY_PATH="${appdir}/usr/bin/${name}"

  if [[ -f "$BINARY_SOURCE_PATH" ]]; then
    cp "$BINARY_SOURCE_PATH" "$TARGET_BINARY_PATH"
  else
    echo "    (rebuilding ${name})"
    (cd "${dir}" && go build -o "$TARGET_BINARY_PATH" .)
  fi

  # Check if binary was created and is executable
  if [[ ! -f "$TARGET_BINARY_PATH" ]]; then
    echo "Error: Binary ${TARGET_BINARY_PATH} was not created for ${name}. Skipping."
    continue # Skip this example
  fi
  if [[ ! -x "$TARGET_BINARY_PATH" ]]; then
    echo "Warning: Binary ${TARGET_BINARY_PATH} is not executable for ${name}. Attempting to make it executable."
    chmod +x "$TARGET_BINARY_PATH"
    if [[ ! -x "$TARGET_BINARY_PATH" ]]; then
      echo "Error: Failed to make binary ${TARGET_BINARY_PATH} executable for ${name}. Skipping."
      continue # Skip this example
    fi
  fi

  # Generate AppRun script
  cat <<EOF > "${appdir}/AppRun"
#!/bin/sh
# APPDIR is the path where the AppImage is mounted.
EXECUTABLE="\${APPDIR}/usr/bin/${name}"
echo "Attempting to execute: \$EXECUTABLE" # Debugging line
exec "\$EXECUTABLE" "\$@"
EOF
  chmod +x "${appdir}/AppRun"

  # --- FIXED .desktop file ---
  desktop_dir="${appdir}/usr/share/applications"
  desktop="${desktop_dir}/${name}.desktop"
  mkdir -p "${desktop_dir}"
  {
    echo "[Desktop Entry]"
    echo "Type=Application"
    echo "Name=${name}"
    # Exec should just be the name of the executable for desktop integration
    echo "Exec=${name}"
    # This refers to the icon filename without the extension
    echo "Icon=${name}"
    echo "Categories=Education;"
  } > "${desktop}"
  
  # AppImageKit expects a .desktop file and an icon at the root of the AppDir.
  cp "${desktop}" "${appdir}/${name}.desktop"

  # Provide a dummy icon and name it after the app so the 'Icon=' entry works.
  cp ../assets/icon_256.png "${appdir}/${name}.png"

  # Build the AppImage
  "${APPIMAGETOOL}" "${appdir}" "${APPIMAGES_ROOT}/${name}.AppImage"
done

echo "Done. AppImages are in: ${APPIMAGES_ROOT}"
