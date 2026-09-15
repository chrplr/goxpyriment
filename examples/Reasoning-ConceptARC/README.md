# ConceptARC — concept abstraction in the ARC domain

A goxpyriment implementation of the human study in **Moskvichev, Odouard &
Mitchell (2023), *The ConceptARC Benchmark: Evaluating Understanding and
Generalization in the ARC Domain***. ConceptARC is a set of 160 tasks in the
format of Chollet's Abstraction and Reasoning Corpus, organised into 16
*concept groups* (Above and Below, Center, Clean Up, Complete Shape, Copy,
Count, Extend To Boundary, Extract Objects, Filled and Not Filled, Horizontal
and Vertical, Inside and Outside, Move To Boundary, Order, Same and Different,
Top and Bottom 2D, Top and Bottom 3D) of 10 tasks each, plus 16 very simple
"minimal" tasks used as attention checks. Each task has 1–5 demonstration pairs
and 3 test inputs.

```bash
go run ./examples/Reasoning-ConceptARC                       # paper protocol, fullscreen
go run ./examples/Reasoning-ConceptARC -w -s 3               # windowed, subject 3
go run ./examples/Reasoning-ConceptARC -concept Center       # the 10 tasks of one concept group
go run ./examples/Reasoning-ConceptARC -tests 3 -explain     # all 3 test inputs per task, typed explanations
```

---

## Task

Each trial shows, on the left, the **demonstrations** of one task — pairs of
grids, an input and the output it turns into, all following the same hidden
rule — and on the right a **test input**. The participant works out the rule
and applies it by painting the output grid:

- pick a colour in the 10-swatch palette (or press its digit key `0`–`9`), then
  click, or click and drag, over the cells of the output grid;
- `Rows −/+` and `Cols −/+` change the size of the output grid (existing cells
  are kept, new cells are black, up to 30×30);
- `Copy input` starts from a copy of the test input, `Reset` blanks the grid at
  its current size;
- `Submit` compares the grid with the expected output.

The output grid starts blank at the size of the test input. A test input is
**solved** when a submission matches the expected output exactly; the
participant has **three attempts** per test input (unlimited on the training
tasks, which must be solved to move on). After the third wrong attempt the
session moves on without revealing the answer. There is no time limit. `ESC`
ends the session; the results file is flushed after every task.

With `-explain`, each task ends with a text-entry screen asking the participant
to describe, in a sentence or two, the rule they used (the paper collected these
descriptions to filter inattentive participants).

## Session

The default session is the protocol of §5.1 of the paper, 17 tasks:

1. **2 training tasks** — minimal tasks with unlimited attempts, to learn the
   interface;
2. then, in random order, **3 minimal tasks** as attention checks and
   **12 corpus tasks** drawn from 12 different concept groups;

with **one test input per task, chosen at random**. All draws are seeded by the
subject ID (`-s`), so the same subject number always gets the same session and a
session can be rebuilt from the results file.

| Flag | Effect |
|---|---|
| `-concept NAME` | Present the 10 tasks of one concept group (no attention checks); `NAME` as in the file names, e.g. `Center`, `AboveBelow`, `TopBottom3D` |
| `-n N` | Number of corpus tasks in the mixed session (default 12; drawn round-robin over the concepts, so up to 16 covers every concept once) |
| `-tests 1\|3` | One random test input per task (default) or all three, in order |
| `-no-training` | Skip the two training tasks |
| `-explain` | Ask for a typed explanation after each task |
| `-w`, `-d N`, `-s ID` | Windowed mode, display index, subject ID (common to all examples) |

## Tasks

`tasks/` holds the 176 task files (`<Concept>N.json` and `<Concept>Minimal.json`),
copied unchanged from <https://github.com/victorvikram/ConceptARC> (MIT licence,
see `tasks/LICENSE` and `tasks/README.md`) and embedded in the binary with
`//go:embed`. Every file is parsed and validated at start-up (three test inputs,
rectangular grids, colours 0–9); `TestEmbeddedCorpus` checks the corpus
structure (16 concepts × 10 tasks + 16 minimal tasks).

Colours are the ten of the ARC interface: 0 black, 1 blue, 2 red, 3 green,
4 yellow, 5 grey, 6 magenta, 7 orange, 8 sky blue, 9 maroon.

## Results

One CSV row per **action**, plus a `trial_start` and a `trial_end` row per test
input.

| Column | Meaning |
|---|---|
| `trial` | Trial number (1-based) |
| `task`, `concept`, `kind` | Task file stem, its concept group, and `training` / `minimal` / `corpus` |
| `test_index` | Which of the task's three test inputs (0–2) |
| `event` | `trial_start`, `palette`, `paint`, `resize`, `copy_input`, `reset`, `submit`, `click_empty`, `trial_end` |
| `t_ms` | Milliseconds since the onset of the trial |
| `event_ts_ns` | SDL3 hardware timestamp (ns) of the click or key press; for cells painted by dragging, the frame's clock |
| `mouse_x`, `mouse_y` | Cursor position, centre-relative, +Y up, in the 1280×800 logical space |
| `row`, `col`, `old_color`, `new_color` | `paint`: the cell and its colour change; `palette`: `new_color` is the selected colour |
| `rows`, `cols`, `grid` | Size and contents of the output grid after the action (`resize`, `copy_input`, `reset`, `submit`, `trial_start`, `trial_end`). `grid` is one digit per cell, rows separated by `\|`: `0000\|0120\|0000` |
| `attempt`, `correct` | `submit`: attempt number and whether it matched the expected output |
| `n_attempts`, `solved`, `trial_ms`, `explanation` | Summary — filled on the `trial_end` row only. `trial_ms` is the time of the last submission; the explanation screen, if any, comes after it |

`paint` rows are written only when a click or drag actually changes a cell's
colour, so replaying the `paint`, `resize`, `copy_input` and `reset` rows in
order from the `trial_start` grid reconstructs the output grid at every moment
of the trial; the `grid` column on `submit` rows gives each submitted answer
directly.

## Implementation notes

| File | Role |
|---|---|
| `task.go` | Task JSON schema, embedded corpus, validation, session sampling — no SDL, unit-tested |
| `grid.go` | `Grid` type: resize, copy, equality, serialisation — no SDL, unit-tested |
| `render.go` | Layout on a 1280×800 logical canvas, grid ↔ screen geometry, palette, buttons |
| `main.go` | Trial loop (per-frame `PollEvents`, paint/drag state), feedback, data logging |
| `tasks/` | The embedded corpus and its licence |

The screen is laid out on a fixed 1280×800 logical canvas that SDL scales to
the window, so every display shows the same arrangement. Demonstrations are
placed in one column (up to three pairs) or two (four or five); each grid gets
the largest cell size that fits its slot, capped at 24 px.

The editing history is deliberately minimal — no flood fill or rectangle
selection, unlike the original ARC web interface — so that every change to the
answer is a single logged cell edit.

## Reference

- A. Moskvichev, V. V. Odouard & M. Mitchell (2023). *The ConceptARC Benchmark:
  Evaluating Understanding and Generalization in the ARC Domain.*
  [arXiv:2305.07141](https://arxiv.org/abs/2305.07141). Corpus and data:
  <https://github.com/victorvikram/ConceptARC>.
- F. Chollet (2019). *On the Measure of Intelligence.*
  [arXiv:1911.01547](https://arxiv.org/abs/1911.01547) — the ARC domain.

---

<!-- BEGIN:links -->
## Try it without building

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/Reasoning-ConceptARC/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
