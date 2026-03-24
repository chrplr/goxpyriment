// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

# clock package

Timing utilities: a global elapsed-time reference and a per-instance `Clock` for local timing windows.

## Package-level functions

```go
clock.Wait(200)         // sleep 200 ms (wraps time.Sleep)
ms := clock.GetTime()   // ms since package init (shared zero point)
```

`GetTime()` initialises its reference point on first call. All subsequent calls share that zero. Useful for logging event times relative to program start.

## Clock type

Each `Clock` has its own independent zero point:

```go
c := clock.NewClock()   // zero = now
c.Reset()               // restart zero to now

d  := c.Now()           // time.Duration since zero
ms := c.NowMillis()     // int64 milliseconds since zero

c.Sleep(50 * time.Millisecond)        // block for duration
c.SleepUntil(200 * time.Millisecond)  // block until 200 ms after zero
```

### SleepUntil

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
