// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

# units package

Vision-science unit conversions. A `Monitor` encodes physical display dimensions and viewing distance; all pixel↔degree↔centimetre conversions derive from it.

## Typical use

```go
// Build from participant info map (as returned by GetParticipantInfo)
widthCm, _    := strconv.ParseFloat(info["screen_width_cm"], 64)
distanceCm, _ := strconv.ParseFloat(info["viewing_distance_cm"], 64)
mon := units.NewMonitor(widthCm, 0, exp.WindowWidth, exp.WindowHeight, distanceCm)

// Express stimulus size in degrees
fixRadius := mon.DegToPx(0.25)   // 0.25° fixation dot
stimSize  := mon.DegToPx(5.0)    // 5° target

// Express spatial frequency in cycles/pixel from cycles/degree
spatialFreqCpDeg := 2.0
spatialFreqCpPx  := spatialFreqCpDeg / mon.PPD()
```

## Key conventions

- `DegToPx` uses `2 · distance · tan(deg / 2) · (widthPx / widthCm)` — correct for large angles.
- For most experiment stimuli, use horizontal conversions (`DegToPx`, `CmToPx`); switch to Y variants only when pixel density differs between axes.
- `NewMonitorFromDiagonal` assumes the display's physical aspect ratio matches its pixel ratio. Verify against manufacturer specs for non-standard panels.
- `PPD()` is equivalent to `DegToPx(1.0)` and is the value to pass to spatial-frequency computations.
- Always call `Validate()` at experiment startup when monitor parameters come from user input.
