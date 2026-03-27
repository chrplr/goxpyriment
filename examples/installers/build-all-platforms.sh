#!/usr/bin/env bash
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Distributed under the GNU General Public License v3.

# Build all goxpyriment examples for Windows, macOS (arm64), and Linux (x86_64).
# Uses Go cross-compilation (CGO_ENABLED=0 required).
#
# Run from the repo root OR from examples/installers/:
#   bash examples/installers/build-all-platforms.sh
#
# Prerequisites:
#   - Go 1.25+ in PATH
#   - CGO_ENABLED=0  (set automatically here, override with env if needed)
#   - examples/installers/appimagetool  (for Linux AppImages)
#     Download: wget https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage -O examples/installers/appimagetool && chmod +x examples/installers/appimagetool
#   - libfuse2 installed (for appimagetool on Linux)
#
# Outputs (all in examples/installers/):
#   goxpyriment-examples-windows-x86_64.zip
#   goxpyriment-examples-macos-arm64.zip
#   goxpyriment-examples-linux-x86_64-appimages.tar.gz

set -euo pipefail

export CGO_ENABLED=0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES_DIR="${SCRIPT_DIR%/installers}"
OUT_DIR="${SCRIPT_DIR}"
ASSETS_DIR="${EXAMPLES_DIR}/../assets"

SKIP_DIRS=("assets" "installers" "xpd_results")

# --- helpers -----------------------------------------------------------------

is_skipped() {
  local name="$1"
  for s in "${SKIP_DIRS[@]}"; do
    [[ "$name" == "$s" ]] && return 0
  done
  return 1
}

# Enumerate example directories that have a main.go
example_dirs() {
  for dir in "${EXAMPLES_DIR}"/*/; do
    local name
    name="$(basename "$dir")"
    is_skipped "$name" && continue
    [[ -f "${dir}/main.go" ]] || continue
    echo "$dir"
  done
}

# =============================================================================
# 1. Windows x86_64 — zip of .exe files
# =============================================================================

echo "=== Building Windows x86_64 binaries ==="
WIN_STAGE="${OUT_DIR}/win-stage"
rm -rf "${WIN_STAGE}"
mkdir -p "${WIN_STAGE}"

while IFS= read -r dir; do
  name="$(basename "$dir")"
  echo "  ${name}.exe"
  GOOS=windows GOARCH=amd64 go build \
    -ldflags="-s -w -H windowsgui" \
    -o "${WIN_STAGE}/${name}.exe" \
    "${dir}"
done < <(example_dirs)

WIN_ZIP="${OUT_DIR}/goxpyriment-examples-windows-x86_64.zip"
rm -f "${WIN_ZIP}"
(cd "${WIN_STAGE}" && zip -q -r "${WIN_ZIP}" .)
echo "  -> ${WIN_ZIP}"

# =============================================================================
# 2. macOS arm64 — zip of .app bundles (unsigned; Gatekeeper note applies)
# =============================================================================

echo "=== Building macOS arm64 .app bundles ==="
MAC_STAGE="${OUT_DIR}/mac-stage"
rm -rf "${MAC_STAGE}"
mkdir -p "${MAC_STAGE}"

while IFS= read -r dir; do
  name="$(basename "$dir")"
  app="${MAC_STAGE}/${name}.app"
  echo "  ${name}.app"

  mkdir -p "${app}/Contents/MacOS" "${app}/Contents/Resources"

  cat > "${app}/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key>
  <string>${name}</string>
  <key>CFBundleIdentifier</key>
  <string>org.goxpyriment.${name}</string>
  <key>CFBundleVersion</key>
  <string>0.1.0</string>
  <key>CFBundleExecutable</key>
  <string>${name}</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleIconFile</key>
  <string>icon.icns</string>
</dict>
</plist>
EOF

  # Icon (optional — skip gracefully if absent)
  if [[ -f "${ASSETS_DIR}/icon.icns" ]]; then
    cp "${ASSETS_DIR}/icon.icns" "${app}/Contents/Resources/icon.icns"
  fi

  GOOS=darwin GOARCH=arm64 go build \
    -ldflags="-s -w" \
    -o "${app}/Contents/MacOS/${name}" \
    "${dir}"

  # Copy per-example assets if present
  if [[ -d "${dir}/assets" ]]; then
    cp -R "${dir}/assets" "${app}/Contents/Resources/"
  fi
done < <(example_dirs)

# Note: codesign is NOT run here — binaries are unsigned.
# macOS users must right-click → Open, or run:
#   xattr -dr com.apple.quarantine <AppName>.app

MAC_ZIP="${OUT_DIR}/goxpyriment-examples-macos-arm64.zip"
rm -f "${MAC_ZIP}"
(cd "${MAC_STAGE}" && zip -q -r "${MAC_ZIP}" .)
echo "  -> ${MAC_ZIP}"

# =============================================================================
# 3. Linux x86_64 — AppImages
# =============================================================================

echo "=== Building Linux x86_64 AppImages ==="
APPDIR_ROOT="${OUT_DIR}/AppImages"
TOOL="${OUT_DIR}/appimagetool"

if [[ ! -x "${TOOL}" ]]; then
  echo "ERROR: appimagetool not found at ${TOOL}"
  echo "Download it with:"
  echo "  wget https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage -O ${TOOL}"
  echo "  chmod +x ${TOOL}"
  exit 1
fi

rm -rf "${APPDIR_ROOT}"
mkdir -p "${APPDIR_ROOT}"

while IFS= read -r dir; do
  name="$(basename "$dir")"
  echo "  ${name}.AppImage"
  appdir="${APPDIR_ROOT}/${name}.AppDir"
  mkdir -p "${appdir}/usr/bin" "${appdir}/usr/share/applications"

  GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o "${appdir}/usr/bin/${name}" \
    "${dir}"
  chmod +x "${appdir}/usr/bin/${name}"

  cat > "${appdir}/AppRun" <<EOF
#!/bin/sh
exec "\${APPDIR}/usr/bin/${name}" "\$@"
EOF
  chmod +x "${appdir}/AppRun"

  desktop="${appdir}/usr/share/applications/${name}.desktop"
  cat > "${desktop}" <<EOF
[Desktop Entry]
Type=Application
Name=${name}
Exec=${name}
Icon=${name}
Categories=Education;
EOF
  cp "${desktop}" "${appdir}/${name}.desktop"

  if [[ -f "${ASSETS_DIR}/icon_256.png" ]]; then
    cp "${ASSETS_DIR}/icon_256.png" "${appdir}/${name}.png"
  fi

  ARCH=x86_64 "${TOOL}" "${appdir}" "${APPDIR_ROOT}/${name}.AppImage" 2>/dev/null
done < <(example_dirs)

LINUX_TARBALL="${OUT_DIR}/goxpyriment-examples-linux-x86_64-appimages.tar.gz"
rm -f "${LINUX_TARBALL}"
(cd "${APPDIR_ROOT}" && tar czf "${LINUX_TARBALL}" *.AppImage)
echo "  -> ${LINUX_TARBALL}"

# =============================================================================
# Cleanup staging directories
# =============================================================================
rm -rf "${WIN_STAGE}" "${MAC_STAGE}" "${APPDIR_ROOT}"

echo ""
echo "Done. Artifacts in ${OUT_DIR}:"
ls -lh \
  "${OUT_DIR}/goxpyriment-examples-windows-x86_64.zip" \
  "${OUT_DIR}/goxpyriment-examples-macos-arm64.zip" \
  "${OUT_DIR}/goxpyriment-examples-linux-x86_64-appimages.tar.gz"
