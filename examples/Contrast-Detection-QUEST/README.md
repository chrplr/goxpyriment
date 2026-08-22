# Contrast Detection Threshold (QUEST)

Estimates the **contrast detection threshold** for a Gabor patch using the
**QUEST** adaptive procedure (Watson & Pelli, 1983). QUEST maintains a Bayesian
posterior over the observer's threshold and, on every trial, picks the contrast
that is most informative given the responses so far — converging efficiently on
the **82 % correct** point (≈ d′ = 1 for a 2-alternative task).

Because contrast is manipulated on a **log₁₀ scale**, a handful of trials spans
several orders of magnitude, from clearly visible down to imperceptible.

---

## Task — 2-Interval Forced Choice (2-IFC)

Each trial presents **two intervals**, marked by the two on-screen boxes labelled
**1** and **2**, which light up in turn. Exactly one interval (chosen at random)
briefly contains a faint tilted grating (the Gabor patch); the other shows only
the fixation cross. The participant reports **which interval contained the
pattern** by pressing **1** or **2**. Brief colour feedback (green = correct,
red = wrong) follows every response.

```
Fixation   Interval 1        ISI    Interval 2        Response      Feedback
(box 0)    box 1 lit         gap    box 2 lit         "1 or 2?"     green/red
500 ms     150 + 50 ms       400 ms 150 + 50 ms       until key     300 ms
```

Within each interval the Gabor is shown for 150 ms, then blanked for 50 ms.

---

## Stimulus

| Parameter | Value |
|-----------|-------|
| Type | Gabor patch (sinusoid × Gaussian envelope) |
| Orientation (θ) | 45° from horizontal |
| Spatial wavelength (λ) | 20 px/cycle (0.05 cycles/px) |
| Envelope SD (σ) | 30 px |
| Patch size | 200 × 200 px |
| Background | mid-gray RGB(128, 128, 128) |
| Contrast | set per trial by QUEST (the manipulated variable) |

The intensity variable tracked by the staircase is **log₁₀(Michelson contrast)**,
so intensity `−2.0` ⇒ 1 % contrast, `−1.0` ⇒ 10 %, `0.0` ⇒ 100 %.

---

## QUEST configuration

| Parameter | Value | Meaning |
|-----------|-------|---------|
| `TGuess` | `-1.5` | Prior mean: initial threshold guess (≈ 3 % contrast) — set with `-guess` |
| `TGuessSd` | `1.5` | Prior SD: wide, ±1.5 log-units of uncertainty |
| `PThreshold` | `0.82` | Criterion performance level QUEST converges on |
| `Beta` | `3.5` | Weibull slope (psychometric steepness) |
| `Delta` | `0.01` | Lapse rate (1 %) |
| `Gamma` | `0.5` | Guess rate / lower asymptote (0.5 for 2-IFC) |
| `IntensityMin … Max` | `-3.0 … 0.0` | Contrast range: 0.1 % … 100 % |
| `IntensityStep` | `0.01` | Resolution of the intensity grid (log-units) |
| `MaxTrials` | `40` | Number of trials — set with `-n` |
| `EstimateMethod` | `"mean"` | Final threshold = posterior mean |

At the end, the program reports the estimated threshold — `log₁₀(contrast)` with
its posterior SD, and the equivalent linear contrast in percent.

---

## Prerequisites

- Go 1.25+

---

## Running

```bash
# Fullscreen, participant 1, 40 trials (defaults)
go run main.go -s 1

# Shorter session
go run main.go -s 1 -n 25

# Different starting guess (e.g. −1.0 ≈ 10 % contrast)
go run main.go -s 1 -guess -1.0

# Windowed (development / testing)
go run main.go -s 1 -w
```

Run from the repository root (or from this directory — `go.work` resolves the
workspace either way).

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID (integer) |
| `-n` | `40` | Number of QUEST trials |
| `-guess` | `-1.5` | Initial log₁₀(contrast) guess (e.g. `-1.5` ≈ 3 %) |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | `-1` | Display ID: monitor index where the window/fullscreen opens (`-1` = primary) |

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file, with the session metadata in a
companion `-info.txt`. One row per trial:

| Column | Description |
|--------|-------------|
| `trial` | Trial number |
| `log_contrast` | log₁₀(contrast) tested on this trial (selected by QUEST) |
| `linear_contrast_pct` | The same contrast expressed as a linear percentage |
| `signal_interval` | Interval (1 or 2) that actually contained the Gabor |
| `response` | Interval (1 or 2) the participant chose |
| `correct` | Whether the response was correct — this is what drives the staircase |
| `quest_threshold` | Running QUEST threshold estimate after this trial (log₁₀ contrast) |
| `quest_sd` | Posterior SD of the threshold estimate (uncertainty) |

The final estimated threshold is the last `quest_threshold` value.

---

## References

Watson, A. B., & Pelli, D. G. (1983). QUEST: A Bayesian adaptive psychometric
method. *Perception & Psychophysics*, 33(2), 113–120.
https://doi.org/10.3758/BF03202828
