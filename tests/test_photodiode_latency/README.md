# test_photodiode_latency

Measures the delay between the timestamp `ShowTS` returns and the moment light
actually leaves the screen.

Each trial blanks the display, presents a white patch with `ShowTS`, and raises
a TTL line on a DLP-IO8 on the very next statement. An external two-channel
instrument records the TTL edge and a photodiode sitting on the patch. Because
both events land on the instrument's own timebase, its clock offset cancels —
you never have to reconcile the instrument's clock with the host's.

```bash
go run ./tests/test_photodiode_latency                      # fullscreen, diode at top-left
go run ./tests/test_photodiode_latency -diode center -trials 200
go run ./tests/test_photodiode_latency -isi-frames 15       # steady-state flipping
go run ./tests/test_photodiode_latency -calibrate           # LED zero point
```

## Wiring

- DLP-IO8 line 0 (`-line N` to change) to the instrument's TTL input, grounds
  common.
- Photodiode **on the patch**, on the screen. The patch is drawn where you say
  the diode is: `topleft`, `top`, `topright`, `left`, `center`, `right`,
  `bottomleft`, `bottom`, `bottomright`, or `x,y` in pixels from the screen
  centre.

  **Top-left is the default, and it is the right one for a latency question.**
  Scanout starts at the top-left corner and sweeps down, so a diode there adds
  the least scanout delay to the answer. Moving it to the bottom adds nearly a
  whole frame — 16.7 ms at 60 Hz — which is larger than everything else this
  test measures put together. Comparing `-diode topleft` against
  `-diode bottomleft` measures that sweep directly, if you want the number for
  your own panel.

## Which instrument

**Prefer an Analog Discovery 3.** Two reasons, and the second is the one that
matters:

1. Resolution: ~1 µs against a BBTK's ~250 µs.
2. **Both AD3 channels are the same ADC on the same timebase**, so there is no
   asymmetry between a "TTL input" and an "optical input" to worry about. On an
   instrument whose two inputs are different circuits, any difference in their
   internal latency is a systematic that does *not* cancel in the subtraction,
   and you would have to calibrate it out (see below).

The AD3 also records the whole waveform rather than a thresholded crossing,
which turns the LCD's rise time from a hidden systematic into a choice you make
in the analysis. `dlp-io8-g/measurements/ad3-capture.py` will capture it and
`ad3.py`'s `rising_edges` interpolates crossings to well under a sample.

A BBTK is still perfectly usable at millisecond precision, and its optical
sensor is purpose-built. If you have both, running both is genuine
corroboration by independent instruments.

## Reading the result

The instrument gives you `M = T_light − T_TTL`. What you want is
`Δ = T_light − T_ShowTS`, and:

```
Δ = M + gap + L_dlp
```

- **`gap`** is this program's own delay between `ShowTS` returning and the TTL
  write being issued. It is logged per trial as `gap_us`, so add it back. It is
  small only because the trigger is fired synchronously on the flip thread —
  measured here at **p50 34 µs, max 37 µs** at `SCHED_FIFO` 50. `triggers.FireTrigger`
  launches from a goroutine instead, and that path has been measured at +0.73 ms
  with about 1 ms of spread at normal priority under load, which is why this test
  does not use it.
