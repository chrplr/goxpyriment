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
- Photodiode **on a patch**, on the screen. By default (`-diode all`) a patch is
  drawn at each of the four corners and the centre, so the diode can go
  anywhere and be moved between runs without changing anything. `-diode
  corners`, a single name, several joined with `+` (`topleft+center`), or one
  `x,y` pair in pixels also work.

  **Top-left is the position a latency question wants.** Scanout begins at the
  top-left corner and sweeps down, so a diode there adds the least scanout
  delay. Measured on a 60 Hz panel here: median TTL-to-light of **19.0 ms** at
  the top-left against **23.0 ms** at the centre. A diode at the bottom would add
  nearly a whole frame, 16.7 ms, which is larger than everything else this test
  measures put together.

  Note `+y is up`: `Screen.CenterToSDL` computes `centreY - y`. And positions
  are in the renderer's **logical** space, which is not always the panel's pixel
  count: a 3840x2160 display at 125% desktop scaling presents as 2304x1296, and
  a patch placed by pixel count would land off-screen. The program reads the
  space at startup rather than assuming it, and prints it with every patch
  centre — check those against what you actually see on the panel, because a
  wrong position is silent otherwise.

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

### Measured, n=34, 1 MS/s, 9 V photodiode behind a 10x probe

| threshold | TTL rise -> light | spread |
|---|---|---|
| 10% | **17.9 us** | 9.8 us |
| 20% | 45.0 us | 15.3 us |
| 50% | **133.7 us** | 11.9 us |
| 80% | 243.1 us | 13.5 us |
| 90% | 290.8 us | 15.1 us |

**The DLP's own latency cancels out of this.** The LED is driven from the same
TTL line the instrument watches, so the write-to-edge delay is common to both
signals and subtracts away. What is left is the photodiode's rise plus whatever
asymmetry the instrument has between its two inputs.

Those two are not separable here, but the smaller bounds the pair: **the AD3's
channel asymmetry is at most 17.9 us**, and since both channels are the same ADC
it is presumably far less than that, with the rest being the diode. The 10% to
90% span, **273 us**, is the photodiode module's rise time and nothing else --
worth knowing on its own, since it is the resolution floor of any optical
measurement made with it.

Against the 20 ms the display pipeline costs, a 134 us correction at the 50%
level is 0.7%. Small, but now measured rather than assumed.

**Do not transfer this number to the screen measurement unchanged**, and the
reason turned out not to be the one expected. Measured against the screen patch
instead of the LED, same diode and same probe:

| source | step amplitude | 10-90% rise |
|---|---|---|
| LED at close range | 8.0 V | **273 us** |
| white patch on the panel | 7.4 V | **6500 us** |

Twenty-four times slower, at essentially the same amplitude. Since the diode
reaches the same voltage either way, it is not the diode being starved of light:
**the 6.5 ms is the panel's own black-to-white pixel transition.**

A photodiode covers a few pixels, not the patch. So the rise it reports is the
temporal response of *those* pixels, with no spatial averaging over the rest of
the square -- which makes 6.5 ms a clean measurement of the transition at one
point, rather than a blur of when different parts of the patch lit up.

Patch height was measured across 800, 400, 200 and 100 px and the rise did not
move: 6578, 6514, 6621, 6440 us. That is a useful negative control -- it
confirms the diode is well inside the patch at every size and that nothing about
the geometry leaks into the number -- but it is not evidence about scanout, and
an earlier version of this file wrongly presented it as such. Sweeping scanout
across the patch could only lengthen the rise for a sensor that integrated the
whole square, and this one does not. The patch's top edge also sits 8 px from
the screen edge at every size, since the inset is `patch/2 + 8`, so the diode's
own pixels are reached at the same moment regardless.

The practical upshot is unchanged: patch size is not a lever on edge sharpness.
Make it large enough that the diode sits comfortably inside, and stop there.

### The same, on a second panel

