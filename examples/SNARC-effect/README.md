# SNARC Effect

Replication of **Experiment 1** from Dehaene, Bossini & Giraux (1993), demonstrating the **SNARC effect** (Spatial-Numerical Association of Response Codes): small numbers are responded to faster with the left hand and large numbers faster with the right hand, regardless of the parity judgment required.

Participants perform a **parity judgment** task (odd or even?) on single digits 0–9. The critical manipulation is the response-key mapping, which is reversed across two blocks.

---

## Trial structure

```
Blank (ITI)  →  Fixation frame  →  Digit in frame  →  Response  →  (next trial)
  1500 ms          300 ms          up to 1300 ms       key press
```

The fixation frame is a rectangular outline (≈ 22 mm × 32 mm) centered on the screen.

---

## Response keys

| Key | Block A | Block B |
|-----|---------|---------|
| `F` (left hand)  | Even | Odd  |
| `L` (right hand) | Odd  | Even |

Block order is counterbalanced by participant ID: even IDs receive Block A first, odd IDs receive Block B first.

---

## Design

- 2 blocks × (12 training + 90 experimental) trials = 204 trials total
- Experimental phase: each digit 0–9 appears exactly 9 times per block
- No two consecutive presentations of the same digit

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

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID (integer) — also determines block order |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file. One row per experimental trial (training trials are not recorded):

| Column | Description |
|--------|-------------|
| `block` | Block name (`A` or `B`) |
| `phase` | `training` or `experimental` |
| `trial` | Trial number within the phase |
| `digit` | Digit presented (0–9) |
| `key` | Key code of the response |
| `rt_ms` | Reaction time in milliseconds (0 on timeout) |
| `correct` | Whether the response was correct |

---

## References

Dehaene, S., Bossini, S., & Giraux, P. (1993). The mental representation of parity and number magnitude. *Journal of Experimental Psychology: General*, 122(3), 371–396. https://doi.org/10.1037/0096-3445.122.3.371
