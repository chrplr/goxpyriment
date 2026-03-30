# Timing-Tests

A hardware timing verification suite for goxpyriment experiments.
Use it to characterise your display's frame timing, verify trigger precision,
and measure the actual audio–visual delay of your system before running
perceptual experiments.

## Equipment

| Test | Display | Photodiode | Oscilloscope | DLP-IO8-G | Keyboard / response box |
|------|---------|-----------|-------------|-----------|------------------------|
| `frames`  | ✓ | ✓ | optional | optional | — |
| `flash`   | ✓ | ✓ | optional | optional | — |
| `av`      | ✓ | ✓ | ✓ | optional | — |
| `jitter`  | ✓ | — | — | — | — |
| `square`  | — | — | ✓ | **required** | — |
| `sound`   | — | — | ✓ (optional) | optional | — |
| `rt`      | ✓ | optional | optional | optional | **required** |
| `drain`   | — | — | — | — | — |

**Photodiode** — tape it to the corner of your monitor where the bright stimulus
appears. Connect its output to oscilloscope channel 1.

**DLP-IO8-G** — connects via USB (appears as `/dev/ttyUSBx` on Linux).
Connect pin 1 (or your chosen trigger pin) to oscilloscope channel 2.
The device is auto-detected if `-port` is omitted.
The user must be in the `dialout` group: `sudo usermod -aG dialout $USER`.

**Audio** (for `av` test) — patch the headphone or line-out jack directly into
oscilloscope channel 3 so you can measure the actual acoustic onset.

---

## Quick start

```bash
# From the repo root (go.work takes care of module resolution):
go run tests/Timing-Tests/main.go -test jitter -w
go run tests/Timing-Tests/main.go -test frames -w -cycles 120
go run tests/Timing-Tests/main.go -test square -period-ms 100 -duty 50 -duration-s 30
```

Add `-w` for a windowed 1024×768 window.
Remove `-w` for fullscreen on the primary display.

---

## Sub-tests

### `frames` — alternating luminance (core photodiode test)

Alternates between a dark level (`-level-a`, default 0) and a bright level
(`-level-b`, default 255) for `-frames-per-phase` frames each, repeating for
`-cycles` complete dark/bright cycles.

A trigger pulse is sent on pin `-trigger-pin` at the start of every bright
phase. The pulse lasts `-trigger-ms` milliseconds.

**On the oscilloscope:**
- Channel 1 (photodiode): should show a square wave matching the luminance period.
- Channel 2 (trigger): should align with the rising edge of the photodiode signal.
- Verify period = `frames-per-phase × frame_duration` (e.g. 2 × 16.67 ms = 33.3 ms at 60 Hz).
- Jitter between trigger and photodiode onset reveals the USB-serial latency (~1–2 ms).

**Output file columns:**
`cycle, phase (0=dark/1=bright), frame, t_before_ms, t_after_ms, interval_ms, trigger`

```bash
go run main.go -test frames -frames-per-phase 2 -cycles 120 -trigger-pin 1 -trigger-ms 5
```

---

### `flash` — single-frame flashes (minimum-duration test)

Presents a single bright frame every `-isi-frames` dark frames, for `-cycles`
flashes. Use this to verify that your OS/driver combination can actually present
a stimulus that is only one frame long (some setups skip frames or double-buffer
in ways that prevent single-frame precision).

**On the oscilloscope:**
- The photodiode pulse width should equal exactly one frame duration (~16.67 ms
  at 60 Hz). If you see double-width pulses, the graphics driver is not
  single-frame capable.

**Output file columns:**
`flash_num, t_before_ms, t_after_ms, interval_ms`

```bash
go run main.go -test flash -isi-frames 60 -cycles 60 -trigger-pin 1
```

---

### `av` — audio–visual synchrony

Presents `-cycles` trials, each consisting of a bright-screen flash and a pure
sine tone (frequency `-freq-hz`, duration `-tone-ms`). The relative onset of
the two stimuli is controlled by `-soa-ms`:

- `soa-ms = 0`: visual flip and audio queue happen back-to-back (minimum SOA).
- `soa-ms > 0`: screen comes first, then the tone is queued after `soa-ms` ms.
- `soa-ms < 0`: tone is queued first, then the screen flips after `|soa-ms|` ms.

> **Important:** `t_audio_queued_ms` is when the PCM data was pushed to the
> SDL audio buffer, *not* when sound comes out of the speaker. The actual
> acoustic onset depends on the system audio output latency (typically 5–30 ms
> at 44100 Hz, 512-sample buffer). Measure it with the oscilloscope:
> `actual_AV_delay = t_audio_channel - t_photodiode_channel`.

Trials are separated by a dark-screen inter-trial interval of `-iti-ms` ms.

