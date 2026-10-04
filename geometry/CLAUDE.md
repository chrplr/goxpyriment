// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

# geometry package

Math helpers for screen coordinates: Euclidean distance, polar↔Cartesian conversion, and degree→radian.

All functions operate on SDL's `sdl.FPoint` or `float32` values. Angles are in **degrees** unless noted.

## Conventions

- Angles are measured from the positive X axis, counter-clockwise, matching standard mathematical convention (not screen convention where Y increases downward).
- `GetDistance` uses the center-based coordinate system that all goxpyriment stimuli use; pass `sdl.FPoint` positions directly.

