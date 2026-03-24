// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

# geometry package

Math helpers for screen coordinates: Euclidean distance, polar↔Cartesian conversion, and degree→radian.

All functions operate on SDL's `sdl.FPoint` or `float32` values. Angles are in **degrees** unless noted.

## Functions

```go
// Euclidean distance between two points
dist := geometry.GetDistance(p1, p2 sdl.FPoint) float32

// Cartesian → polar (angle in degrees, measured from +X axis)
r, angleDeg := geometry.CartesianToPolar(x, y float32)

// Polar → Cartesian (angle in degrees)
x, y := geometry.PolarToCartesian(r, angleDeg float32)

// Degrees → radians (returns float64 despite float32 input)
rad := geometry.DegreeToRadian(deg float32) float64
```

## Conventions

- Angles are measured from the positive X axis, counter-clockwise, matching standard mathematical convention (not screen convention where Y increases downward).
- `CartesianToPolar` uses `atan2(y, x)` — the angle is in `(-180, 180]` degrees.
- `PolarToCartesian` converts the degree input to radians internally via `DegreeToRadian`.
- `GetDistance` uses the center-based coordinate system that all goxpyriment stimuli use; pass `sdl.FPoint` positions directly.

## Typical use

```go
// Is a click within a circular target?
clickPos := sdl.FPoint{X: mx, Y: my}
stimPos  := stim.GetPosition()
if geometry.GetDistance(clickPos, stimPos) < targetRadius {
    // hit
}

// Place 8 stimuli evenly around a circle of radius 200
for i := 0; i < 8; i++ {
    angle := float32(i) * 45.0
    x, y  := geometry.PolarToCartesian(200, angle)
    stims[i].SetPosition(sdl.FPoint{X: x, Y: y})
}
```
