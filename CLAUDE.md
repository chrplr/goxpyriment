# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Documentation

All user-facing documentation lives in `docs/`:

| File | Contents |
|---|---|
| `docs/GettingStarted.md` | Tutorial introduction — Python/Expyriment mapping, 3 worked examples |
| `docs/MigrationGuide.md` | Migration reference — concept maps and side-by-side code for Expyriment, PsychoPy, Psychtoolbox |
| `docs/UserManual.md` | Concept guide — rendering model, timing, input, data, streams, audio, design |
| `docs/API.md` | Complete public API reference organized by package |

Build and preview the docs site locally (Makefile targets at repo root):

```bash
pip install -r docs/requirements.txt   # install MkDocs + Material once

make pdfs      # generate docs/*.pdf via pandoc + xelatex
make serve     # live-reload preview at http://127.0.0.1:8000
make docs      # build static HTML → site/
make deploy    # generate PDFs + build + push to GitHub Pages
make clean-docs  # remove site/
```

PDFs and the `site/` directory are excluded from git (see `.gitignore`); they are generated locally and pushed to the `gh-pages` branch via `make deploy`.

## What this repo is

`goxpyriment` is a Go framework for building behavioral and psychological experiments, inspired by [expyriment.org](http://expyriment.org). It wraps SDL3 (via `go-sdl3`) for hardware-accelerated stimulus presentation with high-precision VSYNC-locked timing.

**Status: alpha / proof-of-concept.** Expect rough edges.

## Build & run

**Prerequisites:** Go 1.25+.

```bash
# Run a single example directly (from repo root — go.work handles the workspace)
go run examples/parity_decision/main.go

# Or from inside the example directory
cd examples/parity_decision && go run . -w -s 1

# Build a single example
cd examples/parity_decision && go build .

# Build all examples
cd examples && ./build.sh

# Build/check a library package (no test binary needed)
go build ./stimuli/
go build ./...
```

Most examples accept `-w` for windowed mode (1024×768 window), `-d N` for display selection (monitor index, -1 = primary), and `-s <id>` for subject ID.

Verification is typically manual: build the package, then run an example with a real display. However, core logic in packages like `control` have unit tests (`go test ./control`).

### Module / workspace layout

The repo uses a Go workspace (`go.work`). `examples/` is a **separate module** (`go.mod` with a `replace github.com/chrplr/goxpyriment => ../` directive). When editing library code and running examples, always stay at the repo root so `go.work` resolves both modules correctly.

## Package architecture

The packages form a deliberate layered stack. Each package has its own `CLAUDE.md` with detailed API notes.

| Package | Role |
|---|---|
| `control/` | Top-level experiment orchestration — `Experiment` facade, SDL re-exports, participant info dialog |
| `stimuli/` | All visual and audio stimuli, VSYNC-locked animation loops, RSVP streams |
| `apparatus/` | SDL window/renderer (`Screen`), keyboard, mouse, gamepad, gamma corrector, response device abstraction |
| `results/` | Experiment data file (`.csv` with `#`-prefixed metadata), buffered output file |
| `design/` | Trial/block structure, randomization utilities, Latin-square counterbalancing |
| `staircase/` | Adaptive threshold estimation — `UpDown` (Levitt 1971) and `Quest` (Watson & Pelli 1983) |
| `units/` | Vision-science unit conversions — pixels↔degrees↔cm via a `Monitor` struct |
| `triggers/` | Hardware trigger interfaces — parallel port, DLP-IO8 USB, generic serial |
| `clock/` | Timing utilities — `Clock` type with `SleepUntil`, global `GetTime` |
| `geometry/` | Math helpers — Euclidean distance, polar↔Cartesian, degree→radian |
| `assets_embed/` | Embedded assets — Inconsolata font, ping/buzzer sounds |

### Minimal boilerplate

```go
exp := control.NewExperimentFromFlags("My Experiment", control.Black, control.White, 32)
defer exp.End()
exp.Run(func() error {
    // return control.EndLoop to exit, nil to continue
})
```

`NewExperimentFromFlags` parses `-w` (windowed mode), `-d N` (display index, -1 = primary), and `-s <subjectID>`, then initialises SDL, audio, window, font, and data file. Key fields: `exp.Screen`, `exp.Keyboard`, `exp.Mouse`, `exp.AudioDevice`, `exp.Data`, `exp.Design`.

**Convenience methods:** `exp.Show(stim)` — clear + draw + flip. `exp.Blank(ms)` — clear + flip + sleep.

**SDL re-exports** in `control/defaults.go` — import only `control` in experiment code (never `go-sdl3` directly): colors (`control.Black` … `control.Gray`), key codes (`control.K_SPACE`, `control.K_F`, …), mouse buttons, type aliases (`Color`, `FPoint`, `FRect`, `Keycode`), helpers (`Point`, `Origin`, `RGB`, `RGBA`, `FontFromMemory`), and the loop sentinel `control.EndLoop` / `control.IsEndLoop(err)`.

**Embedded assets** — `assets_embed` bundles the default Inconsolata font and sounds:
```go
import "github.com/chrplr/goxpyriment/assets_embed"
font, _ := control.FontFromMemory(assets_embed.InconsolataFont, 32)
```

### design/
`design.Experiment` → `[]Block` → `[]Trial`, each with `map[string]interface{}` factors. `AddBWSFactor` + `GetPermutedBWSFactorCondition` implement Latin-square between-subject counterbalancing. See `design/CLAUDE.md`.

### stimuli/
GPU textures are **lazily allocated** on first `Draw` call. `PreloadVisualOnScreen(screen, stim)` forces early allocation for timing-sensitive code. `PresentStreamOfImages` is the high-precision RSVP loop (GC disabled, VSYNC-locked). See `stimuli/CLAUDE.md`.

`spatialFreq` parameters are in **cycles per pixel**. `temporalFreq` is in **Hz**. `orientation` is in **degrees from horizontal**.

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
