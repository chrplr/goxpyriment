# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

`goxpyriment` is a Go framework for building behavioral and psychological experiments, inspired by [expyriment.org](http://expyriment.org). It wraps SDL3 (via `go-sdl3`) for hardware-accelerated stimulus presentation with high-precision VSYNC-locked timing. **Status: alpha.**

Task-specific notes live in skills, loaded on demand: `/verify` (what CI runs),
`docs` (docs site, PDFs, generated README), `wasm` (browser builds and the
go-sdl3 fork). Most packages have their own `CLAUDE.md`. `make help` lists the
remaining tools (`gv-convert`, `gv-getinfo`, `share-NAME`, …).

## Build & run

Go 1.25+. SDL3, SDL3_ttf and SDL3_image are bundled inside the binary (go-sdl3's `binsdl`/`binttf`/`binimg`, loaded via `dlopen` from a temp dir) — no system SDL install is needed.

```bash
# From the repo root (go.work resolves the workspace). Name the package, not
# main.go: naming a file compiles only that file.
go run ./examples/parity_decision -w -s 999
go build ./...
cd examples && ./build.sh          # build all examples
```

Most examples accept `-w` (1024×768 window), `-d N` (display index, -1 = primary) and `-s <id>` (subject ID). Unit tests exist for the core logic packages (see `/verify` for the list); everything display-related is verified by running a program on a real display.

### Module / workspace layout

`go.work` lists three modules: the library at the root, `examples/`, and `tests/` — each of the latter two has its own `go.mod` with `replace github.com/chrplr/goxpyriment => ../`. Stay at the repo root so `go.work` resolves all of them. The project uses **vendor mode**: never edit `vendor/`; use package-level helpers instead. Scripts must also work under Windows Git Bash.

### examples/ vs tests/

The distinction is **what you do with the output**:

- **`examples/`** — `meta.yaml` category **`experiment`** (a real paradigm recording behavioural data, e.g. `Stroop_task`) or **`demo`** (demonstrates a feature; nothing measured; directory **prefixed `demo_`**).
- **`tests/`** — category **`test`**: programs whose results are analysed to check timing or hardware performance (e.g. `Timing-Tests`, `test_tearing`, `test_parallel_port`). Run and inspected by hand, not via `go test`. **Prefix `test_`**, underscores.

Every example/test directory needs a `meta.yaml` (`category:`, `description:`, `reference:`). After touching a `meta.yaml`, an example `README.md`, or `examples/installers/wasm-skip.txt`, run `make update-examples-gallery` — CI fails if `docs/GalleryOfExamples.md` or the per-example README link blocks are stale. See `examples/CLAUDE.md` for writing a new example.

## Generated files

Before editing any file, `grep` the `Makefile` for its name. Notably `README.md` is **generated** from `docs/index.md` (`make readme`), and the gallery tables from `meta.yaml` files (`make update-examples-gallery`); both have CI sync guards. `docs/*.pdf` are tracked and regenerated with `make pdfs`/`make book` (see the `docs` skill).

## Package architecture

| Package | Role |
|---|---|
| `control/` | Top-level orchestration — `Experiment` facade, SDL re-exports, participant info dialog |
| `stimuli/` | All visual and audio stimuli, VSYNC-locked animation loops, RSVP streams |
| `media/` | Multi-clip `.gv` video playback (`MovieManager`/`Movie`); complements single-clip `stimuli.GvVideo` |
| `apparatus/` | SDL window/renderer (`Screen`), keyboard, mouse, gamepad, gamma corrector, response devices |
| `results/` | Data file (plain `.csv`; `#`-metadata goes to a companion `-info.txt`), buffered output |
| `design/` | Trial/block structure, randomization, Latin-square counterbalancing |
| `staircase/` | Adaptive thresholds — `UpDown` (Levitt 1971), `Quest` (Watson & Pelli 1983) |
| `units/` | Pixels↔degrees↔cm via a `Monitor` struct |
| `eyetracker/` | Vendor-neutral `Tracker` interface, socket client for SDK bridges (EyeLink, Tobii), mouse simulator |
| `triggers/` | Hardware/network trigger interfaces (parallel port, GPIO, DLP-IO, FT232H, LabJack, MEG TTL box, serial, NetStation, BEL) — desktop only |
| `clock/` | `Clock` with `SleepUntil`, global `GetTime` |
| `geometry/` | Distance, polar↔Cartesian, degree→radian |
| `assets_embed/` | Embedded Inconsolata font, ping/buzzer sounds |
| `vblank/` | Per-platform vblank clocks (Linux DRM, macOS CVDisplayLink) anchoring flip timestamps |
| `sysinfo/` | Machine snapshot printed by `-sysinfo` and written into every `-info.txt` |

`assets_embed/`, `vblank/` and `sysinfo/` have no `CLAUDE.md` — read their doc comments.

## Key conventions

- **Coordinate system:** positions are screen-center relative (`(0,0)` = center), and **+Y points UP** (opposite of SDL's Y-down pixels — `Screen.CenterToSDL` computes `height/2 - y`). Use *negative* Y to go below center. Using negative Y for "up" mirrors the layout vertically — a recurring bug.
- **Import only `control`** in experiment code, never `go-sdl3` directly: `control/defaults.go` re-exports colors, key codes, mouse buttons, type aliases (`Color`, `FPoint`, …), helpers (`FontFromMemory`, `RGB`, …) and `control.EndLoop` / `control.IsEndLoop(err)`.
- **Embedding assets:** use `//go:embed` for fonts, images and audio (required in the browser build).
- **Error handling:** functions return `error`; callers use `log.Fatalf` or propagate. No panics in library code. Never silently swallow errors — especially in rendering/SDL code, where a dropped error becomes a blank screen.
- **GC during timing:** disable with `debug.SetGCPercent(-1)` and defer restore around any VSYNC-locked loop (pattern in `stimuli/stream.go`, `stimuli/gvvideo.go`).
- **Build tags:** a `!linux && !windows && !darwin` fallback silently catches `js/wasm`, and fails only at link time. Any new `_other.go` fallback needs `&& !js && !wasip1` plus a js implementation.
- **go.mod indirect → direct:** when a package starts importing a previously-indirect dependency, move it to the direct `require` block (or `go mod tidy`).
- **Copyright header** on every `.go` file outside `vendor/` (new files included):
  ```go
  // Copyright (2026) Christophe Pallier <christophe@pallier.org>
  // Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
  ```

## Verifying

Run `/verify` before declaring a Go change done and before any push — it runs what CI runs (desktop build, vet, unit tests, `GOOS=js GOARCH=wasm` builds, generated-file sync). A local `go build ./...` alone has missed CI failures before.

## Workflow conventions

- Wait for explicit confirmation before starting edits when the user is discussing or agreeing with a prior suggestion, and actually start any server/process you offer to start.
