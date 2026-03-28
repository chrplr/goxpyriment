# psychopy-test

PsychoPy counterpart to the goxpyriment `Timing-Tests` binary.
Implements the same sub-tests and prints statistics in the **same format** so results
can be pasted side-by-side for a direct framework comparison.

---

## Installation

### 1 — Create a virtual environment

```bash
cd tests/Timing-Tests/psychopy-test

python3 -m venv .venv
source .venv/bin/activate          # Windows: .venv\Scripts\activate
```

### 2 — Install wxPython precompiled wheel (Linux only)

PsychoPy depends on wxPython, which rarely compiles cleanly from source. 

**Install a precompiled wheel for Linux before running `pip install psychopy`.**

**Linux** — wheels are hosted per-distro at
`https://extras.wxpython.org/wxPython4/extras/linux/gtk3/`.
Replace `ubuntu-22.04` with your actual distro slug (see the index for the full list):

```bash
# Find the right slug — browse https://extras.wxpython.org/wxPython4/extras/linux/gtk3/
# Common examples: ubuntu-20.04, ubuntu-22.04, ubuntu-24.04,
#                  fedora-38, debian-12, opensuse-15.5

pip install -f https://extras.wxpython.org/wxPython4/extras/linux/gtk3/ubuntu-22.04/ wxPython
```

### 3 — Install dependencies

```bash
pip install -r requirements.txt
```

PsychoPy pulls in `numpy`, `sounddevice`, and several other packages automatically.
`pyserial` is needed only if you have a DLP-IO8-G trigger device.

### 4 — Optional: psychtoolbox (recommended for the `rt` test)

```bash
pip install psychtoolbox
```

With psychtoolbox installed, PsychoPy timestamps key events at hardware-interrupt
time — matching goxpyriment's SDL3 nanosecond clock.  Without it, key timestamps
reflect Python poll-loop time (~1–5 ms jitter), which inflates the RT standard
deviation.

---

## Quick start

```bash
# Windowed 10 s jitter test (no hardware needed):
python timing_tests.py --test jitter -d

# Fullscreen 30 s jitter test:
python timing_tests.py --test jitter --duration-s 30

# Keyboard RT test, 60 trials, windowed:
python timing_tests.py --test rt --cycles 60 -d

# Audio drain test (measures audio pipeline latency):
python timing_tests.py --test drain -d

# Tone stream onset-jitter, 300 tones:
python timing_tests.py --test sound --cycles 300 --tone-ms 50 --iti-ms 450 -d
```

Press **ESC** at any time to stop a test early and print the statistics collected so far.

---

## Equivalent goxpyriment commands

| PsychoPy | goxpyriment |
|---|---|
| `python timing_tests.py --test jitter -d` | `go run main.go -test jitter -d` |
| `python timing_tests.py --test frames --cycles 120` | `go run main.go -test frames -cycles 120` |
| `python timing_tests.py --test flash --isi-frames 60` | `go run main.go -test flash -isi-frames 60` |
| `python timing_tests.py --test av --soa-ms 0` | `go run main.go -test av -soa-ms 0` |
| `python timing_tests.py --test square --period-ms 100` | `go run main.go -test square -period-ms 100` |
| `python timing_tests.py --test sound --cycles 300` | `go run main.go -test sound -cycles 300` |
| `python timing_tests.py --test rt --cycles 60` | `go run main.go -test rt -cycles 60` |
| `python timing_tests.py --test drain` | `go run main.go -test drain` |

---

## Sub-tests

### `jitter` — pure frame-interval statistics

Flips a mid-gray screen continuously for `--duration-s` seconds and records the
wall-clock interval between consecutive flip returns.  No hardware needed.

`win.flip()` blocks until the next VSYNC boundary (`waitBlanking=True`), mirroring
SDL VSync in goxpyriment.  The Python GC is disabled during the loop (`gc.disable()`),
mirroring Go's `debug.SetGCPercent(-1)`.

```
python timing_tests.py --test jitter --duration-s 30 -d
```

Sample output:

```
Estimated refresh rate: 59.943 Hz  (use --hz 59.94 for frame targets)

── Frame intervals ───────────────────────────────
  n       : 1786
  target  : 16.682 ms
  mean    : 16.682 ms
  SD      : 0.143 ms
  min/max : 16.412 / 19.801 ms
  p5/p95  : 16.582 / 16.782 ms
  >0.5 ms : 3 (0.2 %)
  >1.0 ms : 1 (0.1 %)
```

---

### `frames` — alternating luminance

Alternates between dark (`--level-a`, default 0) and bright (`--level-b`, default 255)
for `--frames-per-phase` frames each, for `--cycles` complete cycles.  A trigger pulse
is sent at the first frame of each bright phase (requires DLP-IO8-G; skipped otherwise).

```
python timing_tests.py --test frames --frames-per-phase 2 --cycles 120 -d
```

---

### `flash` — single-frame flashes

Presents one bright frame every `--isi-frames` dark frames, for `--cycles` flashes.
Records flash-to-flash intervals.

```
python timing_tests.py --test flash --isi-frames 60 --cycles 60 -d
```

---

### `av` — audio–visual synchrony

Presents `--cycles` trials of a white-screen flash and a pure sine tone at a
configurable SOA (`--soa-ms`).  Records the time of the flip and the time `tone.play()`
was called.

`t_audio_queued_ms` is when PCM data was pushed to the driver buffer, **not** when
sound reaches the speaker.  The acoustic onset occurs `pipeline_latency` ms later
(use the `drain` test to measure this).  For absolute AV delay, use an oscilloscope:
audio line-out → channel 1, photodiode on screen → channel 2.

