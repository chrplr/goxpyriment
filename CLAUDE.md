# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Documentation

All user-facing documentation lives in `docs/`:

| File | Contents |
|---|---|
| `docs/GettingStarted.md` | Tutorial introduction — Python/Expyriment mapping, 3 worked examples |
| `docs/MigrationGuide.md` | Migration reference — concept maps and side-by-side code for Expyriment, PsychoPy, Psychtoolbox |
| `docs/ComparisonWithPsychoPy.md` | Feature-by-feature comparison with PsychoPy — parity, gaps, and how to choose |
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

The generated `docs/*.pdf` files **are tracked in git** — regenerate them with
`make pdfs` and commit the result when the underlying Markdown changes. The
`site/` directory is excluded (see `.gitignore`); it is built locally and the
push to GitHub Pages is handled by GitHub Actions (`.github/workflows/docs.yml`).

## What this repo is

`goxpyriment` is a Go framework for building behavioral and psychological experiments, inspired by [expyriment.org](http://expyriment.org). It wraps SDL3 (via `go-sdl3`) for hardware-accelerated stimulus presentation with high-precision VSYNC-locked timing.

**Status: alpha / proof-of-concept.** Expect rough edges.

## Build & run

**Prerequisites:** Go 1.25+.

```bash
# Run a single example directly (from repo root — go.work handles the workspace).
# Name the package, not main.go: naming a file compiles only that file, so the
# command silently breaks the moment an example gains a second .go file.
go run ./examples/parity_decision

# Or from inside the example directory
cd examples/parity_decision && go run . -w -s 1

# Build a single example
cd examples/parity_decision && go build .

# Build all examples
cd examples && ./build.sh

# Export one example as a self-contained module a colleague can build alone
# (generated go.mod requiring the published goxpyriment; no go.work/replace).
# Output: _build/share/NAME/ — see examples/share.sh and examples/README.md.
make share-parity_decision

# Convert a video, or a directory of numbered images, to the .gv format used
# for timing-critical playback. MPEG-1 is decoded in pure Go; other video
# formats need ffmpeg on PATH. Image dirs sort numerically (not
# lexicographically) and reject size mismatches unless -force-size is passed.
make gv-convert
./_build/gv-convert clip.mpg clip.gv     # video
./_build/gv-convert frames/ anim.gv      # image sequence

# Inspect a .gv file (frame count, frame size, fps, compression, consistency)
make gv-getinfo && ./_build/gv-getinfo clip.gv

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

Experiments also compile to WASM and run in a browser (verified: `demo_hello_world`
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
- Bundle/serve from the repo root: `make wasm-NAME` builds a self-contained
  bundle into `_build/wasm/NAME/`; `make wasm-NAME-serve` serves it at
  http://localhost:8080/?s=1 (runs the fork's `wasmsdl` bundler out of the
  module graph — no local clone or Emscripten needed).
- In the browser, flags come from URL query parameters (`?s=3&w`); the
  participant-info dialog never opens (see `control/platform_js.go`).
- Assets must be `//go:embed`-ed (no filesystem in the browser); path-based
  loaders fail on js.
- `cmd/gen-wasm-exports` lists the go-sdl3 calls goxpyriment uses whose js
  bindings are still panic-stubs, and regenerates the emcc export list.
- `triggers/` is desktop-only (serial ports) and is excluded from js builds —
  don't add it to `GOOS=js go build ./...` expectations.

Verification is typically manual: build the package, then run an example with a real display. However, core logic in packages like `control` have unit tests (`go test ./control`).

### Module / workspace layout

The repo uses a Go workspace (`go.work`) listing three modules: the library at the root, `examples/`, and `tests/`. Both `examples/` and `tests/` are **separate modules** (each a `go.mod` with a `replace github.com/chrplr/goxpyriment => ../` directive). When editing library code and running an example or test, always stay at the repo root so `go.work` resolves all modules correctly.

### examples/ vs tests/

The distinction is **what you do with the output**, and it maps to the two folders:

- **`examples/`** holds two `meta.yaml` categories:
  - **`experiment`** — a real paradigm that records behavioural data (e.g. `Stroop_task`, `parity_decision`).
  - **`demo`** — a short program that demonstrates the use of a feature/function; nothing is measured (illusions, minimal templates, single-widget showcases). **Demo directories are prefixed `demo_`** (e.g. `demo_hello_world`, `demo_menu`, `demo_stream_images`) so they stand out when browsing `examples/`.
- **`tests/`** holds only **`test`** — programs whose *results are analysed to check performance*, typically timing or hardware: timing/display (`Timing-Tests`, `tearing_test`, `test_av_sync`, `test_vsync_blocking`, `set_fullscreen`) and hardware triggers (`test_parallel_port`, `test_ft232h`, `test_labjackt4`, `test_linuxgpio`, plus `GvFiles`/`test_stream_trigger` for photodiode/TTL sync). These are run and inspected by hand, not via `go test`. **Naming convention: prefix with `test_`** and use underscores.

Rule of thumb: *demonstrates how to use a function* → `demo` in `examples/`; *measures whether something performs* → `test` in `tests/`.

Both folders are catalogued in `docs/GalleryOfExamples.md`. Each directory carries a `meta.yaml` (`category:` is `experiment`, `demo`, or `test`; plus `description:` and `reference:`). `make update-examples-gallery` (runs `cmd/gen-gallery`) scans both `examples/` and `tests/` and regenerates the tables between the `<!-- BEGIN:experiments -->`, `<!-- BEGIN:demos -->`, and `<!-- BEGIN:tests -->` sentinels. Add a `meta.yaml` to any new example or test so it appears in the gallery (the generator warns about directories that lack one).

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
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
```
New files must include this header.

## Key conventions

- **Coordinate system:** all positions are screen-center relative (`(0,0)` = center), and **+Y points UP** (opposite of SDL's Y-down pixels — `Screen.CenterToSDL` computes `height/2 - y`). Larger Y = higher on screen; use *negative* Y to go below center. Using negative Y for "up" mirrors the layout vertically — a recurring bug. Use `sdl.FPoint{X: x, Y: y}`.
- **Colors:** defined in `control/defaults.go` (`control.Black`, `control.White`, `control.Red`, etc.) as `sdl.Color`.
- **Embedding assets:** use `//go:embed` to bundle fonts, images, and audio into the binary.
- **go.mod indirect → direct:** when a new package starts importing a previously-indirect dependency, move it to the direct `require` block manually (or run `go mod tidy`).
- **Error handling:** functions return `error`; callers use `log.Fatalf` or propagate. No panics in library code.
- **GC during timing:** disable with `debug.SetGCPercent(-1)` and defer restore around any VSYNC-locked loop, following the pattern in `stimuli/stream.go` and `stimuli/gvvideo.go`.

## Before pushing: read the workflows, don't guess

`go build ./...` passing on this machine does not mean CI passes. Read
`.github/workflows/*.yml` and run what it actually runs — the file is short and
the check costs one grep. Two failures on 2026-08-07 were both discoverable this
way beforehand:

- **Cross-compilation targets.** CI builds `GOOS=js GOARCH=wasm` (with
  `triggers/` excluded, which is itself the clue that this target exists). New
  build-tagged files were cross-compiled for windows, darwin and freebsd but not
  for the browser, and a `!linux && !windows && !darwin` tag silently caught
  `js/wasm`. `x/sys/unix` compiles there and defines none of the syscalls, so
  nothing fails until link time. Any new `_other.go` fallback needs
  `&& !js && !wasip1` and a sandbox implementation.
- **Generated files.** `README.md` is generated from `docs/index.md` by
  `cmd/gen-readme`, and a CI job fails if they drift. Editing `docs/index.md`
  without running `make readme` left it red for three pushes. Before editing any
  file, check whether the `Makefile` has a target that regenerates it.

The general rule: a repo states its own invariants in its workflows, Makefile
and build tags. Inferring them from the source alone finds the ones that happen
to be visible.

## Go Development

- After writing or modifying Go code, always run `go build ./...` and `go vet ./...` to verify the project compiles and vets cleanly before reporting completion. (The `/verify` skill does both.)
- Before pushing, also run the CI workflow's own steps, including the
  `GOOS=js GOARCH=wasm` builds. See the section above.
- Never silently swallow errors; always check and surface error returns (especially in rendering/SDL code) so failures are visible instead of producing blank screens.

## Build & Environment

- This project uses Go modules in vendor mode; do not edit vendored libraries directly—use package-level helper functions instead, and ensure cross-platform (Windows Git Bash) compatibility for scripts.

## Workflow Conventions

- Wait for explicit confirmation before starting edits when the user is discussing or agreeing with a prior suggestion, and actually start any server/process you offer to start.
