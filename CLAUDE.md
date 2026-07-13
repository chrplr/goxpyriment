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

`README.md` (repo root) is **generated** from `docs/index.md` — do not edit it by
hand. `docs/index.md` is the single source of truth for the landing page; edit it,
then run `make readme` (which runs `cmd/gen-readme`, copying the content and
rewriting relative links to reach files through `docs/`). A CI guard
(`.github/workflows/readme-sync.yml`) fails if `README.md` is out of date.

The docs site is built with [Zensical](https://zensical.org/). Build and preview locally (Makefile targets at repo root):

```bash
pip install -r docs/requirements.txt   # install Zensical once

make pdfs      # generate docs/*.pdf via pandoc + xelatex
make serve     # live-reload preview at http://127.0.0.1:8000 (zensical serve)
make docs      # build static HTML → site/ (zensical build --clean)
make deploy    # generate PDFs + build site locally
make clean     # remove _build/ and site/
```

PDFs and the `site/` directory are excluded from git (see `.gitignore`); they are generated locally. The push to GitHub Pages is handled by GitHub Actions (`.github/workflows/docs.yml`).

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

### SDL3 is bundled — no system install required

SDL3, SDL3_ttf, and SDL3_image are embedded as gzip-compressed blobs inside the Go binary via `go-sdl3`'s `binsdl`/`binttf`/`binimg` packages (see `vendor/github.com/Zyko0/go-sdl3/bin/`). `control.Initialize()` calls `binsdl.Load()` which decompresses the library to a temp directory and loads it via `dlopen`. No system SDL3 package is needed on the target machine.

### NVIDIA + X11 — fullscreen rendering

On Linux with NVIDIA proprietary drivers and X11, the OpenGL renderer can silently fail in fullscreen mode (blank screen or SIGSEGV in `SDL_RenderPresent`). Windowed mode (`-w`) is unaffected. `apparatus/screen.go` now hints SDL to prefer the Vulkan renderer on Linux, which resolves this with NVIDIA RTX hardware. If Vulkan is unavailable, SDL falls back to OpenGL.

Manual override if needed:
```bash
SDL_RENDER_DRIVER=vulkan ./my_experiment      # force Vulkan
SDL_RENDER_DRIVER=software ./my_experiment    # force software (always works)
./my_experiment -w                            # windowed mode (avoids fullscreen path)
```

### Browser / WebAssembly (GOOS=js)

Experiments also compile to WASM and run in a browser (verified: `hello_world`
renders in Chrome). Full status, build commands, and the remaining-work roadmap
live in `docs/WASM.md`. The essentials:

- go-sdl3 is replaced by the fork `github.com/chrplr/go-sdl3-wasm` (branch
  `wasm-render-fixes`; local clone at `~/00_git/go-sdl3-wasm`), which
  implements the js bindings. `go.mod` pins a pseudo-version of the fork —
  after changing the fork, bump the pseudo-version (or temporarily point the
  replace at the local clone) and re-sync `vendor/` with
  `GOWORK=off go mod vendor`. Projects that import goxpyriment need the same
  `replace` line in their own `go.mod` for browser builds (see
  `docs/WASM.md`).
- Bundle/serve with the fork's tool:
  `go run ./cmd/wasmsdl serve <path-to-example>` (from the fork directory).
- In the browser, flags come from URL query parameters (`?s=3&w`); the
  participant-info dialog never opens (see `control/platform_js.go`).
- Assets must be `//go:embed`-ed (no filesystem in the browser); path-based
  loaders fail on js.
- `cmd/gen-wasm-exports` lists the go-sdl3 calls goxpyriment uses whose js
  bindings are still panic-stubs, and regenerates the emcc export list.
- `triggers/` is desktop-only (serial ports) and is excluded from js builds —
  don't add it to `GOOS=js go build ./...` expectations.

### Raspberry Pi — fullscreen rendering workaround

On Raspberry Pi (tested: Ubuntu 25.10 + GNOME/Wayland), fullscreen mode renders nothing (gray screen) while windowed mode works correctly. The SDL3 exclusive-fullscreen path does not properly attach the renderer to the visible framebuffer under the Pi's V3D/KMS stack. Workaround: force the software render driver and Wayland video driver:

```bash
SDL_RENDER_DRIVER=software SDL_VIDEODRIVER=wayland go run main.go
```

A convenience wrapper `examples/run_pi.sh` is available:

```bash
#!/bin/bash
SDL_RENDER_DRIVER=software SDL_VIDEODRIVER=wayland go run "$@"
```

Verification is typically manual: build the package, then run an example with a real display. However, core logic in packages like `control` have unit tests (`go test ./control`).

### Module / workspace layout

The repo uses a Go workspace (`go.work`) listing three modules: the library at the root, `examples/`, and `tests/`. Both `examples/` and `tests/` are **separate modules** (each a `go.mod` with a `replace github.com/chrplr/goxpyriment => ../` directive). When editing library code and running an example or test, always stay at the repo root so `go.work` resolves all modules correctly.

### examples/ vs tests/

- **`examples/`** holds real experiments (record behavioural data) and demonstrations (illusions, minimal feature templates) — the showcase a user browses to learn the framework.
- **`tests/`** holds standalone technical tests: hardware (`test_parallel_port`, `test_ft232h`, `test_labjackt4`, `test_linuxgpio`), timing/display (`Timing-Tests`, `tearing_test`, `test_av_sync`), and single-feature checks (`test_keyboard`, `test_menu`, `test_stream_*`). These are run and inspected by hand, not via `go test`. **Naming convention: prefix with `test_`** and use underscores (e.g. `test_text_input`, `test_joystick`).

Both folders are catalogued in `docs/GalleryOfExamples.md`. Each example/test directory carries a `meta.yaml` (`category:` is `experiment`, `demo`, or `test`; plus `description:` and `reference:`). `make update-examples-gallery` (runs `cmd/gen-gallery`) scans both `examples/` and `tests/` and regenerates the tables between the `<!-- BEGIN:experiments -->`, `<!-- BEGIN:demos -->`, and `<!-- BEGIN:tests -->` sentinels. Add a `meta.yaml` to any new example or test so it appears in the gallery (the generator warns about directories that lack one).

## Package architecture

The packages form a deliberate layered stack. Each package has its own `CLAUDE.md` with detailed API notes.

| Package | Role |
|---|---|
| `control/` | Top-level experiment orchestration — `Experiment` facade, SDL re-exports, participant info dialog |
| `stimuli/` | All visual and audio stimuli, VSYNC-locked animation loops, RSVP streams |
| `media/` | Multi-clip `.gv` video playback — `MovieManager`/`Movie` for back-to-back movies; complements the single-clip `stimuli.GvVideo`. See `media/CLAUDE.md` |
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

## Go Development

- After writing or modifying Go code, always run `go build ./...` and `go vet ./...` to verify the project compiles and vets cleanly before reporting completion. (The `/verify` skill does both.)
- Never silently swallow errors; always check and surface error returns (especially in rendering/SDL code) so failures are visible instead of producing blank screens.

## Build & Environment

- This project uses Go modules in vendor mode; do not edit vendored libraries directly—use package-level helper functions instead, and ensure cross-platform (Windows Git Bash) compatibility for scripts.

## Workflow Conventions

- Wait for explicit confirmation before starting edits when the user is discussing or agreeing with a prior suggestion, and actually start any server/process you offer to start.
