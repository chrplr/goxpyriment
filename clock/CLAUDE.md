// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

# clock package

Timing utilities: a global elapsed-time reference and a per-instance `Clock` for local timing windows.

## SleepUntil

`SleepUntil(target)` blocks until the clock has reached `target`. It is a `time.Sleep` loop, not a polling loop: it sleeps for the remaining time and re-checks, so it **never returns early, only late** — if `target` is already in the past, it returns immediately. How late is up to the OS: worst `time.Sleep` overshoot measured on one Linux host (400 sleeps at each of 1, 5, 10, 14 ms) was 0.83 ms at normal priority and 0.73 ms at `SCHED_FIFO` 50 — see `spinTailNS` in `apparatus/screen_present_notjs.go`. In the browser build `time.Sleep` goes through the JavaScript event loop's timers, whose lateness has not been measured here; do not assume the desktop figure.

Useful pattern for fixed-interval trial timing without drift accumulation:

```go
c := clock.NewClock()
for i, trial := range trials {
    target := time.Duration(i) * 500 * time.Millisecond
    c.SleepUntil(target)  // start trial at exact offset
    presentTrial(trial)
}
```

## Key conventions

- `GetTime()` and `Clock.NowMillis()` both return `int64` milliseconds; prefer `Clock` when you need sub-millisecond accuracy via `time.Duration`.
- For VSYNC-locked loops, rely on `Screen.Update()` blocking on VSYNC rather than `SleepUntil` — the display refresh is the authoritative clock for frame-by-frame timing.
- Use `SleepUntil` for inter-trial intervals and non-VSYNC audio timing.
- **Two clocks — never mix them.** Everything in this package (`GetTime`, `GetTimeNS`, `Clock.Now*`) reads the **Go monotonic clock**, whose zero point differs from the **SDL event clock** used by `Screen.FlipTS`, `exp.ShowTS`, and `Keyboard.GetKeyEventTS`. Measure reaction times entirely on the SDL clock; use this package only for scheduling (ISIs, pacing) and log timestamps. See *User Manual §6 "Two clocks"*.
