# eyetracker/

Vendor-neutral gaze tracking, plus a socket client for hardware whose only API
is a C SDK.

## Why the bridge exists

SR Research publishes no network protocol for the EyeLink. The Display PC talks
to the Host PC over a proprietary link and the only supported entry point is the
C library (or pylink, which wraps it). Linking it would cost the pure-Go build,
cross-compilation and the browser target, for one device.

So the SDK runs in a separate process — `bridge/eyelink_bridge.py` — speaking
line-delimited JSON over a loopback socket. `Bridge` is the Go client. The
protocol is specified in `protocol.go` and is deliberately generic: a bridge for
another tracker is a new script, not a new Go package.

That claim has now been tested once. The Tobii Pro SDK for Python is a native
extension (`tobii_research_interop.so`) and hits the same wall for a different
reason, and `bridge/tobii_bridge.py` was added with **no new Go package**: the
transport moved to `bridge/bridgelib.py`, which both scripts import, and the Go
side gained only `calibration.go`. What the Tobii did need was two additions,
and both were forced by the vendor rather than chosen — see below.

## Files

| File | Contents |
|---|---|
| `tracker.go` | Package doc, the `Tracker` interface, `CalibrationOptions`, `Offset`, `NullTracker` |
| `events.go` | `Sample`, `Event`, `Eye`, and `Geometry` (the coordinate conversion) |
| `calibration.go` | `StepwiseCalibrator`, `StepwiseCalibrationReporter`, `CalibrationResult`, `StandardPoints` — for a tracker whose SDK draws no targets |
| `protocol.go` | Wire types and the protocol specification |
| `bridge.go` | The socket client |
| `simulated.go` | `Simulated` — a tracker driven by any position function |
| `bridge/bridgelib.py` | The transport: `Session`, `serve_forever`, and the back-end contract. Shared by every bridge |
| `bridge/eyelink_bridge.py` | The EyeLink back end (pylink, or `--simulate`) |
| `bridge/tobii_bridge.py` | The Tobii Pro back end (tobii_research, or `--simulate`) |

The drawing half of a stepwise calibration lives in `control/eyetracker_calib.go`
(`Experiment.CalibrateTracker`), not here: this package cannot import SDL,
because it also has to build for the browser.

## Two conventions that bite

**Coordinates.** `Sample.X/Y` are TRACKER pixels: origin top-left, +Y down. The
rest of goxpyriment is centre-origin, +Y up. Convert with `Geometry.ToCentre`.
Passing a raw tracker Y to a stimulus position mirrors the display vertically,
and it looks like a calibration fault rather than a units bug.

**Clocks.** `Sample.LocalNs` is stamped from whatever `WithClock` was given,
defaulting to `clock.GetTimeNS` — the Go monotonic clock. Stimulus onsets
(`Screen.FlipTS`, keyboard timestamps) are on SDL's clock, which has a different
origin. Subtracting one from the other yields a plausible-looking number that is
not a latency. In an experiment, pass:

```go
eyetracker.WithClock(func() int64 { return int64(control.TicksNS()) })
```

The package does not import SDL itself because it also has to build for the
browser.

## The Tobii, and the two things it forced

Both differences are the vendor's, not ours, and both are visible in the wire
format.

**No tracker-side data file.** An EyeLink Host PC writes an EDF and
`receive_file` pulls it off. Tobii samples exist only inside an SDK callback, so
`tobii_bridge.py` writes the record itself: `open` names a TSV, a writer thread
fills it at the tracker's full rate, and `receive_file` copies it where the
client asks. The existing `edf` and `receive_file` slots carry this unchanged.

The consequence worth knowing: the full-fidelity record never crosses the
socket, so a slow client cannot put holes in it — the reduced `sample` events
are a *second*, lossy stream for `Latest()` and gaze-contingent loops, and
`Dropped()` describes only that stream. The file is the record; the socket is
the view.

