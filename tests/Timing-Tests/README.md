# Timing-Tests — quick reference

A hardware timing verification suite for goxpyriment experiments. Run these
**before** collecting data, to characterise your system's display and audio
timing and to verify that stimulus presentation behaves as intended.

For background, equipment setup and interpretation see
**[docs/TimingTests.md](../../docs/TimingTests.md)**.

---

## The console numbers are not the answer

Every statistic these tests print comes from software timestamps — when
goxpyriment *believes* a flip happened. Those stayed textbook-perfect throughout
a presentation bug that left a panel showing stale frames for seconds at a time.

**Judge timing from a photodiode and TTL recording.** Treat the console output as
a cross-check on the software side only.

---

## Recommended workflow

```
No hardware needed
  1. check    — can this machine display and make noise at all?
  2. display  — true refresh rate and frame-interval stability
  3. latency  — audio pipeline delay

Photodiode and/or trigger box
  4. av       — THE stimulus timing test: visual + audio + TTL
  5. vrr      — arbitrary durations, not just multiples of a frame
  6. rt       — reaction-time timestamp precision
```

`av` is the default: running with no `-test` flag runs it.

---

## Running

> **Run every test in fullscreen — that is the default. Do not pass `-w`.**
>
> Windowed mode hands your frames to the desktop compositor, which adds a
> variable presentation delay, may drop or duplicate frames, and on some drivers
> stops blocking on VSYNC altogether. Measurements taken windowed describe the
> compositor, not goxpyriment, and are not comparable between machines.
>
> `-w` exists only for developing and debugging the tests themselves. Any number
> reported from a windowed run should be treated as invalid. If a test drops
> frames windowed but not fullscreen, that is the compositor — re-measure before
> investigating anything else.

```bash
# from the repo root (go.work resolves both modules):
go run ./tests/Timing-Tests [flags]

# examples
go run ./tests/Timing-Tests                                          # av, the default
go run ./tests/Timing-Tests -frames-on 12 -frames-off 18 -cycles 100
go run ./tests/Timing-Tests -no-sound -no-ttl                        # visual only, no hardware
go run ./tests/Timing-Tests -soa-ms -50                              # audio leads by 50 ms
go run ./tests/Timing-Tests -frames-on 1 -frames-off 60 -cycles 60   # single-frame flashes
go run ./tests/Timing-Tests -test check
go run ./tests/Timing-Tests -test display -duration-s 30
go run ./tests/Timing-Tests -test latency -audio-frames 256 -drain-reps 20
go run ./tests/Timing-Tests -test vrr     # ~20 s with the defaults
go run ./tests/Timing-Tests -test rt      -cycles 60
go run ./tests/Timing-Tests -sysinfo                                 # config snapshot, then exit
```

Results go to `~/goxpy_data/` by default — or wherever `-outdir` points — as a
`.csv` plus an `-info.txt` header recording the
display and audio configuration actually in use — including **which monitor** the
window opened on. Read geometry from there rather than assuming; on a
multi-monitor machine the displays differ in every relevant parameter.

---

## The `av` test

Every cycle presents all three modalities at once:

| Modality | What happens |
|---|---|
| Visual | Five squares (four corners + centre) bright for `-frames-on` frames, dark for `-frames-off` |
| Audio | Tone of `frames-on × frame-period`, synced to the visual onset (or offset by `-soa-ms`) |
| TTL | A `-trigger-ms` pulse at the visual onset |

`-no-sound` and `-no-ttl` drop a modality. 

**Placing the photodiodes.** An LCD paints top to bottom, so a bottom square
lights close to a full frame period after a top one. Put one photodiode on a top
square and another on a bottom square: the difference is your panel's scan-out
gradient — on the reference rig, 13.15 ms across 80 % of a 1280×1024 screen, or
15.96 µs per pixel row. Two squares on the same row are microseconds apart and
serve as a sanity check. Each display needs its own calibration.

Each diode must sit at its square's **centre**: the gradient is divided by the
top↔bottom separation, and that figure assumes it. The test prints the centres
and the separation at startup and records them in the `-info.txt` header:

```
stimulus: five 400 px squares (corners + centre) on a 2560x1600 render area
  centres:  x = 200 / 1280 / 2360    y = 200 / 800 / 1400
  top↔bottom separation: 1200 px = 0.7500 of screen height
```

