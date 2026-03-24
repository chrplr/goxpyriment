// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

# staircase package

Adaptive psychophysical threshold estimation: classical up-down (Levitt 1971) and Bayesian QUEST (Watson & Pelli 1983), plus a runner for interleaved designs.

## Core interface

```go
type Staircase interface {
    Intensity() float64   // stimulus intensity for the next trial
    Update(correct bool)  // record response, update internal state
    Done() bool           // stopping criterion met?
    Threshold() float64   // current threshold estimate
    History() []Trial     // all trials in order
}
```

Staircases are decoupled from stimulus presentation. The experiment loop calls `Intensity()`, presents the stimulus, records the response, then calls `Update`.

```go
type Trial struct {
    Intensity float64
    Correct   bool
    Reversal  bool  // only used by UpDown
}
```

## UpDown (Levitt 1971)

```go
cfg := staircase.UpDownConfig{
    StartIntensity:          0.5,
    MinIntensity:            0.01,
    MaxIntensity:            1.0,
    StepUp:                  0.1,
    StepDown:                0.05,   // 2:1 up-down ratio targets ~70.7%
    NCorrectDown:            2,       // 2-down-1-up
    MaxReversals:            12,
    NReversalsForThreshold:  6,       // average last 6 reversals
}
sc := staircase.NewUpDown(cfg)
```

### Key behaviors

- Intensity steps **up** on any incorrect response, **down** after `NCorrectDown` consecutive correct responses.
- A **reversal** is recorded whenever direction changes (up→down or down→up).
- Intensity is clamped to `[MinIntensity, MaxIntensity]` on every step.
- `Threshold()` returns the mean of the last `NReversalsForThreshold` reversal intensities (or all reversals if ≤ 0).
- `Done()` returns true when either `MaxReversals` or `MaxTrials` is reached; set either to 0 to disable that criterion.

### Two-phase support

Optional finer step size after an initial exploration phase:

```go
cfg.Phase2StepUp   = 0.02
cfg.Phase2StepDown = 0.01
cfg.Phase2StartReversal = 4  // switch to phase 2 steps after 4 reversals
```

### Extra methods

- `Reversals() []float64` — intensity at each reversal point
- `NReversals() int` — current reversal count

## Quest (Watson & Pelli 1983)

Bayesian adaptive procedure. Places stimulus at the current posterior estimate of threshold.

```go
cfg := staircase.QuestConfig{
    TGuess:        -1.0,    // prior mode (log-contrast or other log-unit)
    TGuessSd:       2.0,    // prior SD (broad = uninformative)
    PThreshold:     0.82,   // target performance level
    Beta:           3.5,    // Weibull slope
    Delta:          0.01,   // lapse rate
    Gamma:          0.5,    // lower asymptote (0.5 for 2AFC, 0 for yes/no)
    IntensityMin:  -4.0,
    IntensityMax:   0.0,
    IntensityStep:  0.01,   // grid resolution
    MaxTrials:      40,
    EstimateMethod: "mean", // "mean" (default) or "mode"
}
sc := staircase.NewQuest(cfg)
```

### Key behaviors

- Discrete grid approximation of the posterior (log-posterior for numerical stability).
- `Update(correct)` multiplies the log-posterior by the Weibull likelihood at the presented intensity.
- `Threshold()` recalculates from the posterior on every call; use sparingly in tight loops.
- `SD()` returns the posterior standard deviation — a natural stopping criterion.
- No reversal concept; `History()[i].Reversal` is always false.
- `NewQuest` panics if `IntensityStep ≤ 0`, `IntensityMin ≥ IntensityMax`, or `TGuessSd ≤ 0`.

### Intensity units

Quest works in any monotonic scale. Log-contrast is conventional (`TGuess = log10(0.1) = -1`). Keep all intensities in the same scale.

## Runner — interleaved staircases

```go
runner := staircase.NewRunner(nil, sc1, sc2, sc3)  // nil = time-seeded RNG
for !runner.Done() {
    sc := runner.Next()           // random non-done staircase
    intensity := sc.Intensity()
    // present stimulus…
    sc.Update(correct)
}
// retrieve thresholds:
for _, sc := range runner.All() {
    fmt.Println(sc.Threshold())
}
```

- `Next()` panics if all staircases are done — always guard with `!runner.Done()`.
- `All()` returns the original staircases in order (same pointers passed at construction).
- Interleaving prevents order effects and keeps multiple threshold estimates independent.

## Key conventions

- Call `Intensity()` once per trial (before presentation); the value is cached until `Update()` is called.
- `UpDown` and `Quest` are not safe for concurrent use from multiple goroutines.
- For Quest, prefer `EstimateMethod: "mean"` — more robust than mode when the posterior is broad.
- Store the full `History()` in your data file for post-hoc analysis.
