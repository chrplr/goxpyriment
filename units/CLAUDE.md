// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

# units package

Vision-science unit conversions. A `Monitor` encodes physical display dimensions and viewing distance; all pixel↔degree↔centimetre conversions derive from it.

## Monitor

```go
// Explicit physical dimensions
m := units.NewMonitor(widthCm, heightCm, widthPx, heightPx, distanceCm)

// From diagonal (inches) — derives width/height from aspect ratio
m := units.NewMonitorFromDiagonal(24.0, 1920, 1080, 60.0)

err := m.Validate()  // returns error if any field ≤ 0
```

## Conversions

### Horizontal (X axis)

```go
px  := m.DegToPx(deg)   // visual angle → pixels
deg := m.PxToDeg(px)    // pixels → visual angle
px  := m.CmToPx(cm)     // centimetres → pixels
cm  := m.PxToCm(px)     // pixels → centimetres
```

### Vertical (Y axis — use when pixels are not square)

```go
px  := m.DegToPxY(deg)
deg := m.PxToDegY(px)
px  := m.CmToPxY(cm)
cm  := m.PxToCmY(px)
```

### Distance only (no pixel density needed)

```go
cm  := m.DegToCm(deg)   // visual angle → physical size at viewing distance
deg := m.CmToDeg(cm)    // physical size → visual angle
```

### Summary statistics

```go
m.PPcmX()          // pixels per cm, horizontal
m.PPcmY()          // pixels per cm, vertical
m.PPI()            // pixels per inch (horizontal)
m.PPD()            // pixels per degree — the canonical vision-science unit
m.HasSquarePixels() // true if X and Y pixel density agree within 0.1%
fmt.Println(m)     // human-readable summary
```

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
