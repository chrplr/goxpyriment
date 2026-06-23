// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

# tests/ — standalone technical tests

This directory holds **hand-run** technical checks, not `go test` unit tests. They are
built and run against real hardware/displays and inspected visually. They fall into
three groups:

- **Hardware** — `test_parallel_port`, `test_ft232h`, `test_labjackt4`,
  `test_linuxgpio`, `test_joystick` …
- **Timing / display** — `Timing-Tests`, `tearing_test`, `test_av_sync`,
  `test_vsync_blocking`, `set_fullscreen` …
- **Single-feature checks** — `test_keyboard`, `test_menu`, `test_text_input`,
  `test_stream_*`, `test_images*` …

For library *unit* tests (run with `go test`), see `../control`, etc. — those live
beside the package, not here.

## Conventions

- **Name with a `test_` prefix** and underscores: `test_joystick`, `test_text_input`.
  (Timing/display fixtures predate this and keep their historic names.)
- **Add a `meta.yaml`** with `category: test`, a one-line `description:`, and
  `reference:` (may be empty). This puts the test in the `tests` table of
  `docs/GalleryOfExamples.md`. Regenerate with `make update-examples-gallery`; the
  generator warns about directories lacking a `meta.yaml`.
- **Copyright header** on every new `.go` file (see root `CLAUDE.md`).
- `tests/` is a **separate module** with its own `go.mod` (a `replace` directive points
  at the library). Run from the repo root so `go.work` resolves the workspace.

## Running

```bash
go run tests/<name>/main.go          # from repo root
cd tests/<name> && go run . -w       # or from the dir; -w = windowed
```

Most accept the same flags as examples: `-w` (windowed), `-d N` (display), `-s <id>`.
These are run and judged by a human at a real display — they are not part of CI and
have no automated pass/fail.

## Writing a new test vs. an example

Put it here if it exercises **hardware or a single framework feature** in isolation and
is verified by eye. Put it in `../examples/` if it's a real experiment that records
behavioural data or a demonstration users would browse to learn the framework. See
`../examples/CLAUDE.md` for the experiment-authoring workflow.
