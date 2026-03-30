# Magnitude Estimation of Luminance

A classic psychophysical experiment based on Stevens' method of magnitude estimation (Stevens, 1957). Participants assign numbers to perceived brightness, allowing the experimenter to measure the relationship between physical luminance and perceived brightness.

---

## Task

On each trial a gray disk appears briefly on a mid-gray background. The participant assigns it a number that reflects its perceived brightness relative to all other disks seen so far. No reference or "standard" stimulus is given — participants choose their own scale.

---

## Design

| Parameter | Value |
|-----------|-------|
| Background | Mid-gray — RGB(128, 128, 128) |
| Stimulus | Filled gray disk, 5° visual angle diameter |
| Luminance levels | 7 levels: **10, 25, 50, 100, 150, 200, 255** (8-bit gray) |
| Blocks | 5 |
| Trials per block | 7 (one per luminance level, random order) |
| Total trials | 35 |

Each block uses a "shuffled deck" — every luminance level appears exactly once per block in a randomised order.

---

## Trial structure

```
Fixation cross  →  Gray disk  →  Numeric response  →  ITI
    500 ms          1000 ms        (Enter to confirm)    1000 ms
```

The response screen shows the instruction text and a text-input box. Any positive number is accepted (integers or decimals). Invalid entries prompt an error message and the participant is asked to try again.

---

## Prerequisites

- Go 1.25+

---

## Running

```bash
# Fullscreen, participant 1
go run main.go -s 1

# Windowed (testing)
go run main.go -s 1 -w

# Custom viewing distance (default 60 cm)
go run main.go -s 1 -dist 57
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID (integer) |
| `-dist` | `60.0` | Viewing distance in cm — used to compute the disk diameter in pixels |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file (CSV with a metadata header). One row per trial:

| Column | Description |
|--------|-------------|
| `participant_id` | Value passed with `-s` |
| `trial_number` | Sequential trial number (1–35) |
| `block` | Block number (1–5) |
| `stimulus_luminance` | 8-bit gray value of the disk (10–255) |
| `participant_response` | Number entered by the participant |
| `reaction_time_ms` | Time from disk disappearance to Enter key press (ms) |

---

## Note on gamma correction

SDL renders 8-bit RGB values through the monitor's gamma look-up table. The stimulus values (10, 25, 50, 100, 150, 200, 255) are therefore **not** linearly spaced in physical luminance (cd/m²). If you need a true linear luminance scale:

1. Measure the actual luminance of each gray value with a photometer and record it alongside each trial's RGB value, or
2. Apply an inverse-gamma correction to the RGB values before presenting them (e.g. for a typical γ = 2.2, the corrected value is `round(255 × (target_cd/max_cd)^(1/2.2))`).

For relative magnitude estimation the gamma non-linearity does not invalidate the data — it simply means the physical luminance ratios differ from the RGB ratios.

---

## References

Stevens, S. S. (1957). On the psychophysical law. *Psychological Review*, 64(3), 153–181. https://doi.org/10.1037/h0046162
