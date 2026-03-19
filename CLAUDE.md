# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

`goxpyriment` is a Go framework for building behavioral and psychological experiments, inspired by [expyriment.org](http://expyriment.org). It wraps SDL3 (via `go-sdl3`) for hardware-accelerated stimulus presentation with high-precision VSYNC-locked timing.

**Status: alpha / proof-of-concept.** Expect rough edges.

## Build & run

**Prerequisites:** Go 1.25+, SDL3 development libraries (`sudo apt install libsdl3-dev` on Linux).

```bash
# Run a single example directly (from repo root — go.work handles the workspace)
go run examples/parity_decision/main.go

# Or from inside the example directory
cd examples/parity_decision && go run . -d -s 1

# Build a single example
cd examples/parity_decision && go build .

# Build all examples
cd examples && ./build.sh

# Build/check a library package (no test binary needed)
go build ./stimuli/
go build ./...
```

Most examples accept `-d` for windowed developer mode (1024×768 window) and `-s <id>` for subject ID.

There are no automated tests (`go test` finds nothing meaningful). Verification is manual: build the package, then run an example with a real display.

### Module / workspace layout

The repo uses a Go workspace (`go.work`). `examples/` is a **separate module** (`go.mod` with a `replace github.com/chrplr/goxpyriment => ../` directive). When editing library code and running examples, always stay at the repo root so `go.work` resolves both modules correctly.

## Package architecture

The packages form a deliberate layered stack:

```
control/      ← top-level experiment orchestration
stimuli/      ← stimulus objects (visual + audio)
io/           ← SDL window/renderer, keyboard, mouse, data files
design/       ← trial/block structure, randomization, counterbalancing
clock/        ← timing utilities
geometry/     ← math helpers (polar/cartesian, degrees)
```

### control/
The entry point for every experiment.

**Preferred boilerplate** — `NewExperimentFromFlags` handles flag parsing, initialization, and fatal-on-error in one call (most examples use this):
```go
exp := control.NewExperimentFromFlags("My Experiment", control.Black, control.White, 32)
defer exp.End()
```
Use the lower-level `NewExperiment(...) + Initialize() + defer End()` only when you need non-standard initialization order.

`exp.Run(func() error {...})` wraps the main trial loop; return `control.EndLoop` to exit cleanly, `nil` to continue. Key fields: `exp.Screen`, `exp.Keyboard`, `exp.Mouse`, `exp.AudioDevice`, `exp.Data`, `exp.Design`.

**Convenience methods:** `exp.Show(stim)` — clears, draws, and flips in one call. `exp.Blank(ms)` — clears, flips, and waits `ms` milliseconds.

**SDL re-exports in `control/defaults.go`** — import only `control`, not `go-sdl3`, for day-to-day experiment code:
- Types: `control.Color`, `control.FPoint`, `control.FRect`, `control.Keycode`
- Constructors: `control.Point(x, y)`, `control.Origin()`, `control.RGB(r,g,b)`, `control.RGBA(r,g,b,a)`
- Key codes: `control.K_SPACE`, `control.K_ESCAPE`, `control.K_F`, `control.K_J`, … (see `defaults.go` for full list)
- Mouse buttons: `control.BUTTON_LEFT`, `control.BUTTON_RIGHT`
- Loop sentinel: `control.EndLoop` (return from `exp.Run` callback to exit); `control.IsEndLoop(err)` to check

**Embedded assets** — `assets_embed` package bundles the default Inconsolata font, ping sound, and other shared assets; import as `github.com/chrplr/goxpyriment/assets_embed` and pass `assets_embed.InconsolataFont` (a `[]byte`) to `control.FontFromMemory`.

### stimuli/
All visual stimuli implement `VisualStimulus` (which extends `Stimulus`):
```go
type Stimulus interface {
    Present(screen *io.Screen, clear, update bool) error
    Preload() error   // no-op for most; actual GPU setup needs a Screen
    Unload() error
}
type VisualStimulus interface {
    Stimulus
    Draw(screen *io.Screen) error
    GetPosition() sdl.FPoint
    SetPosition(pos sdl.FPoint)
}
```
GPU textures are **lazily allocated** on first `Draw` call. `PreloadVisualOnScreen(screen, stim)` forces early allocation for timing-sensitive code.

`PresentStreamOfImages` is the high-precision RSVP loop: it disables GC, locks to VSYNC, and returns `[]UserEvent` + `[]TimingLog`.

`PlayGv(screen, path, x, y)` plays a `.gv` (LZ4-compressed RGBA) video file once, frame-by-frame, VSYNC-locked.

#### Animated / dynamic stimuli (VSYNC-locked loops)

Three functions run self-contained VSYNC-locked animation loops and all return a `MotionResult{Key, Button, RTms}` plus an `error`:

| Function | File | Description |
|----------|------|-------------|
| `PresentMovingDotCloud(screen, nDots, dotRadius, cloudRadius, center, speedPxPerSec, maxDurationMs, interruptKeys, catchMouse, dotColor, bgColor)` | `moving_dotcloud.go` | Animated random-dot cloud; each dot moves at a fixed speed in a random direction and is respawned when it exits the cloud boundary. |
| `PresentMovingGrating(screen, width, height, center, orientation, spatialFreq, temporalFreq, contrast, bgLuminance, maxDurationMs, interruptKeys, catchMouse)` | `moving_grating.go` | Drifting sinusoidal grating in a rectangular aperture. All spatial parameters are precomputed; only the phase advances per frame. |
| `PresentMovingGabor(screen, size, sigma, center, orientation, spatialFreq, temporalFreq, contrast, bgLuminance, maxDurationMs, interruptKeys, catchMouse)` | `moving_grating.go` | Drifting Gabor patch (grating × isotropic Gaussian envelope); rendered with per-pixel alpha so the edges fade into the experiment background. |

All three loops: disable GC (`debug.SetGCPercent(-1)`), drain the SDL event queue before the first frame, poll events after each VSYNC, and return `sdl.EndLoop` on ESC / window-close.

`spatialFreq` is in **cycles per pixel** (e.g. `0.05` = one cycle every 20 px). `temporalFreq` is in **Hz**. `orientation` is in **degrees from horizontal** (0° = vertical bars drifting right).

#### Audio segment playback

`Sound.PlaySegment(onset, offset, rampSec float64) error` plays only the portion of a loaded WAV between `onset` and `offset` (both in seconds). `rampSec` applies a linear fade-in at the onset and a symmetric fade-out at the offset; pass `0` for no ramp. The format-aware ramp handles AUDIO_F32*, AUDIO_S16*, and AUDIO_U8 natively; unknown formats are played without ramping. The original `Data` is never modified.

### io/
`Screen` wraps `sdl.Window` + `sdl.Renderer`. All stimulus positions use a **center-origin coordinate system** (0,0 = screen center); `screen.CenterToSDL(x, y)` converts to SDL's top-left origin. `screen.Clear()` + `screen.Update()` map to SDL clear + present (VSYNC blocks in `Update`).

Data is written to `.xpd` files (CSV with metadata header) via `io.DataFile`.

### design/
`design.Experiment` → `[]Block` → `[]Trial`, each with `map[string]string` factors. `AddBWSFactor` + `GetPermutedBWSFactorCondition` implement Latin-square between-subject counterbalancing.

#### Copyright header

Every `.go` file in the repository (outside `vendor/`) carries:
```go
// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
```
New files must include this header.

## Key conventions

- **Coordinate system:** all positions are screen-center relative (`(0,0)` = center). Use `sdl.FPoint{X: x, Y: y}`.
- **Colors:** defined in `control/defaults.go` (`control.Black`, `control.White`, `control.Red`, etc.) as `sdl.Color`.
- **Embedding assets:** use `//go:embed` to bundle fonts, images, and audio into the binary.
- **go.mod indirect → direct:** when a new package starts importing a previously-indirect dependency, move it to the direct `require` block manually (or run `go mod tidy`).
- **Error handling:** functions return `error`; callers use `log.Fatalf` or propagate. No panics in library code.
- **GC during timing:** disable with `debug.SetGCPercent(-1)` and defer restore around any VSYNC-locked loop, following the pattern in `stimuli/stream.go` and `stimuli/gvvideo.go`.