**The SDK draws no calibration targets.** `collect_data(x, y)` assumes the
participant is already looking at `(x, y)` and blocks while it samples, up to
10 s per the SDK headers. This is a *pull* model — the opposite of pylink's
`doTrackerSetup()`, which blocks and calls back — so it fits the existing
one-directional protocol with none of the reversed-request machinery sketched
below. Five additive commands (`calibration_enter`, `_collect`, `_discard`,
`_compute`, `_leave`), no bump to `proto`, and a bridge that does not implement
them rejects them by name.

`Bridge.Calibrate` on a Tobii therefore FAILS, with the alternative named in the
message. That is deliberate: a tracker that reported a calibration it never ran
would produce a session that looks entirely normal until the gaze is analysed.

### Three traps in tobii_bridge.py

- **`nan` must never reach the wire.** Tobii writes `nan` into an invalid gaze
  point, `json.dumps` emits a bare `NaN` for it, and Go's `encoding/json`
  *rejects* that — so one leaked blink drops the connection. Invalid samples
  omit `x` and `y` entirely, and the client marks any sample missing either one
  invalid. `TestAgainstTobiiBridge` requires a blink to have crossed the socket,
  so this path is covered rather than asserted about; the simulator blinks 40 ms
  in every 400 to guarantee it.
- **`pa` is pupil DIAMETER in millimetres**, roughly 2–8. On the EyeLink bridge
  the same field is pupil AREA in arbitrary units, in the thousands. Nothing in
  the protocol can catch a confusion between them, so each bridge states its
  unit at open and it is written into the gaze file's header and the run's
  `-info.txt`.
- **The normalized origin is an assumption, not a measurement.** The conversion
  is `x_px = nx*width`, `y_px = ny*height`, which requires normalized (0,0) to
  be the display area's TOP-LEFT. Tobii's published docs say so; the headers
  shipped with the SDK do not. `tests/test_tobii -corners` settles it against
  the hardware by comparing the residual with a mirrored expectation, and its
  output belongs in the commit that closes the question. Until then, treat it as
  open.

**One thing the Tobii makes easier.** `tr.get_system_time_stamp()` is
`CLOCK_MONOTONIC` in microseconds — measured on this machine, mean offset
−3.2 µs against `clock_gettime(CLOCK_MONOTONIC)`, n=2000, min −19.8, max +4.7;
`REALTIME`, `MONOTONIC_RAW` and `BOOTTIME` are each off by a large constant. So
with the bridge on the display machine the tracker clock and the Go clock are
the same counter with different origins, and the offset should be a *constant*
rather than the drifting rate two physical oscillators give you.

Do not yet rely on that. Which clock `sdl.TicksNS()` uses here is **not
measured**, and `CLOCK_MONOTONIC_RAW` drifted against `CLOCK_MONOTONIC` by
662 µs over that same 2000-call run — NTP slewing, and enough to matter over a
session. Until `sdl.TicksNS()` is pinned down, quote `Sync`'s offset with
`BestRTT/2` as its uncertainty exactly as for the EyeLink. `tests/test_tobii`
reports the drift, which is the figure that tests the claim.

## What must not go through the bridge

Anything whose timestamp is the measurement. An EyeLink Host PC records TTL
input on its parallel port as `INPUT` events in the EDF, timestamped by the Host
when the edge arrives — no round trip, nothing to be delayed by the socket, and
`triggers.FireTriggerSync` already drives that hardware from the flip thread.

`Mark` is for trial labels and bookkeeping. `tests/test_eyelink` times a `Mark`
round trip against the flip it belongs to, on the display machine: 600-719 us
median, 1207 us worst, over three runs on the MEG rig. Comparing the two routes
*in the EDF* — the only place they share a clock — needs a TTL line wired to the
Host's DB25, which the MEG rig does not have: there the TTL goes to the
acquisition's STI channel instead, and meets the gaze in the MISC channels.

## Testing

`go test ./eyetracker/` runs three layers:

- The fake-server tests (`bridge_test.go`) drive the client through every path
  with no Python. They prove the client is self-consistent, and nothing more.
