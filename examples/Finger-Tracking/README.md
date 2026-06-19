# Finger Tracking

Reproduction of the **number-to-position** task from Dotan & Dehaene (2013), *How do we convert a number into a finger trajectory?*

The participant drags a finger (or the mouse, with the left button **held**) from a small box at the bottom of the screen up to a horizontal, unmarked number line at the top, aiming at the screen position that corresponds to a target number between **0 and 40**. The entire pointer trajectory is sampled and recorded, revealing how the quantity representation unfolds over the course of the movement.

On a touchscreen, SDL synthesises the same held-button mouse events from a single finger drag, so one code path serves both mouse and touch.

---

## Trial structure

```
Blank (ITI)  →  Press & HOLD in start box  →  Fixation cross  →  Slide up
  800 ms                                         (above line)
        →  Target number appears (at 70 px from bottom)  →  Cross the line
                                                              ↓
                                          Click sound + green feedback arrow (700 ms)
```

1. The number line (labelled **0** at the left end, **40** at the right) and a dark-grey start rectangle are shown.
2. Pressing and **holding** the button inside the start rectangle begins the trial and shows a fixation cross above the middle of the line.
3. As the pointer slides upward and reaches **70 px from the bottom** of the screen, the target number replaces the fixation cross. This is the **movement onset**.
4. When the pointer **crosses the number line**, the target disappears, a click sound plays, and a green downward arrow marks where the line was crossed.

---

## Layout

Pixel-exact to the paper's iPad geometry, at logical 1024 × 768 (coordinates are screen-center relative, +Y up):

| Element | Geometry |
|---|---|
| Number line | white, 844 × 2 px, 80 px below the top edge (y = +304) |
| End labels | "0" / "40", light grey, at the line ends (x = ∓422) |
| Start rectangle | dark grey, 60 × 40 px, bottom-center (y = −360) |
| Fixation cross / target | center, just above the line (y = +340) |
| Onset threshold | 70 px from the bottom (y = −314); 618 px below the line |

---

## Design

- Each target number 0–40 (41 values) is presented `reps` times, in a single global random order.
- Default `-reps 2` → 82 trials. The paper used **10 repetitions → 410 trials** (`-reps 10`).

---

## Prerequisites

- Go 1.25+
- A pointing device that supports a continuous drag (mouse or touchscreen).

---

## Running

```bash
# Windowed (recommended for development / a mouse)
go run main.go -s 1 -w

# Fullscreen, participant 1, faithful 410-trial run
go run main.go -s 1 -reps 10
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID (integer) |
| `-reps N` | `2` | Repetitions of each target number 0–40 (paper uses 10 → 410 trials) |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file, one row per trial:

| Column | Description |
|--------|-------------|
| `trial` | Trial number |
| `target` | Target number presented (0–40) |
| `endpoint` | Position where the pointer crossed the line, on the 0–40 scale |
| `endpoint_bias` | `endpoint − target` (positive = rightward bias) |
| `endpoint_error` | Absolute value of the bias |
| `target_onset_ms` | Time of movement onset, in ms from the start press |
| `movement_time_ms` | Time from target onset to line crossing |
| `completed` | `true` if the line was crossed; `false` if the button was released early or the trial timed out |
| `trajectory` | Full pointer path as `x,y,t;x,y,t;…` (t in ms from the start press) |

The trajectory is sampled at ~125 Hz. The paper resamples each trajectory to a fixed 100 Hz via cubic-spline interpolation; that step is left to offline analysis.

---

## Scope (v1)

This version records data only. The paper's trial-validity rules — no backward / sideways movement, minimum (6 mm/s) and average velocity limits — and the re-presentation of failed trials are **not** enforced; such filtering can be applied offline.

---

## References

Dotan, D., & Dehaene, S. (2013). How do we convert a number into a finger trajectory? *Cognition*, 129(3), 512–529. https://doi.org/10.1016/j.cognition.2013.07.007
