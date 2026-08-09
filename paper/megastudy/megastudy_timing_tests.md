# Reproducing the Bridges et al. (2020) timing mega-study with `tests/Timing-Tests`

Reference: Bridges D, Pitiot A, MacAskill MR, Peirce JW (2020). *The timing
mega-study: comparing a range of experiment generators, both lab-based and
online.* PeerJ 8:e9414. DOI [10.7717/peerj.9414](https://doi.org/10.7717/peerj.9414)
(open access; their experiment files are at <https://osf.io/3kx7g/>)

This note maps the paper's protocol onto the flags of
`tests/Timing-Tests`, so that goxpyriment measurements can be placed
alongside the published PsychoPy / Psychtoolbox / Presentation / E-Prime /
OpenSesame / Expyriment figures.

---

## What Bridges et al. actually ran

Two separate 1,000-trial runs on a 60 Hz monitor (AOC 238LM00023/I2490VXQ
23.8″, 1920 × 1080, 4 ms response time), measured with a Black Box Toolkit v2.

| | **Stimulus-timing run** (BBTK *DSC* mode) | **Response-time run** (BBTK *DSRE* mode) |
|---|---|---|
| Trial | 300 ms black → **simultaneously**: TTL pulse + white square (top-centre) + 440 Hz tone, all 200 ms | 300 ms black → white square (top-centre) for 200 ms |
| Period | 500 ms | 500 ms |
| Trials | 1,000 | 1,000 |
| Lead-in | 10 s blank screen | 5 s pause |
| TTL / audio | TTL via LabHackers USB2TTL8; 440 Hz tone | neither |
| Response | — | BBTK robotic key actuator, programmed to respond 100 ms after onset, 50 ms keypress, onto a LabHackers MilliKey (1 kHz USB response box) |

At 60 Hz: **300 ms = 18 frames**, **200 ms = 12 frames**, period = 30 frames.

Measures reported: stimulus duration, stimulus onset relative to the TTL,
absolute audio onset relative to the same TTL, audiovisual synchrony, and
reaction-time measurement error.

---

## Commands

First establish the real refresh rate — every duration below assumes 60 Hz:

```bash
go run ./tests/Timing-Tests -test display -duration-s 30
```

### 1. The stimulus-timing run — visual, audio and TTL together

This is their whole DSC run: duration, onset vs. TTL, audio onset vs. TTL and
audiovisual synchrony all come out of this one command.

```bash
go run ./tests/Timing-Tests -test av \
    -frames-on 12 -frames-off 18 -cycles 1010 -warmup 10 \
    -hz 60 -freq-hz 440 -soa-ms 0 \
    -level-a 0 -level-b 255 \
    -trigger-ms 200 -trigger-pin 1 -audio-frames 256
```

Notes on the flags that are not self-explanatory:

- **`-freq-hz 440` is not optional.** The binary defaults to **1000 Hz**.
  Frequency does not move the onset, but it moves where a microphone or
  line-out threshold is crossed, so a 1 kHz run and a 440 Hz run are not
  interchangeable in a comparison against their Table 2.
- **`-cycles 1010 -warmup 10` gives 1000 *analysed* trials.** Warm-up cycles
  are subtracted from the total, not added to it, so `-cycles 1000` yields 990.
  The warm-up cycles are still presented — they flash and fire the TTL — so
  they lengthen the capture window too.
- `av` derives the tone duration from `frames-on × 1000/hz`, so `-hz 60` with
  `-frames-on 12` yields exactly the 200 ms tone. There is no `-tone-ms` flag;
  `-iti-ms` exists but applies only to `rt`.
- `-soa-ms 0` selects the `PlaySyncedWithFlip` path, the closest analogue to
  the paper's "presented simultaneously".

### 2. Visual path in isolation (diagnostic, not one of their measures)

Same stimulus with the audio device and the trigger dropped. Useful when the
combined run looks wrong and you need to know whether the display or the audio
path is responsible.

```bash
go run ./tests/Timing-Tests -test av -no-sound -no-ttl \
    -frames-on 12 -frames-off 18 -cycles 1010 -warmup 10
```

### 3. Reaction time

```bash
go run ./tests/Timing-Tests -test rt -cycles 1000 -iti-ms 300
```

### 4. (Optional) audio pipeline in isolation

Not a separate run in the paper. `latency` measures how long the audio pipeline
takes to drain a tone of each of several durations — a property of the buffer
and the backend, *not* a tone-onset jitter measure at a 500 ms period:

```bash
go run ./tests/Timing-Tests -test latency -audio-frames 256 -drain-reps 20
```

Tone-onset jitter over a long session comes from the microphone or line-out
channel of the `av` recording, not from a separate stimulus run.

### Or run the whole session at once

`tests/Timing-Tests/run-timing-tests.sh` encodes all of the above as its
defaults (440 Hz, 1010 cycles, 10 warm-up, 12/18 frames), runs the `av` step as
a GC-suspended / GC-running pair, and drives the BBTK capture when
`BBTK_CAPTURE=1`. Prefer it over the hand-typed commands: it also records the
display, fullscreen mode and audio buffer identically across every step, which
is what keeps the steps comparable.

---

## Running conditions

Run everything **fullscreen** — do not pass `-w`. Windowed mode hands frames
to the desktop compositor and the resulting numbers describe the compositor,
not goxpyriment.

There is no longer a presentation path to choose between: `-paced-flip` and the
separate paced variants were removed on 2026-08-05, and `Screen.Update()` now
paces every flip. That single path is the ordinary API a naive user gets, which
is what Bridges et al. deliberately measured ("we created the experiments in the
manner that might be expected from a normal user of each package … and therefore
excluded advanced, undocumented code additions to optimize performance").

Hardware, per the paper: photodiode at the top-centre of the display,
TTL line on a second scope channel, audio taken from the 3.5 mm line-out.

---

## Where the match is imperfect

- **Patch position and count.** `av` does not flash the whole screen: it draws
  **five** square patches, at the four corners and the centre, each sized to a
  quarter of the render height unless `-square-px` says otherwise. Bridges et
  al. used **one** 0.25 × 0.25 square at top-centre. There is no flag that
  reduces the count to one, so the panel sees roughly five times their
  luminance step, and there is no top-centre patch at all. Put the photodiode
  on the **top-left** patch: scanout starts there, so it is the earliest and
  the least position-dependent of the five, and it is within a few milliseconds
  of their top-centre. Record which patch you used — the bottom of the panel
  lights nearly a frame after the top.

  (This note previously said these sub-tests clear the entire screen. That was
  true when it was written and is no longer.)

- **No lead-in blank.** Their DSC run began with 10 s of blank screen while the
  BBTK initialised. `av` starts bright on cycle 0, immediately after the window
  opens, and `-warmup` does *not* create a quiet period: those cycles are
  presented and fire the TTL like any other, and are merely excluded from the
  software statistics. The `BBTK_MARGIN_S=8` in `run-timing-tests.sh` pads the
  *recording*, not the stimulus. The first trial also has no preceding dark
  phase — the cycle runs bright-then-dark, so the 300 ms black precedes every
  trial except the first.

- **The TTL edge is not the flip instant.** `runAV` calls `fire()` *after*
  `flip()` returns, and `fire()` spawns a goroutine that then writes to the
  device. Bridges used PsychoPy's `callOnFlip()` into a LabHackers USB2TTL8.
  Their headline metric is the inter-trial SD, so this matters more than a
  constant offset would: goroutine scheduling and the write itself land
  *inside* the "stimulus onset vs. TTL" precision figure rather than cancelling
  out of it.

  The write half of that is now avoidable. `-trigger-device` takes `parallel`
  (LPT via ppdev) and `gpio` (Linux GPIO character device) as well as the
  default `dlpio8`; both are a local ioctl and carry none of the USB-serial
  latency. On a desktop with an LPT port, `-trigger-device parallel` is the
  closest available analogue to their USB2TTL8; on a Raspberry Pi, use
  `-trigger-device gpio` and check that the recorder triggers on 3.3 V. The
  goroutine hop remains on all three.

- **The `rt` ITI is jittered by ±50 %** (`main.go`, `runRT`), so `-iti-ms 300`
  produces 150–450 ms rather than their fixed 300 ms. `rt` needs a real
  keypress, and there is no built-in 100 ms-after-onset robotic responder.
  Reproducing their RT protocol exactly requires the BBTK actuator plus a small
  patch removing the jitter. (`rt` draws the same five patches as `av` — it
  shares `newPainter` — so the patch-position note above applies to it too.
  This note previously said `rt` flashes the whole screen; that is no longer
  true.)

- **No 195 ms fudge is needed.** The paper had to request 195 ms in
  Presentation and Expyriment because those packages overshoot the requested
  duration by one refresh. goxpyriment takes durations in *frames*, so
  `-frames-on 12` is 12 refreshes by construction. This is a genuine
  difference in favour of the frame-based API, not a discrepancy to correct.

- **Trigger pulse width.** `-trigger-ms 200` mirrors their 200 ms TTL, but only
  the leading edge is meaningful, for the reason in the bullet above; the
  oscilloscope is what actually measures the trigger→luminance lag.

- **Trigger hardware differs.** They used a LabHackers USB2TTL8; the
  goxpyriment tests default to a DLP-IO8-G. If you stay on the default, set the
  USB latency timer to 1 ms
  (`echo 1 | sudo tee /sys/bus/usb-serial/devices/ttyUSB0/latency_timer`)
  before comparing absolute onset-vs-trigger values — or sidestep the link
  entirely with `-trigger-device parallel`, per the bullet above. Whichever was
  used is recorded in the results header as `trigger=…`; a session that does
  not state it cannot be compared with one that does.

- **Response device.** Their 1 kHz MilliKey button box removes most keyboard
  latency; a standard USB keyboard adds 20–40 ms (Neath et al., 2011) and
  inflates the `rt` figures accordingly.

---

# Results

Measured 8 August 2026, and written up under `docs/` rather than here, since
that is where the published documentation lives:

**[`docs/TimingMegastudyComparison.md`](../../docs/TimingMegastudyComparison.md)**

**Provenance caveat.** Those sessions predate the 2026-08-09 change to
`run-timing-tests.sh` and so were recorded with the binary's **1000 Hz**
default tone, not the 440 Hz above, and with 990 analysed trials rather than
1000. Neither affects the visual figures. For the audio row, the frequency
determines where the microphone or line-out threshold is crossed, so those
numbers should not be placed beside their Table 2 without either re-measuring
at 440 Hz or stating the difference. Each run's `-info.txt` header records
`freq-hz`, so a session can always be told apart from a later one after the
fact.
