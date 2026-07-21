# Timing-Tests — quick reference

A hardware timing verification suite for goxpyriment experiments.
Run these tests **before** collecting data to characterise your system's
display and audio timing and to verify that stimulus presentation is
behaving as intended.

For full background, equipment setup, interpretation guidance, and worked
examples see **[docs/TimingTests.md](../../docs/TimingTests.md)**.

---

## Recommended workflow

```
1. check    — verify display flash + audio output
2. display  — measure true refresh rate and frame stability
3. latency  — measure audio pipeline latency
4. stream   — verify RSVP / sequential-stimulus timing
5. vrr      — Variable Refresh Rate sweep: 1–N ms in 1 ms steps
6. trigger  — characterise DLP-IO8-G (if available)
7. frames   — validate visual onset and phase duration with photodiode
              (use frames-on=1 for single-frame / minimum-duration testing)
8. gvsync   — validate .gv video onset vs. trigger alignment with photodiode
9. tones    — measure audio onset jitter (long stream)
10. av      — measure audio–visual synchrony
11. rt      — measure reaction-time timestamp precision
```

Steps 1–5 require no external hardware (step 5 benefits from a VRR monitor).
Steps 6–10 require a DLP-IO8-G and/or oscilloscope + photodiode (see docs).
Step 11 requires a keyboard or USB response box.

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
go run ./tests/Timing-Tests -test <name> [flags]

