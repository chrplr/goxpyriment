# examples/

Runnable **experiments and demonstrations** built with `goxpyriment`.

- **Psychological experiments** — complete paradigms that record behavioural data to a `.csv` file (e.g. `parity_decision`, `Stroop_task`, `Mental-Rotation-2D`).
- **Demonstrations** — short programs showing how to use a single feature (visual illusions, minimal templates, widget/stimulus showcases). Demo directories are **prefixed `demo_`** (e.g. `demo_hello_world`, `demo_stimuli_extras`, `demo_menu`) so they stand out among the experiments.

Each directory is a `main.go` program. The whole `examples/` folder is **one** Go
module (this `go.mod`); the individual directories are *not* separate modules —
in-repo they build through the shared module and the workspace `go.work`.

```bash
# from the repo root
go run ./examples/parity_decision/ -w -s 1

# or build everything to _build/
make examples
```

Most programs accept `-w` (windowed), `-d N` (monitor index), and `-s <id>` (subject ID).

## Sharing a single example

To hand one example to a colleague as a **self-contained module** — buildable on
its own, with no `go.work`, no `replace`, and no `go mod init`/`tidy` on their
part — export it:

```bash
make share-parity_decision                 # → _build/share/parity_decision/
make share-parity_decision VERSION=v0.12.3 # pin a specific goxpyriment release
```

This copies the directory and generates a `go.mod`/`go.sum` that require the
**published** `github.com/chrplr/goxpyriment` module (default: the latest git
tag) instead of this repo's shared module. Zip `_build/share/<name>/` and send
it; the recipient just runs `go run .`. The export step needs network the first
time (to fetch the published library and its dependencies into the module
cache). Examples that read sibling files (e.g. shared `.gv` assets via `../`)
are flagged by the exporter — bundle those files manually.

A browsable catalogue with one-line descriptions is in
[docs/GalleryOfExamples.md](../docs/GalleryOfExamples.md); it is generated from
the per-directory `meta.yaml` files via `make update-examples-gallery`.

Hardware and timing/display tests (programs whose results are analysed to check
performance) live in the sibling [`tests/`](../tests/) directory.
