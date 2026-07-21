// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

# tests/ — standalone technical tests

This directory holds **hand-run** technical checks, not `go test` unit tests. A test
belongs here when its **results are analysed to gauge performance** — typically timing
or hardware behaviour. They are built and run against real hardware/displays and
inspected visually. They fall into two groups:

- **Timing / display** — `Timing-Tests`, `tearing_test`, `test_av_sync`,
  `test_vsync_blocking`, `set_fullscreen`, `test_fullscreen` …
- **Hardware** — `test_parallel_port`, `test_ft232h`, `test_labjackt4`,
  `test_linuxgpio`, plus `GvFiles` and `test_stream_trigger` (photodiode / TTL
  display-sync checks) …

Programs that merely **demonstrate how to use a feature** (a stimulus, widget, input
method, stream helper) are *not* tests — they live in `../examples/` as `demo_`-prefixed
demos. For library *unit* tests (run with `go test`), see `../control`, etc. — those
live beside the package, not here.

## Conventions

- **Name with a `test_` prefix** and underscores: `test_ft232h`, `test_av_sync`.
  (Timing/display fixtures like `Timing-Tests` predate this and keep their historic
  names.)
- **Add a `meta.yaml`** with `category: test`, a one-line `description:`, and
  `reference:` (may be empty). This puts the test in the `tests` table of
  `docs/GalleryOfExamples.md`. Regenerate with `make update-examples-gallery`; the
  generator warns about directories lacking a `meta.yaml`.
- **Copyright header** on every new `.go` file (see root `CLAUDE.md`).
- `tests/` is a **separate module** with its own `go.mod` (a `replace` directive points
  at the library). Run from the repo root so `go.work` resolves the workspace.

## Running

```bash
go run ./tests/<name>                # from repo root
cd tests/<name> && go run . -w       # or from the dir; -w = windowed
```

Name the package, not `main.go`: `go run tests/<name>/main.go` compiles only
that one file, so it breaks as soon as a test has more than one.

Most accept the same flags as examples: `-w` (windowed), `-d N` (display), `-s <id>`.
These are run and judged by a human at a real display — they are not part of CI and
have no automated pass/fail.

## Writing a new test vs. an example

Put it here only if its **results are analysed to check performance** (timing, AV/TTL
sync) or it **exercises hardware**, verified by eye. If it merely *demonstrates how to
use* a framework feature in isolation, it is a **demo** — put it in `../examples/` as a
`demo_`-prefixed directory (`category: demo`). Real experiments that record behavioural
data also go in `../examples/`. See `../examples/CLAUDE.md` for the authoring workflow.
