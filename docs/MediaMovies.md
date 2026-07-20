# Media — Multi-Movie Playback with Hardware-Verified Display Onsets

This document describes the `media/` package added to goxpyriment for
multi-movie playback synchronised to a shared master clock, with
hardware-verified display-onset events suitable for wiring to external
EEG / MEG / TMS triggers.

## Table of Contents

1. [Scope and design](#1-scope-and-design)
2. [Quick start](#2-quick-start)
3. [Architecture in one diagram](#3-architecture-in-one-diagram)
4. [API reference (package `media`)](#4-api-reference-package-media)
5. [API reference (package `media/present`)](#5-api-reference-package-mediapresent)
6. [Multi-movie synchronisation: what is guaranteed](#6-multi-movie-synchronisation-what-is-guaranteed)
7. [External-trigger synchronisation with frames on screen](#7-external-trigger-synchronisation-with-frames-on-screen)
8. [Platform support (Stage 5)](#8-platform-support-stage-5)
9. [Mapping to PsyScope movie commands](#9-mapping-to-psyscope-movie-commands)
10. [References](#10-references)

---

## 1. Scope and design

### What `media/` is

A new top-level package alongside `stimuli/` that adds **multi-movie**
playback with shared timing. It is the home for everything related to
synchronising several video streams against one another and to external
hardware (EEG / MEG / TMS triggers, photodiodes).

It imports the semantics of PsyScope Tahoe's movie commands —
`MovieDo[ PLAY/PAUSE/STOP/SET/GET ]` actions and `Movie[Done/At/AtDisplay]`
+ `Display[Onset/Offset]` conditions — adapted to a Go API.

### Hard scope choices

- **`.gv` movies only.** Decoding is delegated to the existing pure-Go
  `funatsufumiya/go-gv-video` library (LZ4-compressed RGBA frames, random
  access). No ffmpeg, no MPEG-1, no H.264.
- **Silent.** Movies have no audio. The package does not load, decode,
  mix, or schedule audio of any kind.
- **Pure Go, no cgo.** SDL3 is loaded via `purego` (already used by
  `go-sdl3`); the macOS Stage 5 backend uses purego for CoreVideo /
  libSystem; the Linux Stage 5 backend uses the standard `syscall`
  package.
- **Coexists with `stimuli`.** `stimuli.GvVideo` and `stimuli.PlayGv`
  remain the single-movie convenience path. `media.Movie` is added
  alongside for the multi-movie / sync use cases.

### What problems it solves

| Problem | Solution |
|---|---|
| Multiple AVPlayer-style movies drift apart over hundreds of pause/resume cycles | One `MasterClock` shared by every `Movie`; positions computed from a single clock reading per draw cycle. Zero cumulative drift by construction. |
| Three sequential `Pause` calls reading the clock at slightly different times | `BeginBurst()` / `EndBurst()` freezes `MasterClock.Now` so all commands inside the burst observe the same value. |
| `Movie[At "f:N"]` needs to fire ~one vsync *before* the target frame is displayed so an overlay can land on the same vsync | `OnAt(Target, fn)` evaluates from inside the per-frame decode loop, *before* `FlipTS`. |
| External EEG trigger must be aligned to actual first-pixel-visible time, not to when our `Present` call returned | Stage 5 `media/present` package; on macOS uses `CVDisplayLink`, on Linux uses `DRM_IOCTL_WAIT_VBLANK`, both report OS-measured vsync. |
| Need to know post-hoc which exact vsync each trigger landed on | `Onset.TimestampNS` is in the same nanosecond reference as `sdl.TicksNS()` and SDL keyboard event timestamps; subtract directly to compute reaction times. |

---

## 2. Quick start

A complete two-movie experiment in ~25 lines, including hardware
trigger wiring (silently no-ops if no DLP-IO8 is plugged in):

```go
package main

import (
    "log"
    "time"

    "github.com/funatsufumiya/go-gv-video/gvvideo"

    "github.com/chrplr/goxpyriment/control"
    "github.com/chrplr/goxpyriment/media"
    "github.com/chrplr/goxpyriment/triggers"
)

func main() {
    exp := control.NewExperimentFromFlags("Demo", control.Black, control.White, 32)
    defer exp.End()

    gvL, _ := gvvideo.LoadGVVideo("clipL.gv")
    gvR, _ := gvvideo.LoadGVVideo("clipR.gv")

    mgr := media.NewMovieManager(exp.Screen)
    defer mgr.Close()

    left, _  := media.NewMovie(mgr, gvL, media.WithTag("L"), media.WithRepeat(-1))
    right, _ := media.NewMovie(mgr, gvR, media.WithTag("R"), media.WithRepeat(-1))
    defer left.Close()
    defer right.Close()

    ttl, _, _ := triggers.AutoDetectDLPIO8()
    defer ttl.Close()

    left.OnAtDisplay(media.Frame(60), func(o media.Onset) {
        log.Printf("frame 60 displayed @ %d (%s)", o.TimestampNS, o.Source)
        _ = ttl.Pulse(0, 5*time.Millisecond)
    })

    mgr.BeginBurst(); left.Play(); right.Play(); mgr.EndBurst()

    exp.Run(func() error {
        _, err := mgr.Draw()
        return err
    })
}
```

A more comprehensive walkthrough with composited overlays, burst pause,
seek-back, and `Display[Onset/Offset]` events lives in
[`examples/demo_sync_two_gv_movies/`](../examples/demo_sync_two_gv_movies/).

---

## 3. Architecture in one diagram

```
                  ┌─────────────────────────────────┐
                  │     media.MovieManager          │
                  │                                 │
                  │  ┌────────────────────────┐     │
                  │  │  media.MasterClock     │     │
                  │  │  (BeginBurst / Now / … │     │
                  │  └────────────────────────┘     │
                  │                                 │
                  │  ┌────────────────────────┐     │
                  │  │  []*media.Movie        │     │
                  │  │  (per-movie state,     │     │
                  │  │   gvvideo.GVVideo,     │     │
                  │  │   conditions, …)       │     │
                  │  └────────────────────────┘     │
                  │                                 │
                  │  ┌────────────────────────┐     │
                  │  │  present.Timer         │ ◄──── auto-detected:
                  │  │  (RecordFlip /         │     │      darwin → CVDisplayLink
                  │  │   OnsetForFlip)        │     │      linux  → DRM ioctl
                  │  └────────────────────────┘     │      other  → Fallback
                  │                                 │
                  │  pendingFlips queue             │
                  │  (deferred OnAtDisplay,         │
                  │   Display[Onset/Offset])        │
                  └─────────────────────────────────┘
                                │
                                │ uses
                                ▼
                       apparatus.Screen (SDL_Renderer)
```

Per-frame call site (mixed compositing form):

```go
exp.Screen.Clear()              // background
mgr.DrawWithoutFlip()           // decode + render all movies, fire OnAt look-ahead
otherStim.Draw(exp.Screen)      // overlay text / fixation cross / …
ts, _ := exp.Screen.FlipTS()    // present to display
mgr.NotifyFlipped(ts)           // fire OnAtDisplay + Display[Onset/Offset]
```

For movie-only frames the all-in-one `mgr.Draw()` collapses these into
one call.

---

## 4. API reference (package `media`)

### 4.1 `MovieManager`

The per-experiment owner of the master clock and the registered movies.

```go
func NewMovieManager(screen *apparatus.Screen, opts ...ManagerOption) *MovieManager
```

Constructor. Auto-detects the best `present.Timer` for the platform
unless overridden via `WithPresentTimer`. Logs the chosen backend at
construction:

```
media: present backend: macOS CVDisplayLink (CoreVideo, hardware-verified) (precision=hardware-verified)
```

```go
type ManagerOption func(*MovieManager)

func WithPresentTimer(t present.Timer) ManagerOption
```

Override the auto-detected timer. Pass `present.NewFallback()` to force
vsync-estimated mode for testing or to silence the macOS `CVDisplayLink`
background thread.

#### Lifecycle

| Method | Purpose |
|---|---|
| `(*MovieManager).Close() error` | Stops the macOS CVDisplayLink thread / closes the Linux DRM fd. Idempotent. **Defer this before `exp.End()`.** |
| `(*MovieManager).Clock() *MasterClock` | Returns the shared master clock (call `Reset()` at trial boundaries if needed). |
| `(*MovieManager).LogicalSize() (float32, float32)` | Returns the renderer's coordinate-space size (HiDPI-correct). Use for layout instead of `Screen.Size()`. |
| `(*MovieManager).LastFlipTS() uint64` | Most recent `FlipTS` passed to `NotifyFlipped`. |

#### Movie set

| Method | Purpose |
|---|---|
| `Add(m *Movie)` | Implicitly called by `NewMovie`. |
| `Remove(m *Movie)` | Deregister a movie. |
| `Movies() []*Movie` | Snapshot of registered movies in registration order. |

#### Burst (atomic command groups)

| Method | Purpose |
|---|---|
| `BeginBurst()` | Freeze `MasterClock.Now` at its current value (nestable). |
| `EndBurst()` | Decrement freeze counter; clock thaws when it reaches zero. |

`MovieManager.Draw` / `DrawWithoutFlip` already wrap their per-frame
body in an internal burst, so all movies on a frame share one
`MasterClock.Now`. Use the public `BeginBurst` / `EndBurst` to make a
sequence of script-style command calls atomic across multiple movies.

#### Per-frame

| Method | Purpose |
|---|---|
| `Draw() (uint64, error)` | All-in-one: clear → decode → render → flip → notify. Returns the post-flip SDL ticks. Movie-only frames. |
| `DrawWithoutFlip() error` | Decode + render, no clear, no Present. Caller composites other stimuli, then calls `Screen.FlipTS()` and `NotifyFlipped(ts)`. |
| `NotifyFlipped(ts uint64)` | Publish post-vsync events: advance each movie's `currentFrameIndex`, fire `OnAtDisplay`, fire `Display[Onset/Offset]`. Defers callbacks until the matching hardware-verified vsync timestamp arrives (see §7.4). |

#### Display[Onset/Offset]

| Method | Purpose |
|---|---|
| `RegisterStimulusName(name string, visible func() bool) func()` | Declare a non-movie stimulus by name. Visibility predicate is sampled in `DrawWithoutFlip`. Returns an unregister function. (Movies are auto-registered with their tag.) |
| `OnDisplayOnset(name string, fn func(Onset)) func()` | Fires when `name` transitions from not-visible to visible across two flips. Returns unsubscribe. |
| `OnDisplayOffset(name string, fn func(Onset)) func()` | Fires when `name` transitions from visible to not-visible. Returns unsubscribe. |

### 4.2 `Movie`

Per-movie state, mirrors `PSMovieDecoder` in PsyScope Tahoe.

```go
func NewMovie(mgr *MovieManager, gv *gvvideo.GVVideo, opts ...Option) (*Movie, error)
```

Wraps an already-loaded `gvvideo.GVVideo` and registers it with the
manager. The caller retains ownership of `gv` and is responsible for
closing the underlying file (matches `stimuli.GvVideo.Unload`).

#### Construction options

```go
type Option func(*Movie)

WithTag(tag string)              // identifier; required for Display[Onset/Offset]
WithRate(r float64)              // playback rate; 1.0 = normal, must be > 0
WithRepeat(n int)                // -1 = infinite, 1 = play once (default)
WithFromTime(d time.Duration)    // initial playhead within movie media-time
WithScale(s ScaleMode)           // ScaleNone | ScaleFit | ScaleFill
WithSize(w, h float32)           // explicit destination size; overrides WithScale
WithPosition(p sdl.FPoint)       // center-relative draw position
WithFadeIn(d time.Duration)      // linear opacity 0→1 ramp over d effective time
WithPersistent(b bool)           // last frame stays on screen after Done
```

#### Playback control

| Method | Effect |
|---|---|
| `Play()` | Start (first call) or resume (after `Pause`). Fresh start anchors playhead at `WithFromTime`. Idempotent. |
| `Pause()` | Freeze at current frame. |
| `PauseWithLoop(window time.Duration)` | Pause and bounce within ±|window| of pause point. Sign chooses bias. Window=0 ⇒ `Pause`. |
| `Stop()` | Halt, release GPU texture, reset state. After `Stop` the movie can be `Play`-ed again from `WithFromTime`. |
| `Close()` | Release GPU resources. Does not close the underlying `gvvideo.GVVideo`. |

#### Property access

| Method | Returns |
|---|---|
| `Time() time.Duration` | Effective media time (rate-scaled, cumulative across loops). |
| `Frame() int` | Cumulative 1-based displayed-frame index (advanced by `NotifyFlipped`). 0 before first frame. |
| `FPS() float64` | Frames per second from the `.gv` header. |
| `FrameCount() int` | Frames per loop iteration. |
| `LoopCounter() int` | 0-based loop iteration currently playing. |
| `Repeat() int` | Configured loop count (-1 = infinite). |
| `Width(), Height() float32` | Native frame dimensions. |
| `IsActive(), IsPaused(), IsDone() bool` | State queries. |

#### Property mutation

| Method | Effect |
|---|---|
| `SetRate(r float64) error` | Change playback rate; preserves current effective time. r ≤ 0 returns an error. |
| `SeekTime(d time.Duration) error` | Jump to elapsed media time `d`. Resets condition fired-flags so re-pass over the same target re-fires. |
| `SeekFrame(n int) error` | Jump to cumulative 1-based frame `n`. |

#### Conditions / callbacks

```go
// Sealed Target type — only these three constructors:
type Frame int                  // 1-based cumulative target
type AtTime time.Duration       // elapsed media time target
type Done struct{}              // end-of-file target

// Look-ahead (fires from DrawWithoutFlip, BEFORE the target frame is presented):
OnAt(target Target, fn func(Onset)) func()    // returns unsubscribe
OnDone(fn func(Onset)) func()                 // sugar for OnAt(Done{}, ...)
DoneCh() <-chan Onset                         // channel form for OnDone

// Hardware-verified (fires from NotifyFlipped, AFTER the target frame appears on screen):
OnAtDisplay(target Target, fn func(Onset)) func()
```

The look-ahead vs hardware-verified distinction is critical for trigger
alignment — see §7.

### 4.3 `MasterClock`

Monotonic media clock with freeze-during-burst support.

```go
func NewMasterClock() *MasterClock

(*MasterClock).Now() time.Duration   // monotonic since last Reset / construction
(*MasterClock).Reset()                // back to zero
(*MasterClock).BeginBurst()           // freeze Now at current value (nestable)
(*MasterClock).EndBurst()             // unfreeze when freeze count hits zero
(*MasterClock).Frozen() bool          // currently inside a burst?
```

### 4.4 Onset events

```go
type Onset struct {
    TimestampNS uint64       // SDL ticks; same reference as sdl.TicksNS / FlipTS
    Frame       int          // cumulative 1-based; 0 if not applicable
    Source      OnsetSource  // precision class
    Movie       *Movie       // nil for generic Display events of non-movie stimuli
    Name        string       // movie tag for movies; user name for others
}

type OnsetSource int

const (
    VsyncEstimated   OnsetSource = iota  // post-Present FlipTS, vsync-period precision
    HardwareVerified                      // OS-measured first-pixel-visible (sub-ms)
    LookAhead                             // pre-vsync, fired from decode loop
)
```

### 4.5 Time-spec helpers

For users translating PsyScope scripts to Go:

```go
ParseTimeSpec("s:3,ms:100")  // → 3.1 * time.Second, nil
ParseFrameSpec("f:186")      // → 186, nil
ParseFrameSpec("frame:1")    // → 1, nil
```

Native API uses `time.Duration` and `int` directly — these helpers are a
convenience for porting existing scripts.

---

## 5. API reference (package `media/present`)

The presentation-timer subpackage. Each backend reports OS-measured
vsync timestamps that pair with `Screen.FlipTS()` values to give
hardware-verified display-onset times.

```go
type Timer interface {
    RecordFlip(flipTS uint64)
    OnsetForFlip(flipTS uint64) (timestamp uint64, source OnsetSource, ok bool)
    Precision() OnsetSource
    Close() error
    Description() string
}

func AutoDetect(screen *apparatus.Screen) Timer  // platform-best, never nil
func NewFallback() Timer                          // always-available no-op
```

`MovieManager` calls `RecordFlip(ts)` immediately after each `FlipTS`,
then `OnsetForFlip(ts)` to read back the matching hardware-verified
vsync timestamp. If `ok=false`, the manager defers and retries on the
next `NotifyFlipped` (the OS may publish vsync timestamps
asynchronously).

Most users never construct a `Timer` directly — `NewMovieManager`
handles auto-detection. Use `WithPresentTimer(present.NewFallback())`
in tests if you need deterministic vsync-estimated behaviour.

---

## 6. Multi-movie synchronisation: what is guaranteed

### 6.1 Mechanism

Every per-frame body opens an internal burst, reads `MasterClock.Now`
exactly once, and computes each movie's effective media time from that
single value:

```go
// Inside MovieManager.DrawWithoutFlip:
mgr.clock.BeginBurst()
defer mgr.clock.EndBurst()
now := mgr.clock.Now()          // ← single read for this frame

for _, m := range movies {
    eff := m.effectiveLocked(now)   // each movie computes its own offset
    target := int(eff.Seconds() * m.fps) + 1
    // … decode + render …
}
```

Because every movie computes from the same `now`, the **inter-movie
spread within one frame is zero by construction**. There is no rounding
error to accumulate. Pause/resume cycles update each movie's
`mediaTimeOffset` independently but always by the same delta when
issued inside the same burst.

### 6.2 What that means in numbers

| Measurement | Result | Notes |
|---|---|---|
| Inter-movie spread at `Play` | **0 ms** | All movies anchored to one `MasterClock.Now` reading inside `BeginBurst`/`EndBurst`. |
| Inter-movie spread at first frame on screen | **0 ms** | All movies render in one `Renderer.Present` call → one vsync → one display refresh. |
| Cumulative drift after 500 pause/resume cycles | **0 ms** | Mathematical: pause/resume only changes per-movie `mediaTimeOffset`; drift would require a per-movie clock, which the design forbids. |
| Same-vsync rendering | All visible movies share the same vsync | Single `Renderer.Present` call → atomic page flip. |

This matches PsyScope Tahoe's measured numbers (see
`MultiMovieSyncStrategy.md` §8 in that repo).

### 6.3 The atomic-burst pattern

Use `mgr.BeginBurst() / mgr.EndBurst()` around any sequence of script-
style commands that should observe the same clock:

```go
mgr.BeginBurst()
left.Pause()      // captures MasterClock.Now at burst-start time T
right.Pause()     // captures the same T
center.Pause()    // captures the same T
mgr.EndBurst()
```

Without the burst, three sequential `Pause` calls would each read
`MasterClock.Now` a few microseconds apart. The burst ensures all three
movies pause at the exact same media time, regardless of scheduling
jitter between the calls.

### 6.4 What is *not* synchronised

- **Rendering completion across multiple Renderers.** This package uses
  one renderer (the `apparatus.Screen.Renderer`), so this is moot. If
  you build a multi-window experiment, each window has its own
  `MovieManager` and they synchronise only as well as the OS schedules
  their flips.
- **Decode latency variation.** Different movies may take different
  times to LZ4-decompress a frame. The decode happens before the
  shared `Renderer.Present` call, so a slow decode would push the entire
  frame past its target vsync. Prefer movies of similar resolution and
  bitrate; avoid per-frame allocations (the decode buffer is reused).
- **Audio.** Audio is out of scope (see §1).

---

## 7. External-trigger synchronisation with frames on screen

This section is the most important one for EEG / MEG / TMS experiments
and the question most users will have. Read it carefully.

### 7.1 The two timestamps that matter

For a callback registered via `mov.OnAtDisplay(media.Frame(N), fn)`,
two distinct timestamps are involved:

| Quantity | What it is | Source | Precision |
|---|---|---|---|
| `Onset.TimestampNS` | When frame N's first pixel begins scanning out of the display controller | OS vsync API (CVDisplayLink / DRM_IOCTL_WAIT_VBLANK) | **Sub-ms** vs scanout start (Stage 5 platforms) |
| Wire-arrival of TTL pulse | When the EEG box sees the trigger line go HIGH | DLP-IO8 USB / parallel port / MEGTTLBox + your callback latency | `TimestampNS + 0.5–5 ms` (constant + small jitter) |

`Onset.TimestampNS` is what you want for **post-hoc analysis**: it is
in the same nanosecond reference frame as `sdl.TicksNS()` and SDL3
keyboard event timestamps, so reaction times computed as
`keyEvent.Timestamp - onset.TimestampNS` are sub-ms accurate.

The wire-arrival is what arrives at your EEG amplifier's TTL input. It
lags `TimestampNS` by a small chain of latencies.

### 7.2 The wire-arrival delay

Breakdown of the ~0.5–5 ms gap between `TimestampNS` and the TTL line
going HIGH:

| Stage | Typical | Notes |
|---|---|---|
| OS publishes vsync timestamp | 0 µs (Linux DRM, sync ioctl) to ~1 ms (macOS, async CV callback may defer) | See §7.4 |
| Manager dispatches the callback | ~1–100 µs | Mutex + closure invocation |
| `ttl.Pulse` writes to the device | 0.5–2 ms (DLP-IO8 USB-CDC); <1 µs (parallel port); ~1 ms (MEGTTLBox 115200 baud) | Bus + firmware |
| **Total** | **~0.5 ms (parallel) to ~5 ms (USB)** | Stable / calibratable |

The delay is **constant and stable**. Calibrate once with a photodiode
+ TTL recording (see §7.5) and subtract the measured offset in your
analysis pipeline.

### 7.3 LookAhead vs HardwareVerified vs VsyncEstimated

The three `OnsetSource` values describe the precision class of
`Onset.TimestampNS`:

| Source | Where it fires | TimestampNS meaning | Use case |
|---|---|---|---|
| `LookAhead` | Inside `DrawWithoutFlip`, before `Present` | `sdl.TicksNS()` at fire time (NOT a vsync) | Set state for the same-vsync compositor (e.g., "show overlay this frame") |
| `VsyncEstimated` | After `Present`, in `NotifyFlipped` | Post-`Present` `FlipTS` value | Vsync-period precision (~16 ms at 60 Hz). Default fallback when no Stage 5 backend is available. |
| `HardwareVerified` | After `Present`, in `NotifyFlipped`, possibly deferred to next flip | OS-measured first-pixel-visible time | Sub-ms precision vs scanout start. Available on macOS + Linux. |

| Callback | Source on Stage 5 platforms | Source on fallback platforms |
|---|---|---|
| `mov.OnAt(t, fn)` | `LookAhead` | `LookAhead` |
| `mov.OnDone(fn)` | `LookAhead` | `LookAhead` |
| `mov.OnAtDisplay(t, fn)` | `HardwareVerified` | `VsyncEstimated` |
| `mgr.OnDisplayOnset(name, fn)` | `HardwareVerified` | `VsyncEstimated` |
| `mgr.OnDisplayOffset(name, fn)` | `HardwareVerified` | `VsyncEstimated` |

When the timer is `VsyncEstimated`, the manager logs a one-time warning
on the first display callback registration. When the timer is
`HardwareVerified`, the warning is suppressed.

### 7.4 The deferral pattern (macOS specifically)

On macOS, the `CVDisplayLink` callback fires on a background thread
asynchronously to `Present`. In the rare race where the callback hasn't
yet published a vsync timestamp by the time `NotifyFlipped(ts)` runs,
the manager:

1. Stashes this flip's callbacks in `pendingFlips`.
2. Continues normally (no error).
3. On the *next* `NotifyFlipped` (one vsync later, ~16 ms wall-clock),
   tries `OnsetForFlip(stashed_ts)` again. The CVDisplayLink callback
   has surely fired by now, so the lookup succeeds and the callbacks
   fire — with the **correct** hardware-verified timestamp for the
   stashed flip.

The trigger pulse therefore arrives ~16 ms after the visual onset in
this race case (the wire is one vsync late), but the
`Onset.TimestampNS` you receive is still sub-ms accurate for *frame N's
actual visual onset*. Post-hoc analysis is unaffected; only the
real-time wire timing is delayed by one vsync.

If a stash sits in the queue for more than `~3 × frame period`
(~50 ms at 60 Hz), the manager gives up waiting and fires the callbacks
with `(FlipTS, VsyncEstimated)` as a fallback, logging a single warning.
This handles compositor stalls and display-link issues without
silently inaccurate timestamps.

On Linux this race does not happen: `RecordFlip` queries DRM
synchronously right after `FlipTS`, the kernel returns the matching
vblank's count and timestamp before `OnsetForFlip` is called, and the
callback fires immediately on the same `NotifyFlipped`.

### 7.5 Calibration

For sub-ms wire-side alignment to visual onset, you must measure the
constant delay of your specific hardware:

1. Display a flashing white square in the corner of the screen.
2. Place a photodiode on the square; route both the photodiode signal
   and the TTL line to a recording amplifier with sub-ms sampling
   (most EEG amplifiers can do this directly).
3. Measure `delta = wire_HIGH_time − photodiode_first_light` over many
   trials. Take the median.
4. In your data analysis, subtract `delta` from every TTL marker
   timestamp.

After calibration, wire-side alignment to visual onset is bounded by
hardware jitter (typically tens of µs) plus your measurement
uncertainty. The `tests/Timing-Tests/` framework in this repository can
drive this calibration measurement automatically.

### 7.6 What is *not* measured

The Stage 5 backends report when the **display controller** starts
scanning out, not when LCD photons actually emit. Two layers of
photon-side latency are invisible to any software-only path on these
platforms:

- **Panel response** — 1–10 ms typical for IPS / VA / OLED panels at
  60 Hz, longer for cheap monitors with image processing turned on
  ("game mode" usually disables this). Fixed offset per monitor.
- **Scanout phase** — at 60 Hz, the top of the screen receives its new
  pixels at vsync; the bottom receives them ~16 ms later (full
  scanout = one frame period). The reported timestamp is **vsync** =
  start-of-scanout, so a stimulus at the bottom of the screen has up
  to a frame period of additional latency before its photons emit.
  Place timing-critical stimuli near the top, or use a photodiode at
  their actual location.

These limits also apply to Apple's `addPresentedHandler` and Vulkan's
`VK_EXT_present_timing` — they are intrinsic to the display pipeline,
not specific to this implementation. Photon-precise onset still
requires a photodiode.

### 7.7 Recommended pattern

```go
// At experiment start:
mov.OnAtDisplay(media.Frame(N), func(o media.Onset) {
    // Always log the hardware-verified timestamp, even if you also pulse a wire:
    exp.Data.Add(trialID, "stim_onset_ns", o.TimestampNS, o.Source.String())

    // Pulse the trigger immediately. The pulse arrives ~0.5–5 ms after o.TimestampNS;
    // calibrate the constant and subtract in analysis.
    _ = ttl.Pulse(0, 5*time.Millisecond)
})
```

Two records per event: the timestamp (hardware-verified, sub-ms) goes
into the data file; the wire pulse goes into the EEG amplifier. Align
post-hoc by subtracting the calibrated wire-arrival delay.

---

## 8. Platform support (Stage 5)

| Platform | Backend | Precision | Status | Prerequisites |
|---|---|---|---|---|
| **macOS** (Apple Silicon + Intel) | `CVDisplayLink` via `purego` (CoreVideo + libSystem) | `HardwareVerified`, sub-ms | **Shipped** | None — frameworks are part of the OS |
| **Linux** (X11 + Wayland) | `DRM_IOCTL_WAIT_VBLANK` via `syscall` on `/dev/dri/cardN` | `HardwareVerified`, sub-ms | **Shipped** | User must be in `video` group (default on most distros). At least one DRM device. |
| **Windows** | (none yet) | `VsyncEstimated` only | **Not yet shipped** | — |
| **FreeBSD / OpenBSD / others** | (none) | `VsyncEstimated` only | Falls back to `Fallback` | — |

The startup log line confirms the chosen backend:

```
media: present backend: macOS CVDisplayLink (CoreVideo, hardware-verified) (precision=hardware-verified)
media: present backend: Linux DRM vblank (DRM_IOCTL_WAIT_VBLANK, hardware-verified) (precision=hardware-verified)
media: present backend: vsync-estimated (post-Present FlipTS, no OS integration) (precision=vsync-estimated)
```

If auto-detection fails (e.g., Linux without `video` group, missing
`/dev/dri/cardN`), the manager logs the failure once and falls back
gracefully:

```
present: Linux DRM vblank unavailable (open /dev/dri/cardN: permission denied); falling back to vsync-estimated
media: present backend: vsync-estimated (post-Present FlipTS, no OS integration) (precision=vsync-estimated)
```

### 8.1 What Windows users get today

Windows builds compile cleanly (the `media/present/autodetect_other.go`
file matches `!darwin && !linux`) and use the `VsyncEstimated`
fallback. This means:

- All multi-movie synchronisation works (§6 guarantees are independent
  of Stage 5).
- All `Movie[At]` look-ahead callbacks work (LookAhead is software-only,
  always available).
- All `OnAtDisplay` / `Display[Onset/Offset]` callbacks fire — but
  with `Source: VsyncEstimated`, accurate to vsync-period (~16 ms at
  60 Hz) rather than sub-ms.
- A one-time warning is logged at the first display-callback
  registration:

  ```
  media: display-onset precision is vsync-period (~16.666ms at this refresh rate). Install a sub-ms presentation backend (Stage 5: Vulkan VK_EXT_present_timing or Metal addPresentedHandler) for hardware-verified timing.
  ```

Workaround for Windows users today: photodiode calibration (§7.5) using
the `VsyncEstimated` timestamp as a reference. The constant offset
includes one extra source of jitter (the ~vsync-period uncertainty),
but for low-trial-rate studies this is often acceptable.

### 8.2 What needs to be done for Windows

To bring Windows up to parity with macOS / Linux, a Stage 5 backend
needs to be added at `media/present/dxgi_windows.go` (next to the
existing `cvdisplaylink_darwin.go` and `drm_linux.go`). The most
promising APIs:

| API | Pros | Cons |
|---|---|---|
| **`IDXGISwapChain::GetFrameStatistics`** (DXGI 1.0+) | Available on every Windows since Vista. Returns `PresentRefreshCount` + `PresentQPCTime`. | DXGI swapchain owned by SDL_Renderer. We can't construct our own without taking over rendering. |
| **DwmGetCompositionTimingInfo** | Returns `qpcCompose` and `qpcVBlank` for the desktop window manager. Doesn't require swapchain ownership. | DWM-specific (composition mode); behaviour with fullscreen exclusive may differ. |
| **`D3DKMTWaitForVerticalBlankEvent`** + `D3DKMTGetScanLine` (gdi32) | Direct kernel-mode access; very precise. | Undocumented headers, requires `libloaderapi` workaround for the kernel32 entrypoint. |

Recommended path: **`DwmGetCompositionTimingInfo`** via purego on
`dwmapi.dll`. It's the cleanest API that doesn't require swapchain
ownership and works alongside SDL_Renderer.

Skeleton:

```go
//go:build windows

package present

// Use purego.Dlopen("dwmapi.dll") to get a handle, then bind:
//   DwmGetCompositionTimingInfo(HWND, *DWM_TIMING_INFO) HRESULT
// where DWM_TIMING_INFO contains qpcVBlank (QueryPerformanceCounter
// units) for the most recent vblank. Convert QPC ticks to SDL ticks
// via QueryPerformanceFrequency at startup.

func newWindowsBackend(screen *apparatus.Screen) (Timer, error) { ... }
```

Estimated effort: ~250 lines of Go (similar shape to the existing macOS
backend), plus ~100 lines of tests (purego symbol-loading, QPC →
SDL-ticks conversion, struct layout pinning).

Until that lands, Windows experiments needing sub-ms wire alignment
should rely on photodiode calibration (§7.5).

---

## 9. Mapping to PsyScope movie commands

Users porting PsyScript-based experiments will recognise this table.
Volume-related rows are explicitly unsupported (audio is out of scope —
see §1).

| PsyScope construct | goxpyriment Go API |
|---|---|
| `EventType Movie`, `Stimulus "clip.gv"`, `MovieTag M1` | `gv, _ := gvvideo.LoadGVVideo("clip.gv"); media.NewMovie(mgr, gv, media.WithTag("M1"))` |
| `MovieRate 1.0` | `media.WithRate(1.0)` / `mov.SetRate(...)` |
| `Repeat 3` / `Repeat -1` | `media.WithRepeat(3)` / `WithRepeat(-1)` |
| `FromTime "s:5"` | `media.WithFromTime(5*time.Second)` |
| `Flags Scale_X Scale_Y` | `media.WithScale(media.ScaleFit)` |
| `FadeIn 1000` | `media.WithFadeIn(1*time.Second)` |
| `ClearType NO_CLEAR` | `media.WithPersistent(true)` |
| `MovieDo[ PLAY ]` | `mov.Play()` |
| `MovieDo[ PAUSE ]` | `mov.Pause()` |
| `MovieDo[ PAUSE ... LoopRegion=ms:N ]` | `mov.PauseWithLoop(±N*time.Millisecond)` |
| `MovieDo[ STOP ]` | `mov.Stop()` |
| `MovieDo[ SET Rate=R ]` | `mov.SetRate(R)` |
| `MovieDo[ SET Time=... ]` | `mov.SeekTime(...)` |
| `MovieDo[ SET Frame=N ]` | `mov.SeekFrame(N)` |
| `MovieDo[ GET Time/TimeMs/Frame/FrameCount/FPS/Counter/Repeat ]` | `mov.Time()`, `Frame()`, `FPS()`, `FrameCount()`, `LoopCounter()`, `Repeat()` |
| `MovieDo[ SET Volume=... ]`, `GET Volume` | **Not supported** (audio out of scope) |
| `Movie[ Done ]` | `mov.OnDone(fn)` or `<-mov.DoneCh()` |
| `Movie[ At THISMOVIE "f:N" ]` | `mov.OnAt(media.Frame(N), fn)` |
| `Movie[ At THISMOVIE "s:T,ms:U" ]` | `mov.OnAt(media.AtTime(d), fn)` |
| `Movie[ AtDisplay THISMOVIE "f:N" ]` | `mov.OnAtDisplay(media.Frame(N), fn)` |
| `Display[ Onset ]` / `Display[ Offset ]` | `mgr.OnDisplayOnset(name, fn)` / `OnDisplayOffset(name, fn)` |
| `SerialOut[ "0xFF" ]` action | call inside the OnDisplay/OnAtDisplay callback into `triggers.OutputTTLDevice` |
| `THISMOVIE` keyword | irrelevant — the closure already captures `mov` |
| `Instances: "-1"` | irrelevant — Go calls execute once |

Burst-pause atomicity (the goxpyriment equivalent of three sequential
`MovieDo[ PAUSE ... ]` script lines hitting the same draw cycle):

```go
mgr.BeginBurst()
movieA.Pause(); movieB.Pause(); movieC.Pause()
mgr.EndBurst()
```

---

## 10. References

In this repository:

- [`examples/demo_sync_two_gv_movies/`](../examples/demo_sync_two_gv_movies/) — comprehensive walkthrough example
- [`tests/Timing-Tests/`](../tests/Timing-Tests/) — empirical timing-measurement framework

External / upstream:

- PsyScope Tahoe `MultiMovieSyncStrategy.md` — describes the original
  Apple-side architecture this package mirrors (CMTimebase, AVAssetReader,
  Metal addPresentedHandler).
- PsyScope Tahoe `CrossPlatformAnalysis.md` — feasibility study that
  led to the present `media/` package.
- [`VK_EXT_present_timing` Vulkan spec](https://docs.vulkan.org/features/latest/features/proposals/VK_EXT_present_timing.html)
  — the future direct-Vulkan path (not yet implemented; would replace
  `SDL_Renderer` entirely).
- Apple [`CVDisplayLink` reference](https://developer.apple.com/documentation/corevideo/cvdisplaylink-714)
  — the API the macOS backend uses.
- Linux DRM uAPI [`drm_wait_vblank`](https://www.kernel.org/doc/html/latest/gpu/drm-uapi.html)
  — the API the Linux backend uses.
