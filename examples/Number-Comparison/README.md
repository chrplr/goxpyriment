# Number Comparison

Replication of the study by Buckley & Gillman (1974) investigating whether digits and dot patterns are compared numerically through the same cognitive process.

Participants are shown two stimuli side-by-side and press a button to indicate which is numerically larger. The key predictions are:

- **Distance effect (Moyer-Landauer):** reaction time decreases as the numerical difference between the two values increases.
- **Min effect:** reaction time increases as the smaller value increases.
- **Format comparison:** digits are processed faster than dot patterns; irregular dot patterns are slower than random ones.

## Stimulus types

| Group | Description |
|-------|-------------|
| Digits | Two Arabic digits (1–9) |
| Regular dots | Bisymmetric dot patterns (like playing cards) |
| Irregular dots | Unique fixed dot arrangement per number |
| Random dots | Dot configuration changes on every trial |

## Usage

**Step 1 — generate dot-pattern assets** (once, before building):

```bash
cd examples/Number-Comparison
make stimuli
# or manually:
go run ./cmd/generate_regular/
go run ./cmd/generate_irregular/
```

**Step 2 — run the experiment:**

```bash
# Digits group, windowed mode, subject 1
go run . -group digits -w -s 1

# Regular dot patterns
go run . -group regular -w -s 2

# Irregular dot patterns (fixed layout per number)
go run . -group irregular -w -s 3

# Random dot patterns (new layout each trial)
go run . -group random -w -s 4
```

**Response keys:** F = left stimulus is larger, J = right stimulus is larger.

**Data:** written to `goxpy_data/<subject_id>.csv` (tab-separated; one row per trial).

| Column | Description |
|---|---|
| `block` | Block number (0 = practice) |
| `is_practice` | true for block 0 |
| `group` | digits / regular / irregular / random |
| `n_left` | Numerosity of left stimulus (1–9) |
| `n_right` | Numerosity of right stimulus (1–9) |
| `response` | F, J, or timeout |
| `rt_ms` | Reaction time in milliseconds (0 on timeout) |
| `correct` | true / false |

**Design:** 36 pairs × 2 positions = 72 trials/block × 11 blocks (block 0 practice).

## Stimulus assets

| Directory | Contents | Generator |
|---|---|---|
| `assets/regular/` | 9 playing-card-style PNGs | `cmd/generate_regular/` |
| `assets/irregular/` | 9 fixed random-layout PNGs | `cmd/generate_irregular/` |

Random-condition stimuli are generated on-the-fly at runtime (no files needed).

## References

Buckley, P. B., & Gillman, C. B. (1974). Comparisons of digits and dot patterns. *Journal of Experimental Psychology*, 103(6), 1131–1136. https://doi.org/10.1037/h0037361