**The default size scales with the panel** — a quarter of the render height, so
the centres land at ⅛ and ⅞ and the separation is **always 0.750**, on any
display. That also keeps each centre well clear of the bezel, which matters: a
diode that cannot lie flat at the centre ends up further out, the real separation
exceeds the printed one, and the derived sweep comes out too large. If it exceeds
one frame period, that is the tell — scan-out cannot outlast the frame it belongs
to. `-square-px` overrides the default when you need a specific size.

**Discard the warm-up.** The first several cycles run long before settling
(36–39 ms falling to ~27 ms on the reference rig). `-warmup` excludes them from
the test's own statistics, but offline tools do not know about it — quote
steady-state numbers and say which cycles you used.

---

## Recording with a BBTK

`run-timing-tests.sh` can drive `bbtk-capture` for you, so the stimulus always
falls inside the capture window without two terminals and a stopwatch:

```bash
BBTK_CAPTURE=1 BBTK_CAPTURE_BIN=~/00_git/bbtkv3/_build/bbtk-capture \
    ./run-timing-tests.sh av-visual
```

| Variable | Default | Meaning |
|---|---|---|
| `BBTK_CAPTURE` | off | Set to `1` to record the photodiode steps |
| `BBTK_CAPTURE_BIN` | `bbtk-capture` | Path to the binary, if not on `PATH` |
| `BBTK_PORT` | auto | Serial port; auto-detected, see below |
| `BBTK_MARGIN_S` | 8 | Seconds recorded either side of the stimulus |
| `BBTK_READY_TIMEOUT_S` | 120 | Give up waiting for the device |
| `SQUARE_PX` | 0 | Stimulus square side, px; 0 = ¼ of the render height |
| `CYCLES` | 1000 | Cycles per `av` step — lower it for a quick placement check |
| `FRAMES_ON` | 12 | Bright frames per cycle |
| `FRAMES_OFF` | 18 | Dark frames per cycle |

`FRAMES_ON`/`FRAMES_OFF` resize the BBTK capture window automatically, so a
shortened cycle no longer needs the window recomputed by hand. Two caveats when
you shorten it:

- **Cycles are not exposure.** GC pauses arrive per unit time, so 1000 cycles of
  4 frames is ~67 s against ~500 s for 1000 cycles of 30. Scale `CYCLES` up to
  match the wall-clock of the run you are comparing against, or the shorter run
  flatters the collector purely by sampling less of it.
- **Check the panel keeps up.** Opto events run ~20 ms longer than the stimulus
  (threshold hysteresis plus LCD decay) — more than one frame at 60 Hz. Pilot any
  `FRAMES_ON=1` configuration with a small `CYCLES` and confirm
  N(Opto1) = N(TTLin1) before committing to a long capture, or you are measuring
  display response rather than the framework. The script prints a warning at
  `FRAMES_ON` ≤ 2.

Only the photodiode steps (`av`, `av-gc`, `av-visual`) are wrapped; `check`,
`display` and `latency` measure nothing the BBTK can see.

**The port is found for you.** `bbtk-capture` resolves the device through its
stable `/dev/serial/by-id/*BBTK*` symlink, which survives replugging and
power-cycling — unlike `/dev/ttyUSBn`, whose numbering is assigned in enumeration
order. Set `BBTK_PORT` only to override that.

**Get the duration right first time.** An interrupted capture is lost: the BBTK
streams its data only when the programmed window completes, so there is no way to
stop early and keep what was recorded. The script sizes the window from the
step's own parameters, but if you interrupt a run, that recording is gone and the
step has to be repeated.

**Why it waits.** `bbtk-capture` needs 11–40 s between launch and the device
actually recording — fixed command pacing, plus an internal-memory erase whose
duration depends on whether the box needs a full format. Its own "Capturing
events…" message is printed several seconds *before* recording starts, so the
script instead blocks on the `BBTK-CAPTURE-READY` line that `bbtk-capture` emits
the instant it sends `RUDS`. Budget that startup per step.

Ctrl-C is safe: `bbtk-capture` traps it, stops the capture, and still writes what
the device recorded.

