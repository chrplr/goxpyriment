# Visual Search

A replication of the classic **Treisman & Gelade (1980)** visual-search paradigm,
the empirical foundation of **Feature Integration Theory**. Participants decide,
as fast as possible, whether a **red T** is present among an array of letters. By
comparing how reaction time scales with the number of items (the *set size*) in
two different search tasks, the experiment dissociates **parallel** ("pop-out")
from **serial** (attention-demanding) search.

---

## The two search tasks

| Task | Target | Distractors | Expected behaviour |
|------|--------|-------------|--------------------|
| **Feature** | Red **T** | Blue **T** | Target is the only red item → it *pops out*; search is parallel |
| **Conjunction** | Red **T** | Blue **T** *and* Red **L** | Target shares a feature with every distractor → attention must *bind* colour + shape; search is serial |

In the conjunction task the distractors are split roughly evenly between blue Ts
and red Ls, so neither colour nor shape alone identifies the target.

---

## Trial procedure

```
ITI (blank)   Fixation      Search array         Response         Feedback
1000 ms       cross 500 ms  until key press      J = present      "Correct!" /
                            (up to no limit)     F = absent       "Incorrect" /
                                                                  "Too slow!"  500 ms
```

- Items are placed on a **jittered 6 × 4 grid** (±20 px random offset per item),
  so a perfect alignment never gives away the layout.
- **J** = "target present", **F** = "target absent".
- Feedback is `Too slow!` when RT > 2000 ms, otherwise `Correct!` / `Incorrect`.
- Reaction time is measured from the array's VSYNC onset timestamp to the key
  event's hardware timestamp (SDL-clock, sub-millisecond).

---

## Design

A fully-crossed **2 × 2 × 3** factorial, 20 repetitions per cell, shuffled:

| Factor | Levels |
|--------|--------|
| Task | feature, conjunction |
| Target presence | present (50 %), absent (50 %) |
| Set size | 4, 12, 24 items |
| Repetitions per cell | 20 |

2 × 2 × 3 × 20 = **240 trials** total, presented in random order.

---

## Prerequisites

- Go 1.25+

---

## Running

```bash
# Fullscreen, participant 1
go run main.go -s 1

# Windowed (development / testing)
go run main.go -s 1 -w
```

Run from the repository root (or from this directory — `go.work` resolves the
workspace either way).

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID (integer) |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | `-1` | Display ID: monitor index where the window/fullscreen opens (`-1` = primary) |

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file, with the session metadata in a
companion `-info.txt`. One row per trial:

| Column | Description |
|--------|-------------|
| `trial` | Trial number (1-based) |
| `task` | `feature` or `conjunction` |
| `set_size` | Number of items in the array (4, 12, or 24) |
| `target_present` | `true` if a red T was in the array |
| `response` | `present` (J) or `absent` (F) |
| `rt_ms` | Reaction time in milliseconds |
| `correct` | Whether the response matched target presence |

A live per-trial summary is also printed to the terminal.

---

## What to expect in the results

Compute the **mean RT of correct trials** and plot it against set size:

- **Feature search** → roughly **flat** (slope ≈ 0 ms/item): the red T pops out
  regardless of how many distractors are present.
- **Conjunction search** → **linear increase** (slope ≈ 20–30 ms/item): each
  added item costs a fixed increment of serial attention.
- Within conjunction search, the **target-absent** slope is typically about
  **twice** the target-present slope — the observer must inspect every item
  before concluding the target is absent.

---

## References

Treisman, A. M., & Gelade, G. (1980). A feature-integration theory of attention.
*Cognitive Psychology*, 12(1), 97–136.
https://doi.org/10.1016/0010-0285(80)90005-5

The original paper is included in this directory as a PDF.
