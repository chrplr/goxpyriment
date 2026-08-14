# media package

Multi-movie playback with shared master-clock synchronisation,
look-ahead frame conditions, and post-vsync display events suitable for
wiring to hardware triggers. Imports the PsyScope Tahoe semantics
documented in `Plan.md`.

## Scope

Only `.gv` movies (decoded by `funatsufumiya/go-gv-video`). No ffmpeg.
No audio. Pure Go, no cgo. The package extends — does not replace —
`stimuli.Video` / `stimuli.GvVideo` / `stimuli.PlayGv`; those remain the
single-movie convenience path.

## Key types

- `MasterClock` — monotonic, freeze-during-burst.
- `MovieManager` — owns the clock and the set of `Movie`s.
- `Movie` — per-movie state (Play/Pause/Stop/SetRate/SeekTime/SeekFrame/...).
- `Onset` — emitted to `OnAt`, `OnAtDisplay`, `OnDisplayOnset`,
  `OnDisplayOffset`, `OnDone` callbacks.
- `Target` (sealed) — `Frame(n)`, `AtTime(d)`, `Done{}`.
- `OnsetSource` — `LookAhead`, `VsyncEstimated`, `HardwareVerified`.

## Per-frame call sites

Movie-only frame:

```go
ts, _ := mgr.Draw()      // clear → decode → render → flip → notify
```

Mixed frame (movies composed with other stimuli):

```go
exp.Screen.Clear()
mgr.DrawWithoutFlip()
otherStim.Draw(exp.Screen)
ts, _ := exp.Screen.FlipTS()
mgr.NotifyFlipped(ts)
```

## Burst pattern (atomic command groups)

```go
mgr.BeginBurst()
movieA.Pause()
movieB.Pause()
movieC.Pause()
mgr.EndBurst()
```

All three Pause calls observe the same `MasterClock.Now` value.
`MovieManager.DrawWithoutFlip` opens its own burst around the per-frame
body, so all movies on a frame are advanced from one clock reading.

## Onset precision (Stage 5 backend, shipped)

The `media/present` subpackage auto-detects the best presentation
timer for the platform:

- **macOS**: `CVDisplayLink` via purego (CoreVideo + libSystem). The OS
  callback publishes per-vsync timestamps from a background thread.
  `Onset.Source: HardwareVerified`.
- **Linux**: `DRM_IOCTL_WAIT_VBLANK` via syscall on
  `/dev/dri/cardN`. Needs read/write on the node — usually `video` group
  membership, though a local login often grants it through a logind ACL;
  otherwise falls back. `Onset.Source: HardwareVerified`.

  It searches **every card node × CRTC 0–3** and takes the first pair that
  answers. Neither "first node that opens" nor "CRTC 0" is safe on its own: a
  render-only node enumerated first returns `EINVAL` (its pipe count is zero),
  and the display need not be on CRTC 0. Both were found on real hardware — an
  Intel/Mesa laptop where `card1` answers and `card2` returns `ENOTSUP` on every
  CRTC, and a Radeon Pro W5700 where the first node to open returned `EINVAL`
  and the old code gave up rather than trying the next. `Description()` names
  the pair that won, so a wrong choice is visible in the log.

  The probe cannot tell a live CRTC from a blanked one — an idle pipe answers
  without its sequence advancing, and distinguishing them means waiting, which a
  constructor must not do. `tests/test_vblank_drift` checks the sequence and
  reports the grid residual for that reason; run it if onsets look wrong.
- **Other / fallback**: post-`Present` `Screen.FlipTS` value.
  `Onset.Source: VsyncEstimated`.

`MovieManager` logs the chosen backend at construction:

```
media: present backend: macOS CVDisplayLink (CoreVideo, hardware-verified) (precision=hardware-verified)
```

To force the fallback for testing:

```go
mgr := media.NewMovieManager(screen,
    media.WithPresentTimer(present.NewFallback()))
```

Look-ahead `OnAt` / `OnDone` callbacks always carry `Onset.Source:
LookAhead`; their `TimestampNS` is the moment the callback fired, NOT
a vsync timestamp. Use look-ahead callbacks to set state read by the
same frame's compositor (e.g., toggle an overlay flag).

When the timer is `VsyncEstimated`, a one-time warning is emitted on
the first display callback registration, naming the current refresh
period. When the timer is `HardwareVerified`, the warning is
suppressed.

### Deferred-fire model (Stage 5)

OS-published vsync timestamps may not be ready at the moment of
`NotifyFlipped` — on macOS the CVDisplayLink callback can fire just
after our query. In that case the callbacks for that flip are stashed
and fired from the *next* `NotifyFlipped` (~16 ms later) with the
correct hardware-verified timestamp. The trigger fires at the right
time on the wire (the timestamp passed to your callback is the actual
first-pixel-visible time); only the in-Go callback invocation is
delayed by one vsync. This mirrors PsyScope Tahoe's
`pendingMovieAtDisplay` deferral pattern.

If a stashed flip sits longer than `~3 × frame period` (~50 ms at
60 Hz), the manager gives up waiting and fires with the FlipTS as a
fallback (`VsyncEstimated`), logging a single warning. This handles
edge cases like compositor latency or display-link stalls without
silently inaccurate timing.

### Lifecycle

`MovieManager.Close()` releases backend resources (stops the macOS
CVDisplayLink background thread; closes the Linux DRM file descriptor).
Defer it before `exp.End()`:

```go
mgr := media.NewMovieManager(exp.Screen)
defer mgr.Close()
```

## Trigger wiring

```go
ttl, _, _ := triggers.AutoDetectDLPIO8()
defer ttl.Close()

mgr.OnDisplayOnset("StimMovie", func(o media.Onset) {
    _ = ttl.Pulse(0, 5*time.Millisecond)
})
movie.OnAtDisplay(media.Frame(186), func(o media.Onset) {
    _ = ttl.Send(0xFF)
})
```

Triggers fire inline from `NotifyFlipped`. Keep callbacks fast — the
DLPIO8 / MEGTTLBox / parallel-port `Pulse`/`Send` calls are sub-ms.

## What is NOT in this package

- No video decoders other than `.gv`.
- No audio.
- No new SDL renderer backend (uses the existing
  `apparatus.Screen` SDL_Renderer for drawing).
- No cgo. The macOS Stage 5 backend uses purego for CoreVideo /
  libSystem; the Linux Stage 5 backend uses the standard `syscall`
  package for the DRM ioctl and clock_gettime. SDL3 itself loads via
  purego (`go-sdl3`).
- No Windows hardware-verified backend yet — Windows currently gets
  the vsync-estimated fallback. DXGI `GetFrameStatistics` is the
  planned addition.

See `Plan.md` §8 for the architectural rationale and the future
direct-Vulkan / direct-Metal path that would unlock sub-display-
controller precision.
