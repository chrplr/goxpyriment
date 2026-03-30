# Letter Height Superiority Illusion

Replication of:

> New, B., Doré-Mazars, K., Cavézian, C., Pallier, C., & Barra, J. (2015).
> **The letter height superiority illusion.**
> *Psychonomic Bulletin & Review*, 22(4), 1010–1018.
> https://doi.org/10.3758/s13423-014-0753-8

Participants compare the heights of two briefly presented stimuli and decide which one is taller (or whether they are the same height). The key finding is that letters are perceived as taller than pseudoletters or mirror-image letters of identical physical size.

---

## Experiments

### Experiment 1 – Letters (`-exp 1`)

| Parameter | Value |
|-----------|-------|
| Stimuli | 9 lowercase letters: **a c e m r s v w z** |
| Controls | Mirror letter (horizontal flip) · Pseudoletter (approximation) |
| Sizes | Small: 0.28° · Tall: 0.30° visual angle |
| Eccentricity | ±2.75° from centre |
| Stimulus duration | 700 ms |
| Total trials | 648 (216 unique × 3 repetitions) |
| Breaks | Every 108 trials |
| Training stimuli | u, n, x |

### Experiment 2 – Words (`-exp 2`)

| Parameter | Value |
|-----------|-------|
| Stimuli | 9 uppercase French words: **BATEAU BUREAU CAMION CANAL GENOU JARDIN LAPIN PARFUM TUYAU** |
| Controls | Mirror word · Reversed-syllable pseudoword · Nonword (approximation) |
| Sizes | Small: 0.40° · Tall: 0.44° visual angle |
| Eccentricity | ±0.90° from centre |
| Stimulus duration | 500 ms |
| Total trials | 864 (288 unique × 3 repetitions) |
| Breaks | Every 144 trials |
| Training stimuli | RADIO, PAPIER, MAISON |

---

## Prerequisites

- Go 1.25+
- A display — ideally a **17-inch 1024×768 monitor at 64 cm viewing distance** to match the original study. On any other setup the visual angles will be approximate.

---

## Running

```bash
# Experiment 1, participant 12, fullscreen
go run main.go -exp 1 -s 12

# Experiment 2, participant 12, fullscreen
go run main.go -exp 2 -s 12

# Windowed 1024×768 (development / testing)
go run main.go -exp 1 -s 0 -w
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-exp` | `1` | Experiment number: `1` (letters) or `2` (words) |
| `-s` | `0` | Participant ID (integer) |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |

---

## Trial structure

```
Fixation cross  →  Two stimuli  →  Blank screen + response  →  ITI
    200 ms           700/500 ms        (wait for key)           750 ms
```

**Response keys**

| Key | Meaning |
|-----|---------|
| ← Left arrow | Left stimulus is taller |
| → Right arrow | Right stimulus is taller |
| ↓ Down arrow | Both stimuli are the same height |

---

## Training

Before the main experiment the participant completes a practice phase using stimuli not shown in the main session (letters **u, n, x** for Exp 1; words **RADIO, PAPIER, MAISON** for Exp 2). Only same-category pairs (letter vs. letter, word vs. word) are used, and feedback (CORRECT / WRONG) is given after each trial. The practice loops until **80 % accuracy** is reached.

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file (CSV with a metadata header). One row per trial:

| Column | Description |
|--------|-------------|
| `trial` | Trial number (1-based) |
| `anchor` | The letter or word always rendered normally |
| `anchor_pos` | Position of the anchor: `left` or `right` |
| `comp_type` | Comparison type: `letter` · `mirror` · `pseudoletter` · `reversed_syllable` · `nonword` |
| `height_cond` | `same_small` · `same_tall` · `anchor_tall` · `anchor_small` |
| `response` | Participant's response: `LEFT` · `RIGHT` · `SAME` |
| `rt_ms` | Reaction time in milliseconds (stimulus offset → key press) |

---

## Notes on stimulus fidelity

- **Font:** Liberation Serif Regular is used as a metric-compatible substitute for Times New Roman (the original font). The font file is embedded in the binary.
- **Mirror stimuli** are produced by horizontally flipping the rendered letter/word texture, which exactly matches the paper's "vertical mirror symmetry transformation".
- **Pseudoletters (Exp 1) and nonwords (Exp 2)** are approximated by *vertically flipping* the source texture. The original study used custom-drawn glyphs carefully matched in height, pixel count, and contiguity. For a faithful replication, obtain the original stimuli from the authors and load them as image files.
- **Reversed-syllable pseudowords** (Exp 2) are the actual text strings from the paper's appendix (TEAUBA, REAUBU, MIONCA, NALCA, NOUGE, DINJAR, PINLA, FUMPAR, YUTAU).
- **Visual angles** are computed for a 17-inch 1024×768 screen at 64 cm viewing distance (75.3 DPI). On a different screen or distance the physical sizes will differ.