- `TestAgainstPythonBridge` starts the real `eyelink_bridge.py --simulate` and
  runs a whole session against it. This is the only check that the Python and
  the Go agree; it skips when `python3` is absent and never needs pylink.
- `TestAgainstTobiiBridge` does the same for `tobii_bridge.py --simulate`,
  calibration included, and additionally requires that both eyes arrived, that
  a blink crossed the socket, and that `pa` looks like a diameter in mm rather
  than an area. It never needs the Tobii SDK.
- The `Simulated` tests cover the no-hardware tracker.

Both Python round-trip tests run in CI: `.github/workflows/go-build.yml` runs
`go test ./eyetracker/` on ubuntu, where `python3` is present.

None of it touches a tracker. The parts that only hardware can exercise are the
pylink calls in `EyeLinkTracker` and the `tobii_research` calls in
`TobiiTracker`, and they are the parts to distrust. Note in particular that
`--simulate` cannot check the coordinate origin, the real sample rate, or
whether `collect_data` accepts a point: those need the runbook in `TODO.md`.

## Open

**Calibration graphics drawn by goxpyriment — done for the Tobii, still open
for the EyeLink.**

The routine now exists: `control/eyetracker_calib.go`'s
`Experiment.CalibrateTracker` draws the targets with `stimuli/`, in
goxpyriment's own window, on the flip clock, and records the per-target sample
counts. A tracker gets it by implementing `StepwiseCalibrator` **and** reporting
`SupportsStepwiseCalibration() == true`. Both are needed because `*Bridge`
satisfies the interface whatever answered the socket: the Go type cannot see
which back end is on the far side, so `CalibrateTracker` asks, and the answer
comes from the `caps` list in the bridge's hello (computed from the methods the
Python back end actually defines).

The EyeLink does not, and cannot without the work below, because pylink's
calibration inverts the direction of control. Everything from here down is
about that, and still stands. What has changed is that the *client* half is
written and proven against a real tracker's API, so this is now one side of a
working pattern rather than a design from scratch.

Two paths exist today for the EyeLink and neither is ours:

