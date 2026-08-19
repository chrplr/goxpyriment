# SDL3 Fullscreen in a Linux Virtual Console (TTY)

> This page is the narrow troubleshooting note for one symptom: SDL not going
> fullscreen on a bare console. For running experiments there properly — the
> groups it needs, why the console's display mode (not the monitor's native one)
> is the mode you get, how to pin it, and what a second lit head does to the
> vblank clock — see [Running from a virtual
> console](SettingPriorityUnderLinux.md#running-from-a-virtual-console-vt).

## Problem

When running with no display server (GDM stopped, bare TTY), SDL3 may fail to switch to
fullscreen — the console stays visible instead of the experiment window.

## Root Cause

SDL3 auto-detects the video driver. Without X11 or Wayland running, it should fall back to
the **KMS/DRM** (`kmsdrm`) driver, but auto-detection may pick a non-functional driver silently.

## Diagnostics

Check which driver SDL3 selects and whether KMS is available:

```bash
# Explicit KMSDRM driver — should work in a bare TTY
SDL_VIDEODRIVER=kmsdrm go run main.go -test display

# Check if SDL3 was built with KMSDRM support (no display needed)
SDL_VIDEODRIVER=offscreen go run main.go -test display 2>&1 | head -20
```

If `SDL_VIDEODRIVER=kmsdrm` gives "no available video device", SDL3 was not built with KMSDRM
support — you need to rebuild SDL3 with KMSDRM enabled, or install a package that includes it.

## Fix

### Option 1 — environment variable (quickest)

```bash
SDL_VIDEODRIVER=kmsdrm go run main.go -test display
```

Or via the existing wrapper:

```bash
# e.g. a run_vt.sh wrapper
SDL_VIDEODRIVER=kmsdrm go run "$@"
```

### Option 2 — programmatic hint in screen.go

In `apparatus/screen.go`, set the hint before `sdl.Init` when no display server is detected:

```go
import "os"

// Before sdl.Init(...)
if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
    sdl.SetHint("SDL_VIDEODRIVER", "kmsdrm")
}
```

## Notes

- `window.SetFullscreen(true)` after creation may be unreliable on KMSDRM; creating the window
  with fullscreen from the start (as `apparatus/screen.go` already does) is the right approach.
- On a machine that already has a compositor running, `SDL_VIDEODRIVER=wayland`
  is the right choice; `kmsdrm` is for a bare TTY with no compositor at all.
- Ensure the user running the program is in the `video` (and possibly `input`) group:
  ```bash
  sudo usermod -aG video,input $USER
  ```
  KMS/DRM access is permission-gated; without group membership SDL will fail to open the device.
