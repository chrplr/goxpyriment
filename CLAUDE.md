# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

`goxpyriment` is a Go framework for building behavioral and psychological experiments, inspired by [expyriment.org](http://expyriment.org). It wraps SDL3 (via `go-sdl3`) for hardware-accelerated stimulus presentation with high-precision VSYNC-locked timing.

**Status: alpha / proof-of-concept.** Expect rough edges.

## Build & run

**Prerequisites:** Go 1.25+, SDL3 development libraries (`sudo apt install libsdl3-dev` on Linux).

```bash
# Run a single example directly
go run examples/parity_decision/main.go

# Build a single example
cd examples/parity_decision && go build .

# Build all examples
cd examples && ./build.sh

# Build/test a package
go build ./stimuli/
go build ./...
```

Most examples accept `-d` for windowed development mode and `-s <id>` for subject ID.

There are no automated tests (`go test` will find nothing meaningful). Verification is manual: build the package, then run an example with a real display.

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
The entry point for every experiment. `NewExperiment(...)` + `Initialize()` + `defer End()` sets up the SDL window, default font, audio device, keyboard/mouse handlers, and event log. `exp.Run(func() error {...})` wraps the main trial loop. Key fields: `exp.Screen`, `exp.Keyboard`, `exp.Mouse`, `exp.AudioDevice`, `exp.Data`, `exp.Design`.

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

### io/
`Screen` wraps `sdl.Window` + `sdl.Renderer`. All stimulus positions use a **center-origin coordinate system** (0,0 = screen center); `screen.CenterToSDL(x, y)` converts to SDL's top-left origin. `screen.Clear()` + `screen.Update()` map to SDL clear + present (VSYNC blocks in `Update`).

Data is written to `.xpd` files (CSV with metadata header) via `io.DataFile`.

### design/
`design.Experiment` → `[]Block` → `[]Trial`, each with `map[string]string` factors. `AddBWSFactor` + `GetPermutedBWSFactorCondition` implement Latin-square between-subject counterbalancing.

## Key conventions

- **Coordinate system:** all positions are screen-center relative (`(0,0)` = center). Use `sdl.FPoint{X: x, Y: y}`.
- **Colors:** defined in `control/defaults.go` (`control.Black`, `control.White`, `control.Red`, etc.) as `sdl.Color`.
- **Embedding assets:** use `//go:embed` to bundle fonts, images, and audio into the binary.
- **go.mod indirect → direct:** when a new package starts importing a previously-indirect dependency, move it to the direct `require` block manually (or run `go mod tidy`).
- **Error handling:** functions return `error`; callers use `log.Fatalf` or propagate. No panics in library code.
- **GC during timing:** disable with `debug.SetGCPercent(-1)` and defer restore around any VSYNC-locked loop, following the pattern in `stimuli/stream.go` and `stimuli/gvvideo.go`.