- **pylink draws them, in its own window.** `EyeLinkTracker.calibrate` calls
  `pylink.openGraphics()` around `doTrackerSetup()`, so on a rig where pylink
  ships a graphics environment the targets appear in a SECOND window, opened by
  the Python process. This works, and it is what you see when a calibration
  screen comes up. The costs: the bridge must run on the *display* machine (a
  bridge on another host draws the calibration on that host's screen), its
  window competes with goxpyriment's fullscreen one — closing it hands focus to
  whatever the window manager finds next, which is why `CalibrateTracker` and
  `tests/test_eyelink` call `Experiment.ReclaimDisplay` afterwards — and target
  onsets are on pylink's clock rather than flip-locked.
- **The bridge refuses instead.** Where `pylink.openGraphics` is absent, the
  bridge raises an error naming the problem rather than hanging on a blank
  screen, and the operator calibrates from the SR Research display software.
  `--simulate` sleeps 0.2 s and draws nothing at all.

### Sketch: what the protocol needs

The obstacle is direction. Every command today is client→bridge, and events are
one-way bridge→client with no reply. But `doTrackerSetup()` blocks inside Python
and *calls back* — pylink's `openGraphicsEx(EyeLinkCustomDisplay)` expects
`draw_cal_target`, `clear_cal_display`, `play_beep`, `get_input_key` and friends
to be invoked by the tracker, synchronously. So the bridge has to be able to ask
the client for something and wait for the answer.

Add a **reversed request**, distinguished by `req`/`rid` as `id` and `ev`
already distinguish the other two kinds of line:

```
bridge → client   {"req":"draw_cal_target","rid":1,"x":960.0,"y":540.0}
client → bridge   {"rid":1,"ok":true,"result":{"t":1234567.0}}
```

`rid` is the bridge's own counter, independent of the client's `id`, so the two
demultiplexers never collide. `message.isEvent` becomes a three-way split, and
`Bridge` grows a reader-side dispatch to a handler the caller installs.

Negotiate it from the client, not the bridge, since a client that cannot draw
must never be sent a draw request. Keep `hello` at `proto` 1 for old bridges and
put the opt-in in the existing command:

```
{"id":2,"cmd":"calibrate","args":{"points":9,"graphics":"client"}}
```

A bridge that does not understand `graphics` falls through to `openGraphics()`
as now; one that does installs a custom display and drives the reversed
requests.

**The interface, verified on this machine** — pylink 2.2.596.0 in the 3.10 venv
at `~/eyelink/`, read with `hasattr` and `inspect.signature`. No tracker is
needed to repeat this: it is an import-time check.

| `EyeLinkCustomDisplay` method | v1 |
|---|---|
| `setup_cal_display(self)` | yes — enter setup, clear the screen |
| `clear_cal_display(self)` | yes |
| `draw_cal_target(self, x, y)` | yes — the one that matters |
| `erase_cal_target(self)` | yes |
| `exit_cal_display(self)` | yes |
| `play_beep(self, beepid)` | yes |
| `get_input_key(self)` | yes — must not block |
| `get_mouse_state(self)` | stub |
| `alert_printf(self, msg)` | stub — route it to the `log` event |
| `record_abort_hide(self)` | stub |
| `setup_image_display(self, width, height)` | no — decline, see below |
| `exit_image_display(self)` | no |
| `image_title(self, title)` | no |
| `draw_image_line(self, width, line, totlines, buff)` | no |
| `set_image_palette(self, red, green, blue)` | no |
| `draw_line(self, x1, y1, x2, y2, colorindex)` | no — camera overlay |
| `draw_lozenge(self, x, y, width, height, colorindex)` | no — camera overlay |
| `draw_cross_hair(self)` | no — camera overlay |

`pylink.openGraphicsEx(eyeCustomDisplay)` takes the display object.
`openGraphics`, `closeGraphics` and `EyeLinkCustomDisplay` are all present
beside it.

**`openGraphicsEx` is not a rescue for a missing `openGraphics`.** Both exist in
this pylink, and calibration already works through `openGraphics` on the stim
PC, so the `hasattr` guard above never fires on our hardware. Whether the two
ever diverge on some other install is unknown and must not be assumed either
way. The reasons to move the drawing here are the three above — one machine
instead of two, one window instead of two, a flip-locked target onset — not
availability.

Four things to get right:

- **Coordinates.** `draw_cal_target` carries TRACKER pixels, like everything
  else on this wire — origin top-left, +Y down. The handler must go through
  `Geometry.ToCentre` before positioning the stimulus, or the calibration is
  mirrored vertically and every subsequent gaze position is wrong in a way that
  looks like bad calibration rather than a units bug. This is the same trap as
  `showLiveGaze` in `tests/test_eyelink`.
- **`get_input_key` must not block.** pylink polls it in a tight loop, so the
  handler returns the keys pending *now* (possibly none) and returns at once. It
  also has to map goxpyriment keycodes to EyeLink's — Enter, Esc, `c`, `v`, the
  arrows, and PageUp/PageDown for switching the camera view.
- **The camera image is the hard part.** `draw_image_line(width, line, totlines,
  buff)` delivers the eye image one scanline at a time, its colours in a
  separate `set_image_palette` — far too chatty for a line-delimited JSON socket
  at any useful frame rate. v1 should decline it, and with it `draw_line`,
  `draw_lozenge` and `draw_cross_hair`, which only draw overlays on that image.
  That leaves camera and pupil setup on the Host PC's own screen, where the
  operator already stands. A later version can send whole frames as one base64
  blob on a separate event.
- **`get_mouse_state` needs a stub, not a mouse.** On the KMSDRM console SDL
  cannot create a cursor shape at all, so return a fixed position and no buttons
  rather than wiring a real pointer.

Reply timing is worth stating in the protocol text: the client answers
`draw_cal_target` *after* the flip that put the target on screen, and returns
that timestamp. That is the whole point of moving the drawing here — it is the
one thing pylink's own graphics cannot report.
