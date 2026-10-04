// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

# clock package

Timing utilities: a global elapsed-time reference and a per-instance `Clock` for local timing windows.

## SleepUntil

`SleepUntil(target)` blocks until the clock has reached `target`. It uses a polling loop — if `target` is already in the past, it returns immediately. OS scheduling determines the exact wake time; expect ±1–2 ms jitter on typical systems.

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