**Output file columns:**
`trial, t_visual_before_ms, t_visual_after_ms, t_audio_queued_ms, soa_intended_ms, soa_actual_ms`

```bash
go run main.go -test av -soa-ms 0 -freq-hz 1000 -tone-ms 50 -iti-ms 1000 -cycles 30
```

---

### `jitter` — pure frame-interval statistics

Flips a gray screen continuously for `-duration-s` seconds (default 10) and
records the wall-clock interval between consecutive `RenderPresent` returns.
No trigger device required.

Prints a summary at the end:

```
Estimated refresh rate: 59.94 Hz
── Frame intervals ──────────────────────────────
  n       : 599
  target  : 16.670 ms
  mean    : 16.683 ms
  SD      : 0.112 ms
  min/max : 16.401 / 18.203 ms
  p5/p95  : 16.601 / 16.801 ms
  >0.5 ms : 2 (0.3 %)
  >1.0 ms : 0 (0.0 %)
```

High SD or many frames `>0.5 ms` indicate CPU scheduling interference.
Consider `chrt -r 99` or isolating a CPU core for the experiment process.

```bash
go run main.go -test jitter -duration-s 30 -w
```

---

### `square` — DLP-IO8-G square wave (trigger stability test)

Drives a square wave on pin `-trigger-pin` for `-duration-s` seconds, with
period `-period-ms` and duty cycle `-duty` %. No display stimulus is involved.

Use this to characterise the timing precision of the DLP-IO8-G in isolation,
before relying on it for experiment triggers. A `period-ms` of 100 ms (10 Hz)
and `duty` of 50 % is a good starting point.

**On the oscilloscope:**
- Measure the actual period and duty cycle.
- Observe jitter on the rising and falling edges.
- Typical result: ~1–2 ms jitter due to USB-CDC latency; the average period
  should match the requested value within < 0.1 ms.

Prints a jitter summary at the end:

```
── Rising-edge jitter (ms from target) ────────
  mean    :  0.831 ms
  SD      :  0.294 ms
  min/max :  0.201 / 1.842 ms
```

**This test requires a DLP-IO8-G.** It will fail immediately if no device is
found.

```bash
go run main.go -test square -period-ms 100 -duty 50 -duration-s 30 -trigger-pin 1
```

---

### `sound` — long regular tone stream (audio onset-jitter test)

Plays a sequence of `-cycles` identical sine tones (frequency `-freq-hz`,
duration `-tone-ms`) separated by a silence of `-iti-ms` ms, using
`stimuli.MakeRegularSoundStream` + `stimuli.PlayStreamOfSounds`. GC is
disabled for the duration of the stream (handled inside `PlayStreamOfSounds`).

The default settings (300 tones × 50 ms on / 450 ms ISI) produce a ~2.5 minute
stream, long enough to reveal cumulative drift and OS scheduling outliers.

The **onset error** is `actual_onset − target_onset` for each tone, where
`target_onset = index × SOA`. It grows if the audio clock drifts relative to
the wall clock. The **inter-onset interval (IOI)** is the gap between
consecutive actual onsets; it should equal `SOA = tone_ms + iti_ms` with low
variance.

> `actual_onset_ms` is when `Play()` was called (i.e. when PCM data was queued
> to the SDL audio stream), not the acoustic onset. The acoustic onset occurs
> `audio_latency` ms later (system-dependent, typically 5–30 ms at 44100 Hz).
> The latency is constant, so **jitter** is still meaningful even though the
> absolute times are offset.

If a DLP-IO8-G is connected, a trigger pulse is sent on `-trigger-pin` just
before each `Play()` call. Connect pin 1 to oscilloscope channel 2 and the
audio line-out to channel 1: the gap between the trigger edge and the acoustic
onset is the **software-to-DAC latency** for that tone. Plotting it across all
300 tones reveals both the mean latency and any trial-to-trial jitter.

Prints two statistics tables at the end:

```
── Onset error vs target (ms) ────────────────
  n       : 300
  target  :   0.000 ms
  mean    :   1.243 ms   ← cumulative drift ~4 ms/min
  SD      :   0.381 ms
  min/max :   0.002 / 3.847 ms
  >0.5 ms :  47 (15.7 %)
  >1.0 ms :  12 ( 4.0 %)

── Inter-onset interval (ms) ─────────────────
  n       : 299
  target  : 500.000 ms
  mean    : 500.004 ms
  SD      :   0.389 ms
  min/max : 499.121 / 502.847 ms
  >0.5 ms :   9 ( 3.0 %)
  >1.0 ms :   1 ( 0.3 %)
```

**Output file columns:**
`tone_num, target_onset_ms, actual_onset_ms, onset_error_ms, actual_offset_ms, ioi_ms, ioi_error_ms`

