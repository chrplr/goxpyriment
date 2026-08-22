# Mouse Tracking — Phonological Competition (Spivey et al., 2005)

A mouse-trajectory paradigm that uses the **continuous path of the hand** as a
readout of how a spoken/written word is recognised over time. Participants click
a start box and immediately begin moving the cursor upward toward one of two
labelled boxes; the target word appears only *after* they have started moving. If
a distractor's name overlaps phonologically with the target, the cursor is
**continuously attracted** toward it mid-flight — the trajectory bows toward the
competitor before curving back to the target. That curvature is the signature
that lexical candidates compete in real time rather than being resolved all at
once.

> **This is a written-word adaptation.** The original Spivey et al. (2005) study
> used **spoken** target words and **pictures**. This version substitutes a
> **written** target word and **text-labelled boxes** (no audio, no image files),
> keeping the trajectory logic identical. See `description.md` for the full
> specification.

---

## Conditions

| Condition | Distractor relation | Example (target → distractor) |
|-----------|---------------------|-------------------------------|
| **cohort** | shares word onset with the target | candle → cand**y** |
| **control** | phonologically unrelated | candle → jacket |

Fixed word set (each target appears in both conditions):

| Target | Cohort distractor | Control distractor |
|--------|-------------------|--------------------|
| candle | candy | jacket |
| bear | beard | lamp |
| cat | carrot | table |
| fork | forest | pencil |

---

## Layout & trial procedure

```
        [ left box ]                 [ right box ]        ← two labelled boxes (top)



              [ CLICK HERE TO START ]  /  target word     ← bottom-center
```

1. A green **start box** appears at the bottom-center. The trial begins when the
   participant clicks inside it.
2. The two labelled boxes appear immediately (top-left and top-right).
3. After a **500 ms** delay, the **target word** appears at the bottom-center.
4. Participants are told to start moving the cursor **upward as soon as they
   click start** — before the word appears — then click the matching box.
5. The trial ends on a click inside either box (or a timeout at 8 s). A wrong
   click triggers a buzzer; a timeout shows "Too slow!".

Target side (left vs. right) is counterbalanced across trials.

---

## Design

- 4 cohort pairs + 4 control pairs
- each presented twice (target on the left, then on the right)
- **16 trials** total, shuffled.

---

## Trajectory recording

The cursor position is sampled at **~36 Hz** (every 28 ms) from image onset until
the response, in the center-relative coordinate system (`(0,0)` = screen centre).
Each sample is `(x, y, t)` where `t` is milliseconds from image onset. The whole
path is stored in the `trajectory` column as semicolon-separated triples:

```
x,y,t;x,y,t;x,y,t;…      e.g.  0.0,-290.0,0;12.3,-250.1,28;…
```

A typical trial yields ~30–60 samples. For analysis, resample each trajectory to
a fixed number of equally time-spaced points (e.g. 101) and compute curvature
(area between the actual path and the straight start→end line) or proximity to
each box over time.

---

## Prerequisites

- Go 1.25+
- A mouse (trackpads work but change the trajectory dynamics)

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
| `condition` | `cohort` or `control` |
| `target` | The target word shown at the bottom |
| `left_item` | Label of the left box |
| `right_item` | Label of the right box |
| `target_side` | Which box was the target: `left` or `right` |
| `response` | Label of the box clicked (empty on timeout) |
| `correct` | Whether the clicked box matched the target |
| `rt_ms` | Time from image onset to the click, in ms (`-1` on timeout) |
| `trajectory` | Full cursor path: `x,y,t;…` (center-relative coords, ms) |

Timing (`rt_ms` and the trajectory timestamps) is measured on the millisecond
Go clock from image onset — appropriate for the trajectory-shape analysis this
paradigm relies on.

---

## What to expect in the results

Resample and average the trajectories per condition:

- **Control** trials → paths run fairly **straight** from the start box to the
  target box.
- **Cohort** trials → paths **curve toward the competitor** before homing on the
  target, producing greater curvature / area-under-the-curve.

The difference in curvature between cohort and control is the continuous-attraction
effect: phonological competitors pull the hand mid-movement.

---

## References

Spivey, M. J., Grosjean, M., & Knoblich, G. (2005). Continuous attraction toward
phonological competitors. *Proceedings of the National Academy of Sciences*,
102(29), 10393–10398. https://doi.org/10.1073/pnas.0503903102

Maldonado, M., Dunbar, E., & Chemla, E. (2019). Mouse tracking as a window into
decision making. *Behavior Research Methods*, 51(3), 1085–1101.
https://doi.org/10.3758/s13428-018-01194-x

Both papers are included in this directory as PDFs.