```
python timing_tests.py --test av --soa-ms 0 --freq-hz 1000 --tone-ms 50 --iti-ms 1000 --cycles 30 -d
```

---

### `square` — DLP-IO8-G square wave

Drives a square wave on `--trigger-pin` for `--duration-s` seconds.  **Requires a
DLP-IO8-G USB trigger device** (`pip install pyserial`).

Uses a busy-spin approach (sleep until 500 µs before target, then spin) to minimise
edge jitter, matching Go's `sleepUntil()`.

```
python timing_tests.py --test square --period-ms 100 --duty 50 --duration-s 30 --trigger-pin 1
```

---

### `sound` — tone stream onset jitter

Plays `--cycles` tones (frequency `--freq-hz`, duration `--tone-ms`) separated by
`--iti-ms` ms of silence.  Reports onset error (actual − target onset time) and
inter-onset interval statistics.

```
python timing_tests.py --test sound --cycles 300 --tone-ms 50 --iti-ms 450 -d
# Quick check (30 tones):
python timing_tests.py --test sound --cycles 30 --iti-ms 450 -d
```

---

### `rt` — keyboard reaction time

Presents `--cycles` single-frame white flashes; the participant presses any key as
fast as possible.  RT = key event timestamp − flip timestamp (both on `defaultClock`).

With **psychtoolbox**, `key.tDown` is a hardware-interrupt timestamp, directly
comparable to goxpyriment's SDL3 `KeyboardEvent.Timestamp`.  Without it, timestamps
are at Python poll-loop time.

```
python timing_tests.py --test rt --cycles 60 --iti-ms 1000 -d
```

---

### `drain` — audio pipeline latency

Measures how long the audio driver takes to consume pre-generated PCM data after
`sounddevice.play()` is called.  Tests five fixed durations: 25, 50, 100, 200, 500 ms.

`sd.wait()` returns when the software buffer is empty (last sample dispatched to the
DAC), mirroring Go's spin-poll on `stream.Queued() == 0`.  The pipeline latency is
`mean(drain_ms) − nominal_ms`.

```
python timing_tests.py --test drain -d
```

---

## Flags reference

### Common flags

| Flag | Default | Description |
|------|---------|-------------|
| `--test` | *(required)* | Sub-test to run |
| `-d` | false | Windowed 1024×768 developer mode |
| `--screen` | 0 | Screen index for fullscreen |
| `--port` | auto | Serial port for DLP-IO8-G |
| `--trigger-pin` | 1 | Output pin on DLP-IO8-G |
| `--trigger-ms` | 5 | Trigger pulse duration (ms) |
| `--cycles` | 60 | Cycles / flashes / trials |
| `--hz` | 60.0 | Expected display refresh rate (Hz) |
| `--warmup` | 10 | Frames discarded at start |

### Per-test flags

| Flag | Applies to | Default | Description |
|------|-----------|---------|-------------|
| `--level-a` | frames, flash, av | 0 | Dark luminance 0–255 |
| `--level-b` | frames, flash, av | 255 | Bright luminance 0–255 |
| `--frames-per-phase` | frames | 2 | Frames per dark/bright phase |
| `--isi-frames` | flash | 60 | Dark frames between flashes |
| `--soa-ms` | av | 0 | Visual-to-audio SOA (ms); negative = audio first |
| `--iti-ms` | av, sound, rt | 1000 | Inter-trial/stimulus interval (ms) |
| `--freq-hz` | av, sound, drain | 1000 | Tone frequency (Hz) |
| `--tone-ms` | av, sound | 50 | Tone duration (ms) |
| `--duration-s` | jitter, square | 10 | Measurement duration (seconds) |
| `--period-ms` | square | 100 | Square-wave period (ms) |
| `--duty` | square | 50 | Duty cycle (%) |
| `--drain-reps` | drain | 10 | Repetitions per tone duration |

---

## Timing methodology

### Frame-interval measurement (`jitter`, `frames`, `flash`)

The interval is `t_after_flip[i] − t_after_flip[i−1]`, where `t_after_flip` is the
return value of `win.flip()` — captured immediately after `SwapBuffers` returns (i.e.,
after the VSYNC wait).  This mirrors `fillGray()`'s `tAfter` timestamp in the Go binary.

### Key-event timestamps (`rt`)

| Backend | Timestamp source | Precision |
|---------|-----------------|-----------|
| psychtoolbox | Hardware interrupt (same as SDL3 `KeyboardEvent.Timestamp`) | sub-ms |
| Default (no ptb) | Python event-loop poll time | ~1–5 ms |

### Audio onset timestamps (`av`, `sound`)

`tone.play()` / `core.getTime()` is called immediately after the call to queue PCM
data to the OS driver.  The acoustic onset occurs `pipeline_latency` ms later.  This
matches the Go version, where `tone.Play()` queues data and returns immediately.

### DLP-IO8-G command protocol

The same ASCII commands as goxpyriment's `triggers/dlpio8.go`:

| Command | Byte | Effect |
|---------|------|--------|
| Set HIGH pin 1–8 | `'1'`–`'8'` | Drive pin high |
| Set LOW  pin 1–8 | `'Q'`,`'W'`,`'E'`,`'R'`,`'T'`,`'Y'`,`'U'`,`'I'` | Drive pin low |
| Ping | `"'"` | Device responds `'Q'` |

To reduce USB-serial latency to ~1 ms on Linux:

```bash
echo 1 | sudo tee /sys/bus/usb-serial/devices/ttyUSB0/latency_timer
```