```bash
go run main.go -test sound -cycles 300 -freq-hz 1000 -tone-ms 50 -iti-ms 450
# quick check (30 tones, ~15 s):
go run main.go -test sound -cycles 30 -iti-ms 450 -w
```

---

### `rt` — SDL event-timestamp RT precision test

Measures keyboard reaction time using SDL3 hardware event timestamps for
`-cycles` trials (default 60). Each trial:

1. The screen goes blank for a jittered ITI (`-iti-ms` ± 50 %, default 1000 ms).
2. The screen flashes white for one frame; `Screen.FlipNS()` records the SDL
   nanosecond tick immediately after `SDL_RenderPresent` returns.
3. The participant presses any key; `WaitKeysEventRT` returns the SDL3
   `KeyboardEvent.Timestamp` — the nanosecond time of the hardware interrupt,
   not a polling-time delta.
4. `RT = eventTimestamp − onsetNS` (nanoseconds; stored and printed in ms).

Because both timestamps come from the same `SDL_GetTicksNS()` clock, this RT
is free of polling latency on the response side. The remaining jitter reflects
OS keyboard scheduling (typically < 2 ms on Linux with standard kernel).

**For validation with a hardware response box:**
Connect a USB response box that presents as a keyboard. Its internal timestamping
and the SDL event timestamp should agree within 1–2 ms; the gap between them
characterizes the OS keyboard-event pipeline delay.

**Optional trigger output:**
If a DLP-IO8-G is connected, a trigger pulse is sent on `-trigger-pin` just
before each flash. Combine with a photodiode on channel 1 and the trigger on
channel 2 to measure the actual display onset latency (trigger → light pulse
rising edge), which can then be subtracted from the RT to obtain the true
stimulus-onset to button-press interval.

**Output file columns:**
`trial, onset_ns, event_ts_ns, rt_ns, rt_ms`

Prints statistics at the end:

```
── RT (ms, event-timestamp method) ─────────────
  n       : 60
  mean    : 287.4 ms
  SD      :  42.1 ms
  min/max : 198.3 / 401.2 ms
  p5/p95  : 221.7 / 358.9 ms
```

```bash
go run main.go -test rt -cycles 60 -iti-ms 1000 -w
# With trigger output for photodiode validation:
go run main.go -test rt -cycles 60 -trigger-pin 1 -trigger-ms 5
```

---

### `drain` — audio pipeline latency (self-contained)

Measures how long it takes the SDL audio pipeline to consume a tone of a given
nominal duration. No external equipment is needed.

For each tone duration in the fixed set **25, 50, 100, 200, 500 ms**, the test
repeats `-drain-reps` trials (default 10). Each trial:

1. Calls `tone.Play()`, which puts the pre-generated PCM data into the SDL audio
   stream bound to the device.
2. Polls `stream.Queued()` in a 0.5 ms sleep loop until the device has consumed
   all queued bytes (`Queued() == 0`).
3. Records `drain_ms` — the wall-clock elapsed time from `Play()` to drain
   completion.

The **audio pipeline latency** is `mean(drain_ms) − nominal_ms`.  It represents
the combined delay of the SDL audio conversion pipeline and the hardware DAC
buffer.  The **SD** of `drain_ms` captures trial-to-trial jitter in the audio
scheduler, which is the quantity that matters for precise auditory stimulus
timing.

> **Note:** `drain_ms` measures when the last byte leaves the SDL software
> queue, *not* when the last sample reaches the speaker. The DAC/amplifier adds
> a further constant (typically sub-millisecond) delay that is not captured
> here. For absolute acoustic-onset latency, use the `av` test with an
> oscilloscope.

A typical run on a 44100 Hz, 512-sample device prints:

```
  25 ms  rep  0:  drain=37.1 ms  overhead=+12.1 ms
  25 ms  rep  1:  drain=37.3 ms  overhead=+12.3 ms
  …
── Drain time for 25 ms tone (latency = mean − target) ────────
  n       : 10
  target  : 25.000 ms
  mean    : 37.2 ms
  SD      :  0.3 ms
  …
  pipeline latency ≈ 12.2 ms
```

A latency of ~11.6 ms at 512 samples / 44100 Hz is the theoretical minimum
(`512 / 44100 × 1000`); larger values indicate OS mixing overhead
(PulseAudio / PipeWire).

**Output file columns:**
`duration_ms, rep, drain_ms, overhead_ms`

```bash
go run main.go -test drain -w
go run main.go -test drain -wrain-reps 20 -freq-hz 1000 -audio-frames 256
```

---

## Flags reference

### Common flags

