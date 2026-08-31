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

## Files

| File | Contents |
|---|---|
| `tracker.go` | Package doc, the `Tracker` interface, `CalibrationOptions`, `Offset`, `NullTracker` |
| `events.go` | `Sample`, `Event`, `Eye`, and `Geometry` (the coordinate conversion) |
| `protocol.go` | Wire types and the protocol specification |
| `bridge.go` | The socket client |
| `simulated.go` | `Simulated` — a tracker driven by any position function |
| `bridge/eyelink_bridge.py` | The bridge process (pylink, or `--simulate`) |

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

## What must not go through the bridge

Anything whose timestamp is the measurement. An EyeLink Host PC records TTL
input on its parallel port as `INPUT` events in the EDF, timestamped by the Host
when the edge arrives — no round trip, nothing to be delayed by the socket, and
`triggers.FireTriggerSync` already drives that hardware from the flip thread.

`Mark` is for trial labels and bookkeeping. `tests/test_eyelink` measures the
difference between the two routes in the EDF itself, which is the only place
they share a clock.

## Testing

`go test ./eyetracker/` runs three layers:

- The fake-server tests (`bridge_test.go`) drive the client through every path
  with no Python. They prove the client is self-consistent, and nothing more.
- `TestAgainstPythonBridge` starts the real `eyelink_bridge.py --simulate` and
  runs a whole session against it. This is the only check that the Python and
  the Go agree; it skips when `python3` is absent and never needs pylink.
- The `Simulated` tests cover the no-hardware tracker.

None of it touches an EyeLink. The parts that only hardware can exercise are the
pylink calls in `EyeLinkTracker`, and they are the parts to distrust.

## Open

Calibration graphics. `doTrackerSetup` needs somewhere to draw targets;
pylink's built-in graphics are not available everywhere, and goxpyriment owns
the display. The bridge reports this rather than opening a second window. The
fix is a calibration routine drawn with `stimuli/` that reports target positions
back over the protocol — not yet written.
