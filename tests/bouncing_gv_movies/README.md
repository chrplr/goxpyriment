# Bouncing GV Movies

Two `.gv` movies smoothly bounce around the screen while their sizes
oscillate. Wherever they overlap, their RGB values are summed
(`sdl.BLENDMODE_ADD`) so both movies remain visible — the overlap
region appears brighter rather than one movie obscuring the other.

This example exercises **every runtime mutator** of `media.Movie` from
the keyboard, fires two distinct beeps at user-configurable frame
markers, and logs both movies' full `Snapshot` to the experiment data
file on every event (frame trigger or key press). It also demonstrates
the **multi-movie synchrony invariant**: every command that changes
playback state (rate, pause, seek, pause-with-loop) is wrapped in
`mgr.BeginBurst` / `mgr.EndBurst` so both movies observe the same
`MasterClock.Now` value when they apply the change.

---

## Quick start

```bash
# From inside the example directory (uses the bundled test fixtures)
cd examples/bouncing_gv_movies
go run main.go -w

# From the repo root
go run examples/bouncing_gv_movies/main.go -w
```

The defaults look for `PhysicalViolation.gv` and `PhysicalViolation2.gv`
in the current directory, then in `../../tests/test_playgv/`, then in
`tests/test_playgv/`.

---

## Keyboard cheatsheet

### Playback control (synchronised across both movies)

| Key | Action | Underlying API |
|---|---|---|
| `SPACE` | Pause ↔ resume both | burst { `Pause()` / `Play()` } |
| `L` | Pause-with-loop, sawtoothing through 10 frames forward | burst { `PauseWithLoop(window)` } |
| `1` | Rate → 1.0× (normal speed) | burst { `SetRate(1.0)` } |
| `2` | Rate → 2.0× (double speed) | burst { `SetRate(2.0)` } |
| `3` | Rate → 0.5× (half speed) | burst { `SetRate(0.5)` } |
| `+` / `=` | Multiply rate by 1.25 (clamped to [0.1, 8.0]) | burst { `SetRate(rate × 1.25)` } |
| `-` | Divide rate by 1.25 | burst { `SetRate(rate ÷ 1.25)` } |
| `Z` | Seek both to frame 1; re-arms the LEFT/RIGHT frame triggers | burst { `SeekFrame(1)` } |

### Rendering

| Key | Action | Underlying API |
|---|---|---|
| `F` | Toggle blend mode: `ADD` ↔ `BLEND` | `SetBlendMode(...)` |

### Animation (example-side, not Movie API)

| Key | Action |
|---|---|
| `R` | Reset bounce positions to opposite corners |
| `Up` | Scale bounce speed × 1.25 |
| `Down` | Scale bounce speed × 0.8 |
| `W` | Widen size oscillation (max width × 1.15) |
| `N` | Narrow size oscillation (max width ÷ 1.15) |

### Data logging

| Key | Action | Data row(s) |
|---|---|---|
| `A` | Snapshot LEFT movie now | one row, `event=key_press:A` |
| `B` | Snapshot RIGHT movie now | one row, `event=key_press:B` |

### Dynamic frame-trigger scheduling (PsyScope `Movie[AtDisplay]` equivalent)

