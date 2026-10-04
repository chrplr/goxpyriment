// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

# staircase package

Adaptive psychophysical threshold estimation: classical up-down (Levitt 1971) and Bayesian QUEST (Watson & Pelli 1983), plus a runner for interleaved designs.

## UpDown (Levitt 1971)

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

## Quest (Watson & Pelli 1983)

Bayesian adaptive procedure. Places stimulus at the current posterior estimate of threshold.

### Key behaviors

- Discrete grid approximation of the posterior (log-posterior for numerical stability).
- `Update(correct)` multiplies the log-posterior by the Weibull likelihood at the presented intensity.
- `Threshold()` recalculates from the posterior on every call; use sparingly in tight loops.
- `SD()` returns the posterior standard deviation — a natural stopping criterion.
- No reversal concept; `History()[i].Reversal` is always false.
- `NewQuest` returns an error if `IntensityStep ≤ 0`, `IntensityMin ≥ IntensityMax`, or `TGuessSd ≤ 0`.

### Intensity units

Quest works in any monotonic scale. Log-contrast is conventional (`TGuess = log10(0.1) = -1`). Keep all intensities in the same scale.

## Runner — interleaved staircases

- `Next()` returns an error if all staircases are done — always guard with `!runner.Done()`.
- `All()` returns the original staircases in order (same pointers passed at construction).
- Interleaving prevents order effects and keeps multiple threshold estimates independent.

## Key conventions

- Call `Intensity()` once per trial (before presentation); the value is cached until `Update()` is called.
- `UpDown` and `Quest` are not safe for concurrent use from multiple goroutines.
- For Quest, prefer `EstimateMethod: "mean"` — more robust than mode when the posterior is broad.
- Store the full `History()` in your data file for post-hoc analysis.