- **`L_dlp`** is the DLP's own write-to-edge latency, which cannot be measured on
  this bench: nothing here shares a clock with the device and the module has no
  clock of its own. Bounded at **0.793 ms** by arithmetic alone and at a few tens
  of microseconds by extrapolating the FTDI latency-timer sweep — see
  [Why no absolute latency is quoted here](https://github.com/chrplr/dlp-io8-g#why-no-absolute-latency-is-quoted-here).

Both extra terms are non-negative, so **the instrument's figure alone is a lower
bound on Δ.**

## The systematic that dwarfs all of the above

`ShowTS` returns a host-side timestamp taken after the flip. The photons for a
given screen row appear when the scanout reaches that row, so a diode at the top
of the panel sees light almost immediately and one at the bottom nearly a frame
later — **16.7 ms at 60 Hz, sixteen times the millisecond this test is trying to
resolve.**

So: fix the diode's position, let the program draw the patch there, and note
that `patch_x`/`patch_y` are written into every row of the data. Two runs that
placed the diode differently are not comparable, and a run whose diode position
was not recorded is not interpretable.

Pixel response is the other one. An LCD takes milliseconds to go from black to
white, and where the threshold falls on that curve sets the answer. Report the
threshold, and prefer the instrument that lets you choose it after the fact.

## First measurements

Linux 7.0 / Wayland, 3840x2160 at 60 Hz (16.667 ms), `SCHED_FIFO` 50,
DLP-IO8 on AD3 CH1, a 9 V photodiode on CH2 through a 10x probe, patch 900 px at
screen centre, n=18 paired edges per condition. Thresholds are the midpoint of
each channel's own p5..p95, so neither depends on a constant.

| ISI style | min | median | max | spread |
|---|---|---|---|---|
| `-isi 250ms` (display idle between trials) | 15.4 | **23.0** | 31.3 | **15.8 ms** |
| `-isi-frames 15` (steady-state flipping) | 30.9 | **31.9** | 38.4 | **7.5 ms** |

Two things fall out, and neither is about the DLP.

**How you wait between trials changes the onset latency by about 9 ms.** Flipping
through the ISI keeps the pipeline in steady state and halves the jitter, but it
also builds a compositor queue, so the frame goes out a frame or so later. Sleeping
leaves the display idle: the first flip afterwards has nothing to block on, can
return at an arbitrary phase, and the light then waits for the next scanout —
lower latency, but up to a frame of spread. Neither is wrong; they are different
trade-offs, and an experiment should pick one deliberately and report it.

**Everything on the host side is negligible against this.** The ShowTS-to-TTL gap
ran at a median of 17-20 us and the DLP's own write-to-edge latency is tens of
microseconds. Both are three orders of magnitude below the 15-38 ms the display
pipeline costs. The corrections are real and the arithmetic above still applies,
but they will not change a conclusion at this scale.

The residual 7.5 ms of spread in the frame-locked condition is not explained here.
It could be the LCD's own rise varying, the diode's position relative to scanout,
or compositor scheduling; n=18 on one display and one compositor is not enough to
separate them, and no attempt is made to.

## Zero point (`-calibrate`)

Draws nothing and emits TTL pulses. Wire an LED and its resistor to the same TTL
line and point the photodiode at the LED. The interval the instrument then
reports is its own TTL-versus-optical input asymmetry plus the LED's rise, and an
LED rises in microseconds — so that figure is the offset to subtract from the
real runs.

Worth doing on a BBTK. Worth doing on an AD3 too, once, to confirm the
asymmetry is negligible rather than assume it.

## Flags

| flag | default | meaning |
|---|---|---|
| `-trials N` | 100 | number of trials |
| `-line N` | 0 | DLP-IO8 output line |
| `-port` | auto | serial port of the DLP-IO8 |
| `-diode` | `topleft` | 4 corners, 4 edge midpoints, `center`, or `x,y` in px |
| `-patch N` | 240 | side of the white patch, in px |
| `-frames N` | 2 | frames the patch stays on |
| `-isi D` | 500ms | blank interval between trials (sleep) |
| `-isi-frames N` | 0 | if >0, blank by flipping N frames instead of sleeping |
| `-calibrate` | off | LED zero-point mode |

Plus the usual `-w` (windowed), `-d N` (display), `-s ID` (subject), and
`-no-realtime` / `-realtime-priority N`.

**Run it at real-time priority.** The program prints the policy it actually got
and warns if `gap_us` ever exceeds 1 ms, which is the point at which the trials
are measuring the scheduler rather than the display. See
[docs/SettingPriorityUnderLinux.md](../../docs/SettingPriorityUnderLinux.md) —
including the warning there about not combining `chrt` with a continuous
busy-wait.

## Output

A CSV in `~/goxpy_data/` with one row per trial: `trial`, `flip_ts_ns`,
`trigger_ts_ns`, `gap_us`, `patch_x`, `patch_y`, `patch_px`, `frames`, `policy`,
`priority`, `isi_ms`, `onset_interval_ms`.

`onset_interval_ms` is onset to onset, spanning the ISI and the whole trial —
not a frame interval. It is there so that a trial which took much longer than
its neighbours is visible in the data rather than silently averaged in.