Each keypress reads the target movie's *current* displayed frame and
registers a one-shot `OnAtDisplay` condition for `current + 20`. When
the target frame actually appears on screen, the same tone fires that
the auto trigger uses, both movies are snapshotted into the data
file, and the condition unsubscribes itself (so repeated presses
queue distinct one-shot triggers without leaking entries on the
movie's condition list).

| Key | Action | Data row(s) |
|---|---|---|
| `J` | Schedule LEFT tone (440 Hz) + both-movie snapshot at LEFT frame `current+20` | scheduling row `event=schedule:left@N` (1 row); when fired, `event=scheduled_fire:left@N` (2 rows) |
| `K` | Schedule RIGHT tone (880 Hz) + both-movie snapshot at RIGHT frame `current+20` | scheduling row `event=schedule:right@N` (1 row); when fired, `event=scheduled_fire:right@N` (2 rows) |

This mirrors the PsyScope Tahoe pattern of issuing
`Movie[ AtDisplay THISMOVIE "f:N+20" ] ⇒ Actions[ play_beep ]`
dynamically from a script — except in goxpyriment the registration is
just a Go function call that can happen at any moment in your run loop.

You can press `J` (or `K`) several times in quick succession to queue
up a sequence of distinct one-shot triggers; each fires when its
specific target frame appears. Pressing `Z` (seek to frame 1) clears
each pending condition's `fired` flag and rewinds — pending schedules
still fire at their target frames after the rewind, since their
target is a *cumulative* frame index. To cancel a pending schedule
before it fires, press `Stop` (currently only ESC, which exits) — a
future `Cancel` key could be added by holding the unsubscribe handle
keyed by something like the schedule's target frame.

### Misc

| Key | Action |
|---|---|
| `ESC` | Quit |

---

## Automatic frame triggers (registered at startup)

Two `OnAtDisplay` callbacks are registered at startup:

| Trigger | Default frame | Effect |
|---|---|---|
| LEFT movie reaches frame N | `-leftAt N` (default 50) | Play `tone1` (440 Hz, 150 ms, A4) **and** snapshot BOTH movies into the data file |
| RIGHT movie reaches frame N | `-rightAt N` (default 80) | Play `tone2` (880 Hz, 150 ms, A5) **and** snapshot BOTH movies into the data file |

Both triggers use `OnAtDisplay`, so they fire from
`MovieManager.NotifyFlipped` with the **hardware-verified vsync
timestamp** for the frame that actually appeared on screen
(`Onset.Source = HardwareVerified` on macOS and Linux,
`VsyncEstimated` on Windows). See
[docs/MediaMovies.md §7](../../docs/MediaMovies.md#7-external-trigger-synchronisation-with-frames-on-screen)
for the precision contract.

Each trigger fires **once per Stop / SeekFrame(1) cycle**. Press `Z`
to seek both movies to frame 1 — this clears the conditions'
fired-flag and re-arms the triggers.

`PauseWithLoop` (key `L`) does **not** re-arm one-shot triggers on
each sawtooth wrap. If the loop region crosses an auto trigger's
target frame, the trigger fires on the *first* forward sweep across
that frame and stays quiet on subsequent sweeps. To re-fire on every
loop pass, re-register the condition inside the callback (the same
unsub-and-resub pattern is the path to repeating `J`/`K` schedules).

---

## Multi-movie synchrony invariant

Every keyboard command that touches more than one movie's playback
state is wrapped in a `BeginBurst` / `EndBurst` pair. The helper
functions in `main.go` make this explicit:

```go
func setRateBoth(mgr *media.MovieManager, left, right *media.Movie, r float64) float64 {
    mgr.BeginBurst()
    defer mgr.EndBurst()
    _ = left.SetRate(r)
    _ = right.SetRate(r)
    return r
}
```

Inside the burst, `MasterClock.Now()` is frozen at one value — both
`SetRate` calls anchor the same effective time, so they share an
identical playback origin. The same pattern applies to:

- `togglePauseBoth` (Pause / Play)
- `pauseLoopBoth` (PauseWithLoop, including the sawtooth start point)
- `multiplyRateBoth`, `setRateBoth` (any rate change)
- `seekFrameBoth` (any seek)

The animation parameters (bounce velocity, size oscillation max width)
are example-side state and don't touch the Movie API, so they don't
need bursts.

---

## Data file

The example declares 15 columns via `exp.AddDataVariableNames`:

| Column | Meaning |
|---|---|
| `ts_ns` | SDL ticks at event time (Onset.TimestampNS for triggers; sdl.TicksNS for key snapshots) |
| `ts_source` | `hardware-verified` / `vsync-estimated` / `look-ahead` / `wall-clock` |
| `event` | Human-readable event label (`frame_trigger:left@50`, `key_press:A`, etc.) |
| `movie` | Tag of the movie this row describes (`left` or `right`) |
| `frame` | Cumulative 1-based displayed-frame index |
| `time_ms` | Effective media time in ms |
| `rate` | Current playback rate multiplier |
| `loop` | Current 0-based loop iteration |
| `cx`, `cy` | Center position (px, screen-center-relative) |
| `w`, `h` | Explicit destination size (px); 0 if scale-derived |
| `blend` | Blend mode name (`add`, `blend`, `none`, …, or `default`) |
| `is_paused` | true / false |
| `loop_window_ms` | PauseWithLoop window in ms (0 if not in loop) |

Frame triggers write **two rows** (one per movie) so both movies'
state at the moment of trigger is captured atomically. Key snapshots
(`A`, `B`) write **one row** for the targeted movie only.

Output file pattern:
`goxpy_data/<expname>_sub-<NNN>_date-<YYYYMMDD>-<HHMM>.csv`. See
[docs/UserManual.md §8](../../docs/UserManual.md) for the data-file
format details.

---

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-fL` | autodetect | Path to left `.gv` file |
| `-fR` | autodetect | Path to right `.gv` file |
| `-speed` | `500.0` | Initial peak bouncing speed in px/sec, per axis |
| `-sizeMin` | `0.18` | Minimum movie width as fraction of screen width |
| `-sizeMax` | `0.55` | Maximum movie width as fraction of screen width |
| `-leftAt` | `50` | LEFT-movie frame that triggers tone1 + snapshot |
| `-rightAt` | `80` | RIGHT-movie frame that triggers tone2 + snapshot |
| `-s` | `0` | Participant ID |
| `-w` | off | Windowed mode (1024×768 instead of fullscreen) |
| `-d N` | -1 | Display index where the window opens (-1 = primary) |

---

## What you should see in the log

```
loaded LEFT  ../../tests/test_playgv/PhysicalViolation.gv: 1280x720, 7290 frames @ 30 fps
loaded RIGHT ../../tests/test_playgv/PhysicalViolation2.gv: 1280x720, 7290 frames @ 30 fps
media: present backend: macOS CVDisplayLink (CoreVideo, hardware-verified) (precision=hardware-verified)
layout: logical 1024x768, columns 184..563
Controls:
  SPACE      pause / resume both (atomic)
  L          pause-with-loop (sawtooth over 10 frames forward)
  ...

[trigger] LEFT  frame 50 displayed @ 18234567890 (hardware-verified) → tone1 (440 Hz)
[trigger] RIGHT frame 80 displayed @ 19234567890 (hardware-verified) → tone2 (880 Hz)
[+] rate × 1.250 → 1.250×
[L] PauseWithLoop window = 333.333333ms (10 frames @ 30 fps)
[Z] seek both to frame 1 (frame triggers re-armed)
[A] LEFT snapshot logged @ 22345678901
[J] tone scheduled for left frame 137 (current=117, +20 frames)
[scheduled] left frame 137 displayed @ 23456789012 (hardware-verified) → tone
```

---

## Implementation notes

- The animation `dt` is computed from wall-clock `time.Now()`, capped at
  100 ms to dampen huge jumps after the program is paused
  (system pause, debugger break, etc.).
- `PauseWithLoop` implements a **sawtooth** through the configured
  window: positive window cycles forward; negative window cycles
  backward. Both movies pause-with-loop'd inside one burst share the
  same `pauseStartMediaTime`, so they cycle in perfect lockstep.
- During pause (including pause-with-loop), the example freezes the
  bounce animation as well — the panel position stays put while the
  video content cycles. This makes the loop visually obvious.
- `mov.Snapshot()` is an **atomic** read of every observable property —
  one mutex acquisition, one consistent view. Prefer it over chaining
  individual getters when logging.
