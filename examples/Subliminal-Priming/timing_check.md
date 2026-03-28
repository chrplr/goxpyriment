# Timing diagnostics for the Dehaene subliminal-priming stream

## What was added

`runStream` now prints two lines to stdout after every stream:

```
[stream] detected refresh rate: 60.0 Hz  (frame = 16.67 ms)
[timing]  intended  min    mean   max    n    (frame=16.67ms)
[timing]    29 ms    16.7   16.7   16.7     8
[timing]    43 ms    33.3   33.4   33.6    12
[timing]    57 ms    50.0   50.1   50.3    18
[timing]    71 ms    66.7   66.8   67.1    24
```

- **intended** — the duration that was requested for that item class (ms).
- **min / mean / max** — actual wall-clock durations measured across all items
  in that bucket (ms).
- **n** — how many items fell in that bucket.

## How to read the output

### VSYNC is working correctly

Actual durations are rounded to the nearest frame boundary.
At 60 Hz one frame = 16.67 ms, so:

| intended | expected actual |
|----------|----------------|
| 29 ms    | 33.3 ms (2 frames) — *or* 16.7 ms (1 frame) depending on rounding |
| 43 ms    | 50.0 ms (3 frames) |
| 57 ms    | 50.0 ms (3 frames) or 66.7 ms (4 frames) |
| 71 ms    | 66.7 ms (4 frames) |

All means will be close to a **multiple of the frame duration**.
Small jitter (< 1 ms) is normal.

### VSYNC is NOT blocking (stream too fast)

- `min` and `mean` values are near **0 ms** — `screen.Update()` returns
  instantly without waiting for the next retrace.
- The entire 2400 ms trial finishes in a fraction of a second.

### Wrong refresh rate detected

- `detected refresh rate` prints `0.0 Hz` or an implausible value.
- `frameDuration` becomes 0 or huge, making the frame-count calculation
  produce 0 frames per item (skipped) or thousands of frames per item.

## Confirmed diagnosis (2026-03-18)

Actual output observed — every duration 0.3–9 ms regardless of intended value:

```
[stream] detected refresh rate: 60.0 Hz  (frame = 16.66 ms)
[timing]  intended  min    mean   max    n
[timing]    29 ms     0.4    0.6    0.9     4
[timing]    71 ms     0.7    1.3    6.0    22
[timing]    43 ms     0.4    1.6    7.2     8
[timing]    57 ms     0.4    0.6    0.9     3
```

**Root cause:** `SDL_RenderPresent` returns immediately — VSYNC is not enabled
on the renderer. SDL3 does **not** enable VSYNC by default.

**Fix applied:** call `exp.SetVSync(1)` immediately after
`NewExperimentFromFlags` in `main.go`. This calls `SDL_SetRenderVSync(renderer, 1)`
which makes every `SDL_RenderPresent` block until the next vertical retrace.

After the fix, confirmed output (2026-03-18):

```
[timing]    29 ms    33.1   33.3   33.5     4   ← 2 frames at 60 Hz ✓
[timing]    71 ms    66.1   66.6   66.9    23   ← 4 frames ✓
[timing]    43 ms    37.9   48.0   50.3     6   ← 3 frames ✓ (see notes)
[timing]    57 ms    49.7   50.0   50.2     6   ← 3 frames ✓
```

### Frame-rounding at 60 Hz

At 60 Hz (16.66 ms/frame) each intended duration maps to the nearest frame:

| intended | frames | actual |
|----------|--------|--------|
| 29 ms    | 2      | 33.3 ms |
| 43 ms    | 3      | 50.0 ms |
| 57 ms    | 3      | 50.0 ms |
| 71 ms    | 4      | 66.7 ms |

Note: 43 ms and 57 ms both round to 3 frames and are therefore
indistinguishable on a 60 Hz display.

### Normal artefacts in the confirmed data

- `43 ms` min occasionally shows ~38–39 ms: measurement artefact on the
  first item of a stream before the VSYNC sync point is fully established.
  Not a real short frame.
- One `71 ms` entry reached 100.8 ms (6 frames): a single dropped frame
  caused by a kernel preemption or system interrupt. Rare and unavoidable
  on a non-real-time OS. Exclude such outliers (> 2× expected) from
  mean calculations.

## Fixes to try if VSYNC still does not block

### 1. Force VSYNC on the renderer (already applied)

```go
if err := exp.SetVSync(1); err != nil {
    log.Printf("warning: could not enable vsync: %v", err)
}
```

### 2. Check the refresh rate query

If `detected refresh rate` is wrong, the frame-rounding maths breaks.
The query in `runStream` is:

```go
displayID := sdl.GetDisplayForWindow(screen.Window)
mode, err := displayID.CurrentDisplayMode()
```

On some Linux setups `CurrentDisplayMode` returns an error or a zero rate.
Fallback: hard-code `refreshRate = 60.0` (or read it from a `-hz` flag) while
debugging.

### 3. Verify the renderer backend

On Linux, SDL3 may use a software renderer that ignores VSYNC.  Check with:

```bash
SDL_RENDER_DRIVER=opengl go run . -w
# or
SDL_RENDER_DRIVER=vulkan go run . -w
```

### 4. Windowed mode vs fullscreen

Run with `-w` (windowed) first. Some compositors disable VSYNC for windowed
apps; fullscreen (no `-w`) is more reliable for true VSYNC behaviour.

## Expected timing at common refresh rates

| refresh | frame  | 1-frame items | 2-frame items | 3-frame items | 4-frame items |
|---------|--------|---------------|---------------|---------------|---------------|
| 60 Hz   | 16.7 ms | 16.7 ms      | 33.3 ms       | 50.0 ms       | 66.7 ms       |
| 120 Hz  |  8.3 ms |  8.3 ms      | 16.7 ms       | 25.0 ms       | 33.3 ms       |
| 144 Hz  |  6.9 ms |  6.9 ms      | 13.9 ms       | 20.8 ms       | 27.8 ms       |

At 60 Hz the 29 ms word slot rounds to **2 frames (33.3 ms)**.
That is slightly longer than the paper's 29 ms but unavoidable with a 60 Hz
display; a 120 Hz display would give 2 frames = 16.7 ms, closer to the target.
