# test_audio_drift — audio clock rate error, in ppm

Measures how fast the audio hardware actually runs compared to its nominal
sample rate, and whether that rate is steady or wandering.

The motivating observation: a long WAV file does not last as long as its frame
count says it should, and the mismatch is sometimes positive, sometimes
negative. This test decides which of the candidate causes is operating on a
given machine.

## What it does

A tone (default 10 minutes) is synthesised in memory and played through the
normal goxpyriment audio path. While it plays, the test pairs

| quantity | source |
|---|---|
| `t_audio` | frames the device has consumed ÷ nominal sample rate |
| `t_system` | `clock.GetTimeNS()` — Go's monotonic clock |

every 500 ms, and fits `t_audio = a + b·t_system` by least squares. The result
is reported as

```
ppm = (b - 1) × 1e6
```

Positive = the audio clock runs fast, so the file finishes early.

## Why a regression and not a stopwatch

Timing the file end to end is the obvious approach and it is wrong.

The frame count comes from `SDL_GetAudioStreamQueued`, which sees **only SDL's
software queue**. Frames that have left it may still be sitting in SDL's
internal buffers and the hardware DMA buffer, unheard. Measuring when the queue
empties therefore reports the file as *short* — "playing faster than
theoretical" — by however much is buffered downstream. On this machine's
PipeWire setup that intercept measured **+236 ms**, an order of magnitude more
than the 23 ms callback period one might assume.

That lag is constant, so in a regression it lands entirely in the **intercept**
and leaves the **slope** untouched. The slope is the physical quantity; the
intercept is an artefact of where the measurement is taken. `Sound.Wait()` has
the same limitation by construction (see `stimuli/audio.go`), which is worth
remembering before quoting any duration it produces.

## Reading the output

```
rate error       : +4.62 ppm   (95% CI +1.60 … +7.64)
accumulated drift: +2.7 ms over the fitted 594.0 s
residuals (ms)   : SD 9.1, median +0.1, IQR -6.5 … +6.5, range -21.4 … +21.8
  skew +0.01, kurtosis 2.39 (uniform 1.8, normal 3.0), lag-1 autocorr +0.039
```

- **rate error** — the headline figure. Audio codec crystals are commonly spec'd
  ±50–100 ppm, but see the measurement below: the one machine tested came in an
  order of magnitude under that.
- **the residual block is the part that says whether to believe the slope.** A
  sound fit is symmetric about zero, has kurtosis between uniform (1.8) and
  normal (3.0), and is bounded by roughly one callback period. A long tail means
  some samples were produced by a different process than the rest — and on a
  short run a handful of those will set the answer. The tool warns when the
  largest residual exceeds three callback periods.
- **intercept** — buffering ahead of the DAC. Not a drift measurement.

### The tail is contaminated, and it is not obvious from the summary

When the software queue empties, `frames_consumed` clamps at the file length:
the final sample reports 42 ms of audio elapsed over a 200 ms interval. For the
last stretch before that — roughly as long as the audio buffered downstream of
SDL's queue — the downstream buffer is draining rather than staying full, so
"frames that left the software queue" stops tracking "frames played" at the
constant offset the fit assumes.

Measured on this rig, on the same 42 s recording:

| samples included | ppm | residual SD | largest residual |
|---|---:|---:|---:|
| all | −356 | 17.3 ms | 187.7 ms |
| drop the clamped final sample | −213 | 10.8 ms | 41.3 ms |
| drop the last 5 s | +44 | 9.0 ms | 20.5 ms |

`-cooldown-s` (default 5) and the automatic exclusion of drained samples handle
this. It matters enormously for short runs and hardly at all for long ones — the
same correction moved a 10-minute run from +4.62 to +4.66 ppm, because one bad
point has no leverage across a 594 s span. **The summary statistics hid it
completely**: that 10-minute fit reported a healthy 9.12 ms residual SD either
way. Only the extremes of the residual distribution showed the defect.

### The per-segment table is the discriminating part

```
  window (s)              n        ppm      +/-SE
```

Two different mechanisms produce a duration mismatch and they look identical in
the overall slope:

- a **fixed crystal offset** gives the same ppm in every segment;
- an **adaptive resampler** tracking a second clock (loopback, combined or
  network sink, Bluetooth, an aggregate device) makes ppm wander between
  segments while the long-run mean stays near the true ratio.

The test compares the observed scatter of the segment estimates against the
scatter their own standard errors predict, and only calls the rate "wandering"
when it exceeds that. A short run scatters wildly for purely statistical
reasons — the tool says so rather than over-reading it.

## Running

```bash
# from the repo root; -w because the test draws nothing and a 10-minute
# fullscreen blackout helps no one
go run ./tests/test_audio_drift -w

# quick sanity check (too short to resolve ppm — the CI will say so)
go run ./tests/test_audio_drift -w -minutes 0.5 -interval-ms 200 -segment-s 10

# take SDL's resampler out of the path by matching the device rate
go run ./tests/test_audio_drift -w -rate 48000
```

ESC or Q stops early and still fits whatever was collected.

### Flags

