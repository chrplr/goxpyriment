# Geometry triangle completion

Implementation of the two-task paradigm of Hart, Mahadevan & Dillon (2022), which asks how intuitive reasoning about Euclidean geometry develops. 125 children aged 7–12 and 30 adults were tested at the National Museum of Mathematics.

Participants see **fragmented planar triangles** — only the two base corners are drawn — and complete two tasks in a fixed order:

1. **Reasoning task.** Categorical judgements about the missing third corner after a described change to the two visible corners ("What if I increase the distance between the bottom two corners, will the angle size of the top corner get bigger, get smaller, or stay the same size?"). 2 blocks × 8 questions.
2. **Localization task.** Click where the missing top corner is. 49 trials over 7 triangle configurations.

The headline measure is the **scaling exponent** α, computed by the companion `analyse` program (see below).

## Task 1 — reasoning

Eight question types: 4 transformations of the visible corners × 2 properties of the missing corner. The Euclidean-correct answers are:

| Transformation | …will the top corner… (position) | …will its angle size… (angle) |
|---|---|---|
| Base angles **increase** | move **up** | get **smaller** |
| Base angles **decrease** | move **down** | get **bigger** |
| Base distance **increase** | move **up** | stay the **same** |
| Base distance **decrease** | move **down** | stay the **same** |

With base *b* and base angles *L*, *R*, the apex height is *b*·tan*L*·tan*R* / (tan*L* + tan*R*) and the apex angle is 180° − *L* − *R*. So growing both base angles raises the apex and narrows its angle, while lengthening the base merely scales the triangle — the apex rises but its angle is invariant. Every cell above matches the modal adult response in the paper's Fig. 5.

Questions are shuffled within each block and paired with a random triangle from the table below; neither the question type nor the triangle repeats on consecutive trials. No feedback is given.

### Stimuli (paper, Table 1 — scalene)

1 length unit = 1632 px (1920 × 0.85). "Size" is the paper's printed column, which equals the area × 100.

| # | Base (units) | Left angle | Right angle | Size |
|---|---|---|---|---|
| R1 | 0.44 | 32° | 48° | 3.87 |
| R2 | 0.66 | 40° | 32° | 7.80 |
| R3 | 0.77 | 56° | 32° | 13.03 |
| R4 | 0.55 | 32° | 40° | 5.41 |
| R5 | 0.77 | 48° | 56° | 18.82 |
| R6 | 0.66 | 48° | 40° | 10.41 |
| R7 | 0.44 | 40° | 56° | 5.19 |
| R8 | 0.55 | 56° | 48° | 9.60 |

### Demonstration phase

Before the task, the sample triangle (30° base angles, base 0.7 units) is shown as base corners → complete triangle → base corners. Four buttons then **animate** what each of the four possible changes looks like. In the paper the experimenter demonstrated these by hand; an animation is clearer for a 7-year-old. The display stays reachable from every test trial (press `S` or click *show sample changes*), as in the paper.

## Task 2 — localization

One practice trial with the sample triangle, then 49 test trials (7 configurations × 7 repetitions), pseudo-randomised so the same configuration never repeats consecutively. Each trial: click where the missing corner is, then click **Next** (top right) to advance. No feedback.

### Stimuli (paper, Table 2 — isosceles)

| # | Base (units) | Base angles | Leg (units) | Size |
|---|---|---|---|---|
| L1 | 0.90 | 36° | 0.5562 | 14.70 |
| L2 | 0.40 | 36° | 0.2472 | 2.90 |
| L3 | 0.10 | 36° | 0.0618 | 0.18 |
| L4 | 0.04 | 36° | 0.0247 | 0.03 |
| L5 | 0.40 | 45° | 0.2828 | 4.00 |
| L6 | 0.10 | 45° | 0.0707 | 0.25 |
| L7 | 0.04 | 45° | 0.0283 | 0.04 |

The seven **leg lengths** are all distinct — the paper's "seven different side-length values" — and are the x-axis of the α regression. A startup self-check verifies every row's computed area against the paper's printed size and that the seven leg lengths remain distinct, so a typo in the table is caught before any data are collected.

## How the fragments are drawn

At each base corner one segment is drawn along the base and one along the leg, each a fixed **fraction** of that whole side (`-fragment-frac`, default 0.35). The fragment is therefore a similar figure at every triangle size, carrying no size cue that the triangle itself does not carry — which matters, because scale dependence is exactly what the task measures.

The paper is self-inconsistent here: Fig. 1 shows short corner marks, Fig. 3 shows legs running most of the way to the apex. Neither gives a number. The corner-mark reading is used; adjust with `-fragment-frac` if you want longer arms.

The base is horizontal, centred, and its height above the bottom of the canvas is **fixed** for every trial (`-base-y`), so the apex height varies with the triangle — that is the dependent variable.

## Usage

Run from the repo root so `go.work` resolves the workspace. This package has several `.go` files, so name the package — `go run main.go` would compile only part of it.