# examples
go run ./tests/Timing-Tests -test check  
go run ./tests/Timing-Tests -test display -duration-s 30 
go run ./tests/Timing-Tests -test latency 
go run ./tests/Timing-Tests -test stream  -cycles 120 -frames-on 3 -frames-off 3 
go run ./tests/Timing-Tests -test vrr     -vrr-max-ms 50 -cycles 5 
go run ./tests/Timing-Tests -test trigger -period-ms 100 -duty 50 -duration-s 30
go run ./tests/Timing-Tests -test frames  -frames-on 2 -frames-off 2 -cycles 120
go run ./tests/Timing-Tests -test frames  -frames-on 1 -frames-off 60 -cycles 60   # single-frame flashes
go run ./tests/Timing-Tests -test gvsync
go run ./tests/Timing-Tests -test gvsync -cycles 30 -gv-square-px 300      # more repetitions, bigger square
go run ./tests/Timing-Tests -test gvsync -gv-frames-on 3 -gv-frames-off 27 # 50 ms bright, 450 ms dark
go run ./tests/Timing-Tests -test tones   -cycles 300 -freq-hz 1000 -tone-ms 50 -iti-ms 450
go run ./tests/Timing-Tests -test av      -soa-ms 0 -frames-on 3 -frames-off 60 -cycles 30
go run ./tests/Timing-Tests -test rt      -cycles 60 
```

Use `-d N` to select a specific monitor (0-indexed).

Legacy names (`jitter`, `drain`, `square`, `sound`, `audio`) still work as aliases.

---

## Equipment summary

| Test | Display | Photodiode | Oscilloscope | DLP-IO8-G | Keyboard |
|------|:-------:|:----------:|:------------:|:---------:|:--------:|
| `check`   | ✓ | — | — | — | — |
| `display` | ✓ | — | — | — | — |
| `latency` | ✓ | — | — | — | — |
| `stream`  | ✓ | optional | optional | optional | — |
| `vrr`     | ✓ | optional | optional | optional | — |
| `trigger` | ✓ | — | recommended | **required** | — |
| `frames`  | ✓ | **required** | recommended | optional | — |
| `tones`   | ✓ | — | recommended | optional | — |
| `av`      | ✓ | **required** | **required** | optional | — |
| `rt`      | ✓ | optional | optional | optional | **required** |

---

## Flags reference

### Common flags

| Flag | Default | Description |
|------|---------|-------------|
| `-test` | *(required)* | Sub-test name |
| `-w` | false | Windowed mode (1024×768) instead of fullscreen |
| `-d N` | -1 | Monitor index (-1 = primary) |
| `-port` | auto | Serial port for DLP-IO8-G |
| `-trigger-pin` | 1 | DLP-IO8-G output pin (1–8) |
| `-trigger-ms` | 5 | Trigger pulse duration (ms) |
| `-cycles` | 60 | Number of elements / flashes / trials |
| `-hz` | 60.0 | Expected refresh rate (Hz); used by `display`, `stream`, and `av` (not needed for `frames`) |
| `-warmup` | 10 | Cycles/elements excluded from statistics at start |
| `-audio-frames` | SDL default | Hardware audio buffer size in sample frames (e.g. 256, 512, 2048) |
| `-paced-flip` | false | Use `PacedFlip()` instead of `Update()` for frame pacing in `frames` and `av` tests (see note below) |

### Per-test flags

| Flag | Applies to | Default | Description |
|------|-----------|---------|-------------|
| `-level-a` | frames, stream, av | 0 | Dark luminance 0–255 |
| `-level-b` | frames, stream, av | 255 | Bright luminance 0–255 |
| `-frames-on` | frames, stream, av | 1 | Bright frames per cycle; for `av` the tone duration equals `frames-on × refresh period` |
| `-frames-off` | frames, stream, av | 60 | Dark frames per cycle; for `av` this is the dark ITI between stimuli |
| `-duration-s` | display, trigger | 10 | Measurement duration (s) |
| `-period-ms` | trigger | 100 | Square-wave period (ms) |
| `-duty` | trigger | 50 | Duty cycle (%) |
| `-soa-ms` | av | 0 | Visual-to-audio SOA (ms); negative = audio first |
| `-iti-ms` | tones, rt | 1000 | Inter-trial interval / ISI (ms) |
| `-freq-hz` | av, tones, latency | 1000 | Tone frequency (Hz) |
| `-tone-ms` | tones | 50 | Tone duration (ms) |
| `-drain-reps` | latency | 10 | Repetitions per tone duration |
| `-vrr-max-ms` | vrr | 50 | Maximum sweep duration (ms); test runs 1 ms → this value in 1 ms steps |
| `-gv-fps` | gvsync | 60 | Frame rate of the generated `.gv`; **must divide the display refresh rate** |
| `-gv-frames-on` | gvsync | 6 | Bright frames per cycle (6 @ 60 fps = 100 ms) |
| `-gv-frames-off` | gvsync | 24 | Dark frames per cycle (24 @ 60 fps = 400 ms) |
| `-gv-width` | gvsync | 1280 | Stimulus width (px) |
| `-gv-height` | gvsync | 720 | Stimulus height (px) |
| `-gv-square-px` | gvsync | 200 | Side of the centred bright square (px); 0 = fill the frame |
| `-gv-file` | gvsync | — | Play this `.gv` instead of generating one |
| `-gv-keep` | gvsync | false | Regenerate the stimulus and keep it on disk instead of using a temp file |

---

## `gvsync` — .gv playback vs. trigger alignment

Validates `stimuli.PlayGvFunc`: does a `.gv` frame reach the screen when
goxpyriment says it did, and does the TTL trigger line up with it?

The test synthesises its own stimulus — a flash train of `-gv-frames-on` bright
frames (a centred white square) followed by `-gv-frames-off` dark frames,
repeated `-cycles` times — plays it with `PlayGvFunc`, and raises the trigger
line from the frame callback at the onset of every bright phase. With the
defaults that is **100 ms white / 400 ms black, ten times, at 60 fps**.

Nothing needs to be prepared: the stimulus is generated into a temp file and
removed afterwards. `-gv-keep` writes it next to you instead, and `-gv-file`
plays one you supply.

### Setup

```
photodiode ──▶ scope channel 1     (taped over the white square)
DLP-IO8 pin ─▶ scope channel 2     (-trigger-pin, default 1)
```

```bash
go run ./tests/Timing-Tests -test gvsync           # fullscreen — do not add -w
```

### What to read off the trace

| Measurement | Expected | Meaning |
|---|---|---|
| TTL edge → luminance step | constant across cycles | end-to-end presentation lag; its *variance* matters more than its value |
| Bright phase width | 100 ms (`frames-on / fps`) | frame hold is correct |
| Onset-to-onset period | 500 ms (`(on+off) / fps`) | no accumulating drift |
| Any period off by ≥ 1 frame | never | a dropped frame |

The console reports the onset period measured in software and a
`playback: N frames, M skipped` line. **`skipped` > 0 means frames were dropped
and the recording will show it** — fix that before interpreting anything else.

The reported "flip → trigger dispatch" figure is software-side only: the
interval between the flip timestamp and the moment the trigger call was
dispatched. It is a lower bound and excludes the trigger device's own latency,
which is exactly what the scope is there to measure.

### Frame rate

`-gv-fps` must divide the display refresh rate exactly, because `PlayGv` holds
each video frame for `refresh / fps` refreshes. At 60 Hz that allows 60, 30, 20,
15, 12, 10 … but not 24 or 25 — those are refused with an error naming a
workable rate rather than played at the wrong speed.

---

## Output files

Each run writes a `.csv` file to `~/goxpy_data/`.

```python
import pandas as pd
df = pd.read_csv("~/goxpy_data/Timing-Tests_000_*.csv")
```

---

## Frame-pacing mode (`-paced-flip`)

The `frames` and `av` tests support two frame-pacing strategies, selectable
with the `-paced-flip` flag:

| Mode | Call | When to use |
|------|------|-------------|
| default | `Screen.Update()` | Standard `SDL_RenderPresent` with VSYNC block — works well on most systems |
| `-paced-flip` | `Screen.PacedFlip()` | Alternative pacing loop — better on some hardware (e.g. systems where `Update()` shows high jitter) |

If you observe unexpectedly high timing variance in `frames` or `av`, try
re-running with `-paced-flip` and compare the statistics. Report which mode
you used when sharing results.

---

## Hardware notes

**Photodiode** — tape it to the screen corner where the bright stimulus appears
and connect its output to oscilloscope channel 1.

**DLP-IO8-G** — connects via USB (appears as `/dev/ttyUSBx` on Linux).
The user must be in the `dialout` group: `sudo usermod -aG dialout $USER`.
To reduce USB latency to ~1 ms (recommended):
```bash
echo 1 | sudo tee /sys/bus/usb-serial/devices/ttyUSB0/latency_timer
```

**Audio line-out** — for the `tones` and `av` tests, patch the headphone or
line-out jack into oscilloscope channel 2 to measure actual acoustic onset timing.

For **`tones`**: the DLP-IO8-G pin goes HIGH just before `PlayStreamOfSounds`
and LOW immediately after it returns, producing a single square pulse covering
the entire stream. Compare its width against the nominal `cycles × SOA` value
to verify that the audio pipeline does not drift.

For **`av`**: the trigger fires at each visual onset (as in `frames`); compare
the audio waveform onset against the trigger edge to measure the AV delay.