The 6.5 ms above is one monitor, so it was repeated on the laptop's internal
display. Same host, same diode, same probe, same 60 Hz:

| | Dell DP-5, 3840x2160 | laptop eDP-1, 2560x1600 |
|---|---|---|
| nominal frame | 16.667 ms | 16.656 ms |
| step amplitude at the diode | 7.44 V | 7.07 V |
| **10-90% rise** | **6500 us** | **5571 us** |
| onset, TTL to 10% | ~10-18 ms | 38.0 ms |

**The slow rise is not one bad monitor.** Two unrelated LCDs, one external and
one built into a laptop, differ by 14% on a quantity spanning milliseconds. A
multi-millisecond black-to-white transition looks like a property of the panel
technology rather than of this particular Dell, which makes it the general case
to design around rather than a local nuisance.

**The latency is not transferable at all.** The onset differs by more than a
frame between the two, so the delay from ShowTS to light has to be measured on
the display an experiment will actually use. Two caveats on that comparison: the
Dell figure is poorly determined, since those runs used the sleep-based ISI whose
~16 ms of phase jitter with n=12 leaves the median wandering by several
milliseconds, and the diode was not at an identical height on the two panels, so
part of the difference is scanout position rather than pipeline depth. The rise
does not suffer from either, being a difference measured within each trial --
which is why its spread is 35 us across 12 trials on the laptop.

**What this means for the measurement.** A 6.5 ms rise is a floor on how sharply
a visual onset can be defined on this monitor, and it is the dominant remaining
uncertainty: where the threshold sits on that ramp moves the answer by
milliseconds, which is why this test reports the level it used. It also swamps
every correction discussed above -- the 134 us zero point, the 17 us
ShowTS-to-TTL gap, the tens of microseconds of DLP latency. On a panel like this
one, quoting a visual onset to better than a millisecond is not meaningful
without saying which point on the rise is meant.

## Flags

| flag | default | meaning |
|---|---|---|
| `-trials N` | 100 | number of trials |
| `-line N` | 0 | DLP-IO8 output line |
| `-port` | auto | serial port of the DLP-IO8 |
| `-diode` | `all` | `all`, `corners`, names joined with `+`, or `x,y` in px (+y up) |
| `-patch N` | 240 | side of the white patch, in px |
| `-frames N` | 2 | frames the patch stays on |
| `-isi D` | 500ms | blank interval between trials (sleep) |
| `-isi-frames N` | 0 | if >0, blank by flipping N frames instead of sleeping |
| `-calibrate` | off | LED zero-point mode |

| `-no-prompt` | off | skip the calibration instruction screen and pulse at once |

Plus the usual `-w` (windowed), `-d N` (display), `-s ID` (subject), and
`-no-realtime` / `-realtime-priority N`.

**Pass `-s` for any unattended run.** Without it `NewExperimentFromFlags` opens
the participant-info dialog and waits, which from a script looks exactly like a
hang: no output at all, because the dialog blocks before the first line is
printed. `-s 1 -no-prompt` together give a run that starts pulsing immediately
and can be synchronised to by watching its stdout for `emitting`.

**Run it at real-time priority.** The program prints the policy it actually got
and warns if `gap_us` ever exceeds 1 ms, which is the point at which the trials
are measuring the scheduler rather than the display. See
[docs/SettingPriorityUnderLinux.md](../../docs/SettingPriorityUnderLinux.md) —
including the warning there about not combining `chrt` with a continuous
busy-wait.

## Output

A CSV in `~/goxpy_data/` with one row per trial: `trial`, `flip_ts_ns`,
`trigger_ts_ns`, `gap_us`, `patch_positions`, `patch_xy`, `patch_px`, `frames`, `policy`,
`priority`, `isi_ms`, `onset_interval_ms`.

`onset_interval_ms` is onset to onset, spanning the ISI and the whole trial —
not a frame interval. It is there so that a trial which took much longer than
its neighbours is visible in the data rather than silently averaged in.
