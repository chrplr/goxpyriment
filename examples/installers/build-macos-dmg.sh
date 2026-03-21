#!/usr/bin/env bash

# Build a macOS zip containing all goxpyriment examples as .app bundles.
# Run this script on macOS from the examples/installers directory:
#   cd examples/installers
#   bash build-macos-dmg.sh

set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
EXAMPLES_DIR="${SCRIPT_DIR%/installers}"
APPS_DIR="${SCRIPT_DIR}/GoxpyrimentExamples-apps"
ZIP_NAME="goxpyriment-examples.zip"

# Clean up previous builds
rm -rf "${APPS_DIR}"
rm -f "${SCRIPT_DIR}/${ZIP_NAME}"
mkdir -p "${APPS_DIR}"

echo "Building .app bundles into ${APPS_DIR} ..."

for dir in "${EXAMPLES_DIR}"/*/; do
  name="$(basename "${dir}")"
  
  # Skip non-example directories
  if [[ "${name}" == "assets" ]] || [[ "${name}" == "installers" ]] || [[ "${name}" == "goxpy_data" ]]; then
    continue
  fi
  if [[ ! -f "${dir}/main.go" ]]; then
    continue
  fi

  app="${APPS_DIR}/${name}.app"
  echo "  - ${name} -> ${app}"

  mkdir -p "${app}/Contents/MacOS"
  mkdir -p "${app}/Contents/Resources"

  # Minimal Info.plist
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

  # Add icon
  cp "${EXAMPLES_DIR}/../assets/icon.icns" "${app}/Contents/Resources/icon.icns"

  # OPTIMIZATION: Build with -ldflags="-s -w" to strip debug symbols and reduce size
  (cd "${dir}" && go build -ldflags="-s -w" -o "${app}/Contents/MacOS/${name}" .)

  # Copy local assets (if any) into Resources
  if [[ -d "${dir}/assets" ]]; then
    cp -R "${dir}/assets" "${app}/Contents/Resources/"
  fi

  # Ad-hoc sign to prevent macOS Gatekeeper "damaged" error
  codesign --force --deep --sign - "${app}"
done

echo "Creating zip ${ZIP_NAME} ..."

cd "${APPS_DIR}"
zip -r "${SCRIPT_DIR}/${ZIP_NAME}" .

echo "Done. Zip created at: ${SCRIPT_DIR}/${ZIP_NAME}"
