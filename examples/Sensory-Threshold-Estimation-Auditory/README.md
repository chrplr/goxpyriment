# Adaptive Auditory Threshold Estimation

Measures pure-tone hearing thresholds across multiple frequencies using a **1-up/2-down adaptive staircase** combined with a **2-Interval Forced Choice (2-IFC)** paradigm.

The staircase "hunts" for the lowest intensity at which a participant can reliably detect a tone (approximately the 70.7 % detection threshold). Staircases for all tested frequencies are **interleaved** — the frequency is chosen at random on each trial so the participant cannot anticipate the pitch.

---

## Task

On each trial two 500 ms intervals are presented, separated by a 400 ms gap. One interval contains a pure-tone beep; the other is silence. The participant presses **1** or **2** to indicate which interval contained the tone. Brief colour feedback (green/red) follows each response.

---

## Staircase logic (1-up / 2-down)

| Event | Effect |
|-------|--------|
| 1 miss | Increase level by one step (louder) |
| 2 consecutive hits | Decrease level by one step (quieter) |

**Step sizes:**
- Phase 1 (first 2 reversals): 4 dB
- Phase 2 (all subsequent reversals): 2 dB

**Termination:** each staircase stops after **8 reversals**.

**Threshold:** mean intensity at the **last 4 reversals**.

---

## Design

| Parameter | Default value |
|-----------|---------------|
| Frequencies | 50, 250, 500, 1000, 2000, 4000, 8000 Hz |
| Starting level | −20 dBFS |
| Tone duration | 500 ms (with 50 ms fade-in/fade-out) |
| Interval 1 duration | 500 ms |
| Gap between intervals | 400 ms |
| Interval 2 duration | 500 ms |
| Phase 1 step | 4 dB |
| Phase 2 step | 2 dB |
| Reversals to terminate | 8 per frequency |

---

## Audio safety

**Start with a low system volume.** The program begins at −20 dBFS (roughly 10 % of digital full-scale). Raise the volume only if the tones are completely inaudible at that level. Sustained exposure to high volumes can damage hearing.

---

## Prerequisites

- Go 1.25+
- Headphones or calibrated speakers

---

## Running

```bash
# Default frequencies, participant 1, fullscreen
go run main.go -s 1

# Custom frequency list
go run main.go -s 1 -freqs "250,500,1000,2000,4000"

# Custom starting level
go run main.go -s 1 -start -30

# Windowed (development / testing)
go run main.go -s 1 -w
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID (integer) |
| `-freqs` | `"50,250,500,1000,2000,4000,8000"` | Comma-separated list of frequencies in Hz |
| `-start` | `-20.0` | Starting intensity in dBFS (e.g. `-30` for quieter start) |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file (CSV with a metadata header). One row per trial:

| Column | Description |
|--------|-------------|
| `frequency_hz` | Frequency of the tested tone (Hz) |
| `trial_number` | Sequential trial number |
| `current_intensity_db` | Tone level presented on this trial (dBFS) |
| `response_correct` | Whether the participant identified the correct interval |
| `reversal_occurred` | Whether this trial produced a staircase reversal |
| `final_threshold_db` | Estimated threshold (dBFS) — filled only on the last trial of each frequency, `NA` otherwise |

---

## References

Levitt, H. (1971). Transformed up-down methods in psychoacoustics. *Journal of the Acoustical Society of America*, 49(2B), 467–477. https://doi.org/10.1121/1.1912375