```bash
# Both tasks, windowed, subject 1
go run ./examples/Geometry-Triangle-Completion -w -s 1

# Just the localization task, fullscreen on the second monitor
go run ./examples/Geometry-Triangle-Completion -task localization -d 1 -s 7

# A short run for checking the display
go run ./examples/Geometry-Triangle-Completion -w -s 999 -blocks 1 -reps 2 -skip-demo
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-task` | `both` | `both`, `reasoning` or `localization` |
| `-blocks N` | `2` | Reasoning: blocks of 8 questions |
| `-reps N` | `7` | Localization: repetitions of each of the 7 configurations |
| `-fragment-frac` | `0.35` | Visible arm length at each corner, as a fraction of that side |
| `-stroke-px` | `4` | Line width of the figures, in logical pixels |
| `-base-y` | `80` | Height of the base above the bottom of the canvas, in logical pixels |
| `-skip-demo` | off | Skip the introduction and demonstration phase |
| `-w` | off | Windowed mode (1024×768) instead of fullscreen |
| `-d N` | −1 | Display index (−1 = primary) |
| `-s N` | `0` | Subject ID (skips the setup dialog) |

Layout uses the paper's canvas exactly — `1920 × 1006` logical pixels, 1 unit = 1632 px, black figures on the pale green field of Fig. 1 — so every pixel value is independent of the real display resolution and comparable with the published data.

## Controls

| Input | Meaning |
|-------|---------|
| Mouse | Click an option, a demo button, the response location, or **Next** |
| `1` `2` `3` | Pick a reasoning option by keyboard |
| `S` | Revisit the sample-changes display during the reasoning task |
| `SPACE` | Leave the sample-changes display; advance instruction screens |
| `ESC` | Abort the session |

The paper had an experimenter enter the child's spoken answer, so the reasoning options respond to both the mouse and the number keys.

## Output data

Under the results directory (`goxpy_data/` by default). Age and gender are collected in the setup dialog and repeated as columns in both CSVs so each file is self-contained.

**`Geometry-Triangle-Completion_sub-NNN_date-….csv`** — reasoning, one row per trial: `age_years, age_months, gender, block, trial, triangle_id, base_units, left_angle_deg, right_angle_deg, area_units2, question_type, dimension, transformation, direction, selected_option, correct_option, is_correct, rt_ms, onset_ns`.

**`…-localization.csv`** — localization, one row per trial: `subject_id, age_years, age_months, gender, trial, config_id, base_units, base_angle_deg, side_length_units, true_vertex_x, true_vertex_y, clicked_x, clicked_y, error_x, error_y, rt_ms, onset_ns`.

`error_y = y_true − y_clicked` in the paper's sign convention, so a **positive** value means the click fell short of the true vertex — the underestimation the paper reports.

Coordinates are written in the paper's screen-pixel frame (origin top-left, 1920 × 1006, +Y down) for direct comparability with the published data; internally the code uses the goxpyriment centre-based convention with +Y up. Reaction times and onsets come from the SDL hardware event clock, not wall-clock deltas, so responses and stimulus onsets are on one clock.

`…-info.txt` carries the session metadata plus the geometry actually used.

## Analysis — the scaling exponent

```bash
go run ./examples/Geometry-Triangle-Completion/analyse \
    goxpy_data/Geometry-Triangle-Completion_sub-001_date-…-localization.csv
```

For each configuration it reports the number of trials, the mean signed `error_y`, the SD of the clicked *y*, and the median RT; then it regresses log(SD_y) on log(side length) across the seven configurations. The slope is the **scaling exponent α**:

- **α ≈ 0.5** — the local noise of the extrapolation is strongly corrected, preserving the scale-invariant angle information of Euclidean geometry.
- **α ≈ 1.0** — the noise accumulates uncorrected.
- The paper's children produced a median α of 0.83 (95% CI [0.80, 0.86], range [0.56, 1.14]).

Pass `-o summary.csv` to also write the per-configuration table, and several files at once to summarise a batch.

This is the measurable half of the paper's correlated-random-walk model. The full stochastic model (equations 1–3, with τ, ξ and v_p) is **not** implemented: the paper publishes no fitted values for those parameters, so a simulator would have no defensible settings. α is the quantity the paper actually reports per participant.

## References

Hart, Y., Mahadevan, L., & Dillon, M. R. (2022). Euclid's Random Walk: Developmental changes in the use of simulation for geometric reasoning. *Cognitive Science*, 46, e13070. https://doi.org/10.1111/cogs.13070

Hart, Y., Dillon, M. R., Marantan, A., Cardenas, A. L., Spelke, E., & Mahadevan, L. (2018). The statistical shape of geometric reasoning. *Scientific Reports*, 8, 12906.

Dillon, M. R., & Spelke, E. S. (2018). From map reading to geometric intuitions. *Developmental Psychology*, 54(7), 1304–1316.

Izard, V., Pica, P., Spelke, E. S., & Dehaene, S. (2011). Flexible intuitions of Euclidean geometry in an Amazonian indigene group. *PNAS*, 108(24), 9782–9787.

---

<!-- BEGIN:links -->
## Try it without building

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/Geometry-Triangle-Completion/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
