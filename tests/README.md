# tests/

Standalone **technical tests** for `goxpyriment`: hardware and timing/display
checks whose results are analysed by hand to verify performance, rather than
full experiments. (Programs that merely demonstrate a feature are **demos** and
live in [`../examples/`](../examples/) with a `demo_` prefix.)

- **Timing & display** — `Timing-Tests`, `tearing_test`, `test_av_sync`, `test_vsync_blocking`, `set_fullscreen`, `test_fullscreen`.
- **Hardware triggers / I/O** — `test_parallel_port`, `test_ft232h`, `test_labjackt4`, `test_linuxgpio`.
- **Display / TTL sync** — `GvFiles`, `test_stream_trigger` (photodiode + oscilloscope checks).

Each directory is a standalone `main.go`. This folder is its own Go module (see `go.mod`).

```bash
# run a single test from the repo root
go run ./tests/test_av_sync/ -w

# or build them all to _build/
make tests
```

Many tests need specific hardware (parallel port, LabJack, FT232H, photodiode,
oscilloscope) or a real display, so verification is typically manual.

A catalogue with one-line descriptions is in the "Technical Tests" section of
[docs/GalleryOfExamples.md](../docs/GalleryOfExamples.md), generated from the
per-directory `meta.yaml` files via `make update-examples-gallery`.

Reusable experiments and demos live in the sibling [`examples/`](../examples/) directory.
