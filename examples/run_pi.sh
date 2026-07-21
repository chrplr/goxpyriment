#!/bin/bash
# Raspberry Pi fullscreen workaround.
# SDL3 exclusive fullscreen does not render correctly under the Pi's V3D/KMS
# stack; forcing software rendering + Wayland fixes the issue.
#
# Pass a package directory, not a .go file: naming a file makes Go compile only
# that file, so it breaks as soon as an example has more than one.
#
# Usage: ./run_pi.sh <example-dir> [flags...]
# Example: ./run_pi.sh ./Number-Change-Detection -exp preliminary
SDL_RENDER_DRIVER=software SDL_VIDEODRIVER=wayland go run "$@"