Everything a session produces lands in one directory under wherever you launched
the script — `reports-<host>/` by default, or set `OUTDIR` to keep separate
sessions apart:

```
reports-is158520/
  av-visual.txt                              console report
  Timing-Tests_sub-000_date-...csv           per-cycle data  (via -outdir)
  Timing-Tests_sub-000_date-...-info.txt     display/audio configuration
  bbtk-av-visual.log                         raw bbtk-capture output
  bbtk-av-visual-001-events.csv              BBTK events
  bbtk-av-visual-001-dscevents.csv
  bbtk-av-visual-001.dat
```

The script passes `-outdir` to every run, so the data files sit beside the
reports and captures instead of accumulating in `~/goxpy_data`.

---

## Microphone coupling

If you are measuring audio, **max the output volume and put the BBTK microphone
within a few millimetres of the speaker membrane.** Anything less and events are
silently missed — there is no warning, and the run looks like it worked.

Two checks on the resulting `-events.csv`:

- **N(Mic1) should equal N(TTLin1).** Far fewer means bad coupling, not bad
  timing. A run of 197 cycles that yields 6 Mic events is a placement problem.
- **Mic duration should be close to `frames-on × frame period` + 20 ms** — about
  **220 ms** at the defaults, not 200. The BBTK's smoothing filter holds the line
  high for ~20 ms past the true falling edge on every channel it covers (`Mic1`,
  `Mic2`, `Opto1`–`Opto4`); `TTLin1` is outside the mask and so reads true. Recent
  captures carry a `DurationCorrected` column with that tail already removed —
  compare *that* against 200 ms, or subtract 20 ms yourself from `Duration` on
  older three-column files.

  A genuinely short pulse — say 20 ms for a 200 ms tone — still means the
  threshold is catching only the loudest fraction of the tone. Judge that after
  accounting for the offset, not before.

The same +20 ms applies to `Opto1`/`Opto2`, so photodiode durations read ~220 ms
for a 200 ms stimulus. It is a fixed instrument offset, independent of stimulus
length: do not read it as LCD rise/decay or as a threshold problem. **Onsets are
unaffected** — smoothing does not delay the leading edge — so every latency in
these tests (TTL→photodiode, scan-out, AV sync) stands as recorded.

---

## Flags

### Common

| Flag | Default | Meaning |
|---|---|---|
| `-test` | `av` | `av \| vrr \| rt \| check \| display \| latency` |
| `-cycles` | 120 | Number of cycles (`av`, `rt`) |
| `-vrr-max-ms` | 20 | Longest duration the `vrr` sweep targets, in 1 ms steps |
| `-vrr-reps` | 5 | Repetitions per duration step (`vrr`) |
| `-warmup` | 10 | Leading cycles discarded from statistics |
| `-trigger-device` | `dlpio8` | TTL output: `dlpio8` (USB serial) \| `parallel` (LPT via ppdev) \| `gpio` (Linux GPIO chip) |
| `-port` | auto | Serial port for the DLP-IO8-G (`-trigger-device dlpio8`) |
| `-parallel-port` | auto | LPT device, e.g. `/dev/parport0` (`-trigger-device parallel`) |
| `-gpio-chip` | `/dev/gpiochip0` | GPIO chip device (`-trigger-device gpio`) |
| `-gpio-pins` | `17,27,22,5,6,13,19,26` | The 8 GPIO output lines, chip-relative — BCM numbers on a Pi (`-trigger-device gpio`) |
| `-trigger-pin` | 1 | Output pin (1–8) — see the note below on what it names |
| `-trigger-ms` | 5 | Trigger pulse duration, ms |
| `-d` | -1 | Display index (-1 = primary) |
| `-w` | false | Windowed mode — debugging only, never for measurement |
| `-sysinfo` | false | Print system information and exit |
| `-gc` | false | Leave the collector running (suspended by default); run twice to measure its effect |

### `av`

