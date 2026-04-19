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
8. tones    — measure audio onset jitter (long stream)
9. av       — measure audio–visual synchrony
10. rt      — measure reaction-time timestamp precision
```

Steps 1–5 require no external hardware (step 5 benefits from a VRR monitor).
Steps 6–9 require a DLP-IO8-G and/or oscilloscope + photodiode (see docs).
Step 10 requires a keyboard or USB response box.

---

## Running

```bash
# from the repo root (go.work resolves both modules):
go run tests/Timing-Tests/main.go -test <name> [flags]

# examples
go run tests/Timing-Tests/main.go -test check  
go run tests/Timing-Tests/main.go -test display -duration-s 30 
go run tests/Timing-Tests/main.go -test latency 
go run tests/Timing-Tests/main.go -test stream  -cycles 120 -frames-on 3 -frames-off 3 
go run tests/Timing-Tests/main.go -test vrr     -vrr-max-ms 50 -cycles 5 
go run tests/Timing-Tests/main.go -test trigger -period-ms 100 -duty 50 -duration-s 30
go run tests/Timing-Tests/main.go -test frames  -frames-on 2 -frames-off 2 -cycles 120
go run tests/Timing-Tests/main.go -test frames  -frames-on 1 -frames-off 60 -cycles 60   # single-frame flashes
go run tests/Timing-Tests/main.go -test tones   -cycles 300 -freq-hz 1000 -tone-ms 50 -iti-ms 450
go run tests/Timing-Tests/main.go -test av      -soa-ms 0 -frames-on 3 -frames-off 60 -cycles 30
go run tests/Timing-Tests/main.go -test rt      -cycles 60 
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

---

## Output files

Each run writes a `.csv` file to `~/goxpy_data/`.

```python
import pandas as pd
df = pd.read_csv("~/goxpy_data/Timing-Tests_000_*.csv")
```

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
