# Two-Digit Number Comparison

A replication of **Experiments 1 and 2 of Dehaene, Dupoux & Mehler (1990)**. On
each trial a two-digit number appears and the participant decides, as fast as
possible, whether it is **larger or smaller** than a fixed standard. The task
measures the **distance effect** — reaction time *decreases* as the numerical
distance between the target and the standard grows (comparing 92 vs 55 is faster
than 56 vs 55) — evidence that numbers are compared on an analogue "mental number
line" rather than digit-by-digit.

---

## The two experiments

| | Experiment 1 | Experiment 2 |
|---|---|---|
| Standard | **55** | **65** |
| Target range | 11–99 (excluding 55) | 31–99 (excluding 65) |
| Repetitions | 4× for 41–69, 2× otherwise | 4× for every number |
| Experimental trials | **232** | **272** |
| Response mapping | fixed (right = larger) | between-subjects: `LR` or `LL` |

Each experiment is preceded by a **10-trial practice block** (sampled from the
same targets).

Experiment 2 varies the **response mapping** as a between-subjects factor, to
check that any RT discontinuities come from the numbers themselves rather than
from a particular hand assignment:

- **`LR`** — right hand (**J**) = larger, left hand (**F**) = smaller
- **`LL`** — left hand (**F**) = larger, right hand (**J**) = smaller

Experiment 1 always uses the `LR` mapping.

---

## Trial procedure

```
ISI (blank)    Target number        Response window        (pad to 2 s)
2000 ms        shown up to 2000 ms   F or J                 keeps trial = 4 s
```

- The target number is shown for up to **2000 ms**; the trial ends on a keypress
  (or on timeout after 2 s).
- The standard is displayed permanently at the top of the screen, and a response
  reminder ("F = LARGER … J = SMALLER …") at the bottom.
- Each trial occupies a fixed **4 s** (2 s stimulus window + 2 s ISI): after an
  early response the screen blanks for the remainder, so the pace is constant.
- Reaction time is measured from the number's VSYNC onset timestamp to the key
  event's hardware timestamp (SDL-clock, sub-millisecond).

---

## Pseudorandomisation

The main block is shuffled subject to two constraints (up to 2000 attempts):

1. The same target number never appears on two consecutive trials.
2. No more than **3** consecutive trials require the same response direction
   (larger vs. smaller).

The shuffle is seeded from the subject ID, so a given participant gets a
reproducible order.

---

## Prerequisites

- Go 1.25+

---

## Running

```bash
# Experiment 1 (standard = 55), participant 1
go run . -exp 1 -s 1

# Experiment 2 (standard = 65), LR mapping (right = larger)
go run . -exp 2 -group LR -s 2

# Experiment 2, LL mapping (left = larger)
go run . -exp 2 -group LL -s 3

# Windowed (development / testing)
go run . -exp 1 -s 1 -w
```

Run from the repository root (or from this directory — `go.work` resolves the
workspace either way).

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-exp` | `1` | Experiment: `1` (standard 55) or `2` (standard 65) |
| `-group` | `LR` | Response mapping for Exp 2: `LR` (right = larger) or `LL` (left = larger); ignored for Exp 1 |
| `-s` | `0` | Participant ID (integer; also seeds the trial order) |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | `-1` | Display ID: monitor index where the window/fullscreen opens (`-1` = primary) |

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file (with a `#`-prefixed metadata
header). One row per trial:

| Column | Description |
|--------|-------------|
| `exp` | Experiment number (1 or 2) |
| `group` | Response mapping (`LR` or `LL`) |
| `standard` | The fixed standard (55 or 65) |
| `block` | `0` = practice, `1` = main block |
| `is_training` | `true` on practice trials |
| `target` | The two-digit number shown |
| `distance` | Absolute numerical distance \|target − standard\| |
| `response` | Key pressed: `F`, `J`, or `timeout` |
| `rt_ms` | Reaction time in milliseconds (`0` on timeout) |
| `correct` | Whether the response matched target vs. standard |

---

## What to expect in the results

Restrict to correct, non-training trials and plot **mean RT against `distance`**:

- **Distance effect** — RT falls as distance increases; targets far from the
  standard are judged fastest.
- The effect is orthogonal to the response mapping (`LR` vs. `LL` in Exp 2),
  confirming it reflects magnitude comparison rather than motor assignment.

---

## References

Dehaene, S., Dupoux, E., & Mehler, J. (1990). Is numerical comparison digital?
Analogical and symbolic effects in two-digit number comparison. *Journal of
Experimental Psychology: Human Perception and Performance*, 16(3), 626–641.
https://doi.org/10.1037/0096-1523.16.3.626

The original paper is included in this directory as a PDF.