| Flag | Default | Meaning |
|---|---|---|
| `-frames-on` | 12 | Bright frames per cycle (12 = 200 ms at 60 Hz) |
| `-frames-off` | 18 | Dark frames per cycle (18 = 300 ms at 60 Hz) |
| `-square-px` | 0 | Side of each of the five squares, px; 0 = ¼ of the render height |
| `-level-a` | 0 | Dark luminance 0–255 (surround) |
| `-level-b` | 255 | Bright luminance 0–255 (squares) |
| `-soa-ms` | 0 | Visual-to-audio SOA; negative = audio first |
| `-freq-hz` | 1000 | Tone frequency, Hz |
| `-hz` | 60 | Refresh rate used to derive the tone duration |
| `-no-sound` | false | Do not play the tone |
| `-no-ttl` | false | Do not fire the trigger |
| `-audio-frames` | 0 | Audio buffer, sample frames (0 = SDL default) |

`-audio-frames` sets the floor on audio-onset precision: 256 frames at 44100 Hz
quantises tone onsets to 5.8 ms steps, 512 frames to 11.6 ms.

Smaller is not automatically better. `run-timing-tests.sh` passes **512**
(`AUDIO_BUFFSIZE`) because 256 underran repeatedly on the ALSA path, and a
dropped buffer disturbs tone onsets far more than the coarser quantisation does.
Lower it only after confirming the underruns are gone.

### Other tests

| Test | Flags |
|---|---|
| `display` | `-duration-s` (10) |
| `latency` | `-freq-hz` (1000), `-drain-reps` (10) |
| `vrr` | `-vrr-max-ms` (50), `-cycles` |
| `rt` | `-iti-ms` (1000) — mean ITI, jittered ±50 % |

---

## Equipment

| Test | Needs |
|---|---|
| `check`, `display`, `latency` | Nothing |
| `av` | Photodiode + a TTL output device (either alone still works — use `-no-sound` / `-no-ttl`) |
| `vrr` | Photodiode; a VRR-capable display to see any benefit |
| `rt` | Keyboard or USB response box |

A Black Box ToolKit, an oscilloscope, or any multi-channel recorder works for the
photodiode and TTL channels.

### Choosing a TTL output device

`-trigger-device` selects among three. They are not interchangeable: the trigger
is fired right after the flip returns, so whatever the write costs lands between
the flip and the TTL edge, and in the trial-to-trial *spread* of that interval
rather than as a constant offset that would cancel.

| Device | Path to the pin | Logic | Use when |
|---|---|---|---|
| `dlpio8` | USB serial (FTDI) | 5 V | It is the box you have. Set the FTDI latency timer to 1 ms first — see the DLP-IO8 section of [`docs/TimingTests.md`](../../docs/TimingTests.md) |
| `parallel` | `ioctl` on ppdev | 5 V | The machine still has an LPT port — the lowest-latency option on a desktop |
| `gpio` | `ioctl` on a GPIO chip | **3.3 V** | Raspberry Pi, Rock Pi, or any SBC with a GPIO header |

`-trigger-pin` is 1–8 for all three but names something different in each, so
check what the program prints and probe *that* pin:

- **dlpio8** — the number printed on the terminal block.
- **parallel** — a data line: pin 1 is D0, which is **DB25 pin 2** (D0–D7 are
  DB25 pins 2–9). Ground is any of DB25 pins 18–25.
- **gpio** — a *position in `-gpio-pins`*, not a BCM number: with the default
  list, pin 1 is **BCM 17**.

Prerequisites on Linux: `parallel` needs the `ppdev` module (`sudo modprobe
ppdev`) and membership of the `lp` group; `gpio` needs kernel ≥ 5.10 and
membership of the `gpio` group. Both require a re-login after `usermod`. Verify
the wiring with [`../test_parallel_port`](../test_parallel_port) or
[`../test_linuxgpio`](../test_linuxgpio) before a long capture.

**3.3 V is not TTL.** Confirm your recorder actually triggers on the GPIO swing
with a short run before committing to a 1000-cycle capture.

Whichever is used is written to the results header as `trigger=…`, since the
three do not produce the same onset-vs-TTL figure.

---

## Related tests

- [`../test_gv_sync`](../test_gv_sync) — `.gv` playback synchronisation
- [`../test_dlpio8`](../test_dlpio8) — DLP-IO8-G square-wave characterisation
- [`../test_clear_only_frames`](../test_clear_only_frames) — regression test for
  the compositor presentation bug
- [`../test_vsync_blocking`](../test_vsync_blocking) — does `SDL_RenderPresent`
  block on VSYNC here?
