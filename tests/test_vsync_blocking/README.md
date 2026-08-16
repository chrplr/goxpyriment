# VSYNC Blocking Test

Reports what this display and driver actually do, so you know whether a
session's frame timing can be trusted. Run it once on any machine you plan to
collect data on.

## What it measures

Three frame periods, as medians over 120 frames:

| Number | How it is obtained |
|---|---|
| **NOMINAL** | `Screen.FrameDuration()` — derived from the current display mode. What SDL *claims*. |
| **UNAIDED** | `Screen.CalibrateRefresh()` — presents directly with `Update`'s frame pacing bypassed. What `SDL_RenderPresent` does on its own. |
| **PACED** | A normal `Screen.FlipTS()` loop — the path every stimulus goes through. |

It also lists the refresh rates the display offers at its native size, which on
a variable-refresh-rate panel exposes the supported range.

## How to interpret results

- **BLOCKING** — unaided ≈ nominal. The driver honours VSYNC by itself, and no
  hold runs inside `Update` at all, costing nothing. This verdict often comes
  with a count of presents that "came back inside the nominal boundary": those
  *are* blocking presents, landing a fraction of a millisecond early because the
  nominal frame grid and the panel's are never in exact phase. The present is
  still the timestamp anchor, so the offset is constant and cannot accumulate —
  it is reported, not warned about.
- **NON-BLOCKING** — unaided well *below* nominal. `SDL_RenderPresent` returns
  before the retrace (triple/mailbox buffering, or a compositor accepting the
  buffer). Without pacing, stimulus frames would be swallowed before the panel
  scans them out; with it, PACED comes back to nominal. This is common — it has
  been measured on Intel i915 + Wayland driving a well-behaved 120 Hz panel
  (presents returning 6.95 ms apart against an 8.33 ms frame), not only on
  NVIDIA.
- **DROPPING FRAMES** — unaided well *above* nominal. Frames are being lost on
  the way to the panel, typically a compositor throttling an unfocused or
  occluded window. **Pacing cannot fix this**: it enforces a minimum frame
  time, not a maximum. Re-run fullscreen with the window focused.

"Short paced frames" counts frames that still came in under 0.9 × nominal
through the paced path; it should be 0.

## Running the test

From the repository root:

```bash
go run ./tests/test_vsync_blocking        # fullscreen
go run ./tests/test_vsync_blocking -w     # windowed
```

Windowed on a compositing desktop is the worst case and is not representative
of how an experiment runs — always confirm fullscreen before trusting a result.

The same two numbers are recorded automatically in every session's `-info.txt`
(`sys refresh_nominal_hz`, `sys refresh_measured_hz`), so you can also check
after the fact whether a data file was collected under good conditions.
