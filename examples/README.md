# examples/

Runnable **experiments and demonstrations** built with `goxpyriment`.

- **Psychological experiments** — complete paradigms that record behavioural data to a `.csv` file (e.g. `parity_decision`, `Stroop_task`, `Mental-Rotation-2D`).
- **Demonstrations** — visual illusions and minimal templates showing a single feature (e.g. `hello_world`, `stimuli_extras`).

Each directory is a standalone `main.go`. This folder is its own Go module (see `go.mod`).

```bash
# from the repo root
go run ./examples/parity_decision/ -w -s 1

# or build everything to _build/
make examples
```

Most programs accept `-w` (windowed), `-d N` (monitor index), and `-s <id>` (subject ID).

A browsable catalogue with one-line descriptions is in
[docs/GalleryOfExamples.md](../docs/GalleryOfExamples.md); it is generated from
the per-directory `meta.yaml` files via `make update-examples-gallery`.

Hardware, timing, and feature tests live in the sibling [`tests/`](../tests/) directory.