| Flag | Default | Description |
|------|---------|-------------|
| `-test` | *(required)* | Sub-test to run |
| `-w` | false | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |
| `-port` | auto | Serial port for DLP-IO8-G |
| `-trigger-pin` | 1 | Output pin on DLP-IO8-G |
| `-trigger-ms` | 5 | Trigger pulse duration (ms) |
| `-cycles` | 60 | Number of cycles / flashes / trials |
| `-hz` | 60.0 | Expected display refresh rate (Hz). Run `jitter` first, then pass the measured value here so frame-interval targets are exact. |
| `-warmup` | 10 | Frames discarded from statistics at the start of visual tests to avoid GPU pipeline warm-up bias. |
| `-audio-frames` | SDL default | Hardware audio buffer size in sample frames (e.g. 256, 512, 2048). |

### Per-test flags

| Flag | Applies to | Default | Description |
|------|-----------|---------|-------------|
| `-level-a` | frames, flash | 0 | Dark luminance 0–255 |
| `-level-b` | frames, flash | 255 | Bright luminance 0–255 |
| `-frames-per-phase` | frames | 2 | Frames at each luminance level |
| `-isi-frames` | flash | 60 | Dark frames between flashes |
| `-soa-ms` | av | 0 | Visual-to-audio SOA (ms); negative = audio first |
| `-iti-ms` | av, sound, rt | 1000 | Inter-trial/inter-stimulus interval (ms) |
| `-freq-hz` | av, sound, drain | 1000 | Tone frequency (Hz) |
| `-tone-ms` | av, sound | 50 | Tone duration (ms) |
| `-duration-s` | jitter, square | 10 | Measurement duration (seconds) |
| `-period-ms` | square | 100 | Square-wave period (ms) |
| `-duty` | square | 50 | Duty cycle (%) |
| `-drain-reps` | drain | 10 | Repetitions per tone duration |

## Audio buffer size

The hardware audio buffer size (latency) is controlled by the SDL hint
`SDL_AUDIO_DEVICE_SAMPLE_FRAMES`, exposed via the `-audio-frames` flag.
It must be set **before** the audio device opens (i.e. before any other flags
take effect). Use the `sound` test to find the minimum stable value for your
system:

```bash
# Default (platform-dependent, often 512–2048 samples at 44100 Hz):
go run main.go -test sound -cycles 30 -w

# Aggressive low-latency (~5.8 ms at 44100 Hz):
go run main.go -test sound -cycles 60 -audio-frames 256 -w

# Conservative (~46 ms, stable on any system):
go run main.go -test sound -cycles 60 -audio-frames 2048 -w
```

On startup the program prints the **actual** device format after opening,
e.g.:

```
audio: requesting 256 sample frames hardware buffer
audio: 44100 Hz  1 ch  256 sample frames (~5.8 ms latency)
```

The latency printed is the **minimum** delay between `tone.Play()` and the
first sample reaching the DAC. The DLP-IO8-G trigger fires just before
`Play()`, so the oscilloscope gap (trigger → acoustic onset) ≈ this latency
plus USB-serial jitter (~1–2 ms). Reducing `-audio-frames` brings the two
closer together.

On Linux with PulseAudio or PipeWire the server adds its own mixing buffer on
top; use ALSA directly (`SDL_AUDIODRIVER=alsa`) or JACK for the lowest
possible latency.

## DLP-IO8-G notes

The DLP-IO8-G communicates over USB-CDC (virtual serial port, 115200 baud). Each
`SetHigh` / `SetLow` command is a single byte written to the serial port. On
Linux, USB-serial round-trip latency is typically **1–2 ms** with the default
`latency_timer` of 16 ms.

To reduce USB latency to ~1 ms (recommended for triggers):

```bash
echo 1 | sudo tee /sys/bus/usb-serial/devices/ttyUSB0/latency_timer
```

Replace `ttyUSB0` with your actual port name. The change reverts on replug; add
a udev rule to make it permanent.

## DLP-IO8 vs DLP-IO8-G

Both models use the same ASCII command protocol over USB-CDC and are
interchangeable in software. The **-G** (galvanic isolated) variant adds
optocoupler isolation between the USB side and the I/O pins, protecting the
computer from ground loops and voltage transients on the signal lines — strongly
recommended for use with EEG amplifiers and oscilloscopes.

## Parallel port alternative

If you have a legacy parallel port (LPT), use `triggers.NewParallelPort("/dev/parport0")`
in your own experiment code. The `Send(byte)` method sets all 8 data lines
simultaneously, which is the standard way to send EEG trigger codes. Latency is
sub-millisecond (< 10 µs with direct ioctl), far better than USB-serial.

Prerequisites: `sudo modprobe ppdev` and membership in the `lp` group.

## Output files

Each run writes an `.csv` file to `~/goxpy_data/` (standard goxpyriment output).
The file is a CSV with a metadata header. Load it in Python with:

```python
import pandas as pd
df = pd.read_csv("~/goxpy_data/Timing-Tests_000_*.csv", comment="#")
```
