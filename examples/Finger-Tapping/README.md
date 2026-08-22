# Patterned Finger Tapping (Povel & Collard, 1982)

A replication of the **structural finger-tapping** paradigm. The participant
memorises a short finger sequence, then taps it **6 times in a row** as fast as
possible. The interesting measure is not overall speed but the **pattern of
inter-tap intervals**: people spontaneously segment a sequence into structural
"chunks," and they pause slightly at chunk boundaries. The timing profile of the
taps therefore reveals how the motor sequence was mentally organised.

Each digit names a finger of the dominant hand:

| Key | Finger |
|-----|--------|
| `1` | index |
| `2` | middle |
| `3` | ring |
| `4` | little |

(Uses the top-row digit keys; remap via `fingerKey` / `responseKeys` in `main.go`.)

---

## Trial procedure

```
Show pattern      Get ready     Go            Tap 6×          Stop
+ practise        ready tone    go tone       from memory     stop tone
(until SPACE)     (440 Hz)      (880 Hz)      (no display)    (660 Hz)
```

1. The digit sequence is shown; the participant **practises until it is
   memorised**, then presses SPACE.
2. A **"Ready"** tone (440 Hz), a 1 s pause, then a **"Go"** tone (880 Hz).
3. The screen goes blank and the participant taps the sequence **6 times** from
   memory, as fast as possible.
4. On success, a **"Stop"** tone (660 Hz) sounds and the trial ends.
5. On any **wrong key**, a low **error** buzz (220 Hz) sounds and the whole trial
   restarts (pattern shown again). Only fully error-free runs are saved.

Audio output is required — make sure sound is on.

---

## Stimulus sets

12 experimental patterns in **4 sets** of 3 cyclic permutations each (from
Povel & Collard, 1982, Table 1). The sets differ in how strongly the sequence
affords a chunked structure:

| Set | Base sequence | Structure |
|-----|---------------|-----------|
| **A** | `3 2 1 2 3 4` | run/structured |
| **B** | `1 2 3 2 3 4` | two-chunk |
| **C** | `1 2 3 3 2 1` | repeat (mirror) |
| **D** | `2 4 3 4 2 1` | no clear structure |

Each pattern is 6 taps long. The 12 patterns are presented in **random order**.
A **10-pattern practice block** (shorter 4-tap sequences) precedes the
experiment; practice data are not saved.

See `Table1.png` for the original stimulus table.

---

## Prerequisites

- Go 1.25+
- Audio output (headphones or speakers)

---

## Running

```bash
# Fullscreen, participant 1
go run . -s 1

# Windowed (development / testing)
go run . -s 1 -w
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
companion `-info.txt`. Only the **experiment** phase is recorded — practice trials and aborted
(errored) runs are discarded. One row per tap:

| Column | Description |
|--------|-------------|
| `pattern` | Pattern name (e.g. `A1`, `C3`) |
| `set` | Structural set: `A`, `B`, `C`, or `D` |
| `phase` | Always `experiment` in the saved file |
| `rep` | Repetition number within the trial (1–6) |
| `tap` | Position within the sequence (1–6) |
| `finger_expected` | Finger the sequence called for (1–4) |
| `finger_pressed` | Finger actually pressed (1–4) |
| `t_from_go_ms` | Cumulative time from the "Go" tone to this tap (ms) |
| `iti_ms` | Inter-tap interval (ms); for the very first tap this is the initial RT from "Go" |

Because only error-free trials are saved, `finger_pressed` always equals
`finger_expected` in the output; the columns are kept for completeness and for
verification.

### Analysis

`analyze.py` is included for post-hoc analysis of the inter-tap intervals.

---

## What to expect in the results

Plot the mean **`iti_ms`** by tap position (1–6) within a pattern:

- Intervals are **not uniform** — participants pause slightly at the boundaries
  between chunks, so the interval profile has a characteristic shape.
- More strongly structured sets yield **more regular, organised** timing profiles
  than the unstructured set, illustrating Povel & Collard's central point: the
  *structure* a performer imposes on a sequence shapes its motor timing.

---

## References

Povel, D.-J., & Collard, R. (1982). Structural factors in patterned finger
tapping. *Acta Psychologica*, 52(1–2), 107–123.
https://doi.org/10.1016/0001-6918(82)90029-4

The original paper is included in this directory as a PDF, and the stimulus set
as `Table1.png`.
