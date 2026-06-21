# VSYNC Blocking Test

This diagnostic test compares the behavior of `FlipTS()` and `PacedFlipTS()` on your system. It automatically determines whether your platform's display driver blocks on VSYNC or returns immediately (which can cause frame swallowing in tight rendering loops).

## What it does

1.  Runs a loop of 60 frames using `FlipTS()`, measuring the elapsed time between consecutive frame presentations.
2.  Runs a loop of 60 frames using `PacedFlipTS()`, measuring the same.
3.  Calculates and displays the average frame intervals and provides a recommendation.

## Running the test

From the repository root, run:

```bash
go run tests/test_vsync_blocking/main.go -w
```

*Note: Use `-w` for windowed mode or omit it to run in fullscreen mode.*

## How to interpret results

-   **BLOCKING (VSYNC behaves normally):** Both `FlipTS()` and `PacedFlipTS()` yield intervals close to your monitor's nominal frame duration (e.g., ~16.6 ms on a 60 Hz monitor). You can safely use either function.
-   **NON-BLOCKING (Triple/mailbox buffering or compositor active):** `FlipTS()` returns almost instantly (averaging < 2 ms), whereas `PacedFlipTS()` enforces pacing and stays close to the monitor's refresh period. On such platforms, you **must** use `PacedFlip()` or `PacedFlipTS()` inside tight rendering loops to avoid skipping/swallowing frames.