| Flag | Default | Meaning |
|---|---|---|
| `-minutes` | 10 | Tone duration in minutes |
| `-rate` | 44100 | WAV sample rate, Hz |
| `-tone-hz` | 440 | Tone frequency, Hz |
| `-amplitude` | 0.05 | Tone amplitude, 0–1 |
| `-interval-ms` | 500 | Sampling interval, ms |
| `-warmup-s` | 5 | Leading seconds discarded from the fit |
| `-cooldown-s` | 5 | Trailing seconds discarded from the fit (see the tail note above) |
| `-segment-s` | 60 | Per-segment report length, s |
| `-bracket-us` | 200 | Reject samples whose paired read took longer than this |
| `-audio-frames` | 0 | Audio hardware buffer, sample frames (0 = SDL default) |
| `-csv` | `audio_drift.csv` | Per-sample output |
| `-w` | false | Windowed mode — recommended |
| `-d` | -1 | Display index (-1 = primary) |

Each reading brackets the queue query between two clock reads. If the goroutine
is descheduled or the GC pauses it mid-read, the bracket widens and the sample
is **flagged rather than dropped silently** — rejected rows stay in the CSV so a
reanalysis can choose a different threshold.

## A first measurement (not a publishable one)

Two 10-minute runs, 2026-08-08, on a **busy developer machine** — editor,
browser and an agent session live. Not a quiet host. Conditions: PipeWire 1.6.2,
device 44100 Hz / 2 ch / 1024 frames per callback (23.2 ms), WAV 44100 Hz mono
16-bit, 600.0 s nominal, `-amplitude 0.02`.

| | run 1 | run 2 |
|---|---|---|
| samples used | 1188 / 1200 | 1177 / 1200 |
| wide-bracket | 1 | 1 |
| **rate error** | **+4.62 ppm** (CI +1.60 … +7.64) | **+3.59 ppm** (CI +0.53 … +6.66) |
| residual SD | 9.1 ms | 9.1 ms |
| skew / kurtosis | +0.01 / 2.39 | +0.03 / 2.40 |
| lag-1 autocorr | +0.039 | +0.046 |
| intercept | +535.8 ms | +611.3 ms |

The two runs differ by 1.03 ppm, 0.47σ — consistent. Inverse-variance pooled:
**+4.11 ± 1.10 ppm**, i.e. **+2.5 ms per ten minutes**.

Three things stand out.

**The drift is far smaller than the folklore.** An order of magnitude under the
±50–100 ppm crystal tolerance usually quoted, and nowhere near enough to explain
a duration mismatch anyone would notice by ear or by stopwatch. On this machine,
crystal skew is not the explanation for anything. Whether that generalises needs
other hardware.

**The residual distribution reproduced almost exactly** across the two runs — SD,
skew, kurtosis and lag-1 autocorrelation all match to the printed digits. The
measurement process is stable; what varies is the quantity being measured.

**The intercept did not reproduce**: 536 ms versus 611 ms on the same machine,
same settings, minutes apart (and 236–410 ms on shorter runs). The audio
buffered ahead of the DAC is *not* a constant you can calibrate once and
subtract. It is constant *within* a run — which is all the regression needs —
but quoting a fixed audio-output latency for a machine is not supportable from
these data.

Only 1 reading in 1200 exceeded the 200 µs bracket in either run, despite the
load: the sampling is robust, and it is the audio path, not the measurement,
that sets the precision.

### Why the regression, in one line

Three estimators on the same 10-minute recording:

| estimator | ppm |
|---|---:|
| OLS over all 1188 points | **+4.62** |
| means of the first and last 60 s | +4.19 |
| first and last sample only | **−5.79** |

The two-point estimator — which is what timing a file end to end amounts to —
gets the **sign wrong**. Each endpoint carries ~±23 ms of quantisation, and over
594 s that is ±39 ppm of noise on a +4.6 ppm effect. No amount of care with a
stopwatch recovers this; the averaging is what makes it measurable.

(The OLS figure was independently recomputed from the CSV in Python and matched
the Go implementation to all printed digits — an arithmetic check, not a second
measurement.)

## Comparing runs

The ppm figure is a property of *the whole path*, not of the DAC alone. Routing
the same hardware through a different sound server can change it. Every run
therefore records the sound server and version, device rate, channel count and
callback size as `#` metadata in the CSV. A ppm number quoted without them is
not comparable across machines.

To test whether the sound server is contributing, compare the default path
against a direct hardware device:

```bash
# whatever the desktop normally uses
go run ./tests/test_audio_drift -w

# ALSA talking to the hardware directly
SDL_AUDIO_DRIVER=alsa SDL_AUDIO_ALSA_DEFAULT_PLAYBACK_DEVICE=hw:0,0 \
  go run ./tests/test_audio_drift -w
```

**`SDL_AUDIO_DRIVER=alsa` on its own is not enough.** On any PipeWire or
PulseAudio desktop, ALSA's `default` PCM is redirected straight back into the
sound server — check `/etc/alsa/conf.d/`, where a `pcm.!default { type pipewire }`
stanza is typical. Without naming a `hw:` device you bypass nothing. Note that
exclusive `hw:` access locks out every other application, offers no rate
conversion (the WAV rate must be one the hardware accepts natively), and raises
the underrun risk — `Timing-Tests` already runs at 512 frames rather than 256
for exactly that reason on the ALSA path.

## Related

- [`../Timing-Tests`](../Timing-Tests) — `-test av` measures audio *onset*
  latency against a photodiode and TTL box; this test measures *rate*, which
  onset latency cannot see.
- [`../test_av_sync`](../test_av_sync) — `PlaySyncedWithFlip` onset alignment.
