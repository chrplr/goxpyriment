// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

# examples/ — authoring a new experiment

This directory holds real experiments (record behavioural data) and demonstrations
(illusions, feature templates). This file is the orientation for **writing a new one**.
It points at the canonical sources rather than repeating them — when in doubt, read the
linked docs, not this summary.

## Publishing one example as its own standalone repo

To split a single example out into its own git/GitHub repository — so a
colleague can download a prebuilt binary or play it in the browser (WebAssembly
on GitHub Pages) — **follow [`docs/StandaloneExampleRepo.md`](../docs/StandaloneExampleRepo.md)**,
the step-by-step guide. It covers `share.sh`, the release and Pages CI
workflows, the go-sdl3-wasm fork wiring, and the gotchas (fork branch-ref vs.
pinned commit, `GOSUMDB=off` for fresh tags). Reference repos: `chrplr/Rush-Hour`
(browser build) and `chrplr/Language-Localizer-French-audio` (binaries only).
This is different from `make share-NAME`, which only exports a module to zip
(see `README.md`).

## Fastest correct path: copy a sibling

Don't start from a blank file. Copy the closest existing example and adapt it. Good
templates by paradigm:

| You're building | Start from |
|---|---|
| Single-stimulus reaction time | `Stroop_task/` |
| Adaptive threshold (staircase / QUEST) | `Contrast-Detection-QUEST/` |
| Rapid serial visual presentation | `Pictures-RSVP/` |
| 2-AFC perceptual decision | `Number-Comparison/` |

## Docs to consult, in order

1. `docs/GettingStarted.md` — tutorial + Python/Expyriment mapping, worked examples.
2. `docs/UserManual.md` — rendering model, timing, input, data, audio, design.
3. The relevant package `CLAUDE.md` (`../stimuli/`, `../design/`, `../results/`,
   `../staircase/`, `../control/`, …) — detailed API for the package you're using.
4. `docs/API.md` — complete public API reference.

Experiment code imports **only `control`** (plus `design`, `stimuli`, `results` as
needed) — never `go-sdl3` directly. SDL colors, key codes, types and helpers are
re-exported from `control` (see `../control/defaults.go`).

## Typical program skeleton

```go
func main() {
    exp := control.NewExperimentFromFlags("My Task", control.Black, control.White, 32)
    defer exp.End()

    // Declare the data columns (subject_id is prepended automatically).
    exp.AddDataVariableNames([]string{"trial", "condition", "response", "rt", "correct"})

    // Build the trial list (shuffle / counterbalance via design.*).
    trials := buildTrials()
    design.ShuffleList(trials)

    exp.Run(func() error {            // all rendering happens inside Run
        exp.ShowInstructions("…\n\nPress SPACE to start.")
        for i, t := range trials {
            exp.Blank(1000)          // inter-trial interval
            stim := stimuli.NewTextLine(t.word, 0, 0, t.color)
            key, rt, _ := exp.ShowAndGetRT(stim, responseKeys, -1)
            exp.Data.Add(i, t.cond, keyName(key), rt, isCorrect(key, t))
        }
        return nil                   // return control.EndLoop to stop early
    })
}
```

`exp.ShowAndGetRT(stim, keys, timeoutMS)` is the canonical single-stimulus RT call:
it clears stale events, flips with a VSYNC timestamp, waits for a key with
hardware-precision timing, and returns `(key, rtMs, error)`.

## How results are saved

`exp.Data` writes **two files** per session under the data directory (see
`../results/CLAUDE.md`):

- `<name>_sub-<NNN>_date-….csv` — pure CSV, directly importable by R/Excel.
- `<name>_sub-<NNN>_date-…-info.txt` — `#`-prefixed session metadata.

Workflow: call `exp.AddDataVariableNames([...])` once (header — do **not** include
`subject_id`, it's prepended), then `exp.Data.Add(...)` once per trial. Numbers and
bools are written bare; everything else is RFC-4180 quoted.

## Pitfalls checklist

- **RT comes from the SDL event clock**, never wall-clock deltas. Use `ShowAndGetRT`,
  or `exp.ShowTS` + `Keyboard.GetKeyEventTS` and subtract. See UserManual §6.
- **Multi-frame loops use `screen.PacedFlip()`**, not `screen.Update()` — some systems
  present in a non-blocking mode and a naive per-frame loop runs too fast. See
  UserManual §5/§6.
- **Never draw outside `exp.Run`** (the SDL main thread). Drawing from a goroutine
  silently does nothing or crashes.
- **Coordinates are center-relative**: `(0,0)` is screen centre, `sdl.FPoint{X,Y}`.
  **+Y points UP** (larger Y = higher on screen; opposite of SDL Y-down). Using
  negative Y for "up" mirrors the layout vertically — a recurring bug.
- **Use `control.*` constants** for colors and key codes; don't import `go-sdl3`.
- **Add a `meta.yaml`** (`category: experiment` or `demo`, plus `description:` and
  `reference:`) so the example appears in `docs/GalleryOfExamples.md`. Regenerate with
  `make update-examples-gallery`.
- **Copyright header** on every new `.go` file (see root `CLAUDE.md`).
- **Run from the repo root** so `go.work` resolves the workspace
  (`go run ./examples/<name>`). Most examples accept `-w` (windowed), `-d N`
  (display), `-s <id>` (subject).
- **Name the package, not `main.go`.** `go run ./examples/<name>` (or `go run .`
  from inside the directory) compiles every file in the package. `go run
  examples/<name>/main.go` compiles *only that file* — it happens to work while
  an example is a single file, and silently stops finding the rest the moment
  you add a second one. `RSVP-Images` is the example that already has two.
