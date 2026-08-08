# Minimising trigger-to-stimulus jitter (EEG / MEG)

In EEG and MEG the TTL trigger is the timestamp. Everything downstream — epoching,
averaging, latency measurement — is referred to it. So the question is not how
quickly the stimulus appears after the trigger, but **how *consistently* it does**.

That distinction decides what you have to fix:

- **A constant offset is harmless.** If the screen always lights 22 ms after the
  trigger, subtract 22 ms in the analysis and nothing is lost.
- **Jitter is not recoverable.** A trigger-to-stimulus delay that varies from
  trial to trial smears every average by that amount, and no analysis can undo
  it because nothing recorded says which trial was late.

Everything below is aimed at the second quantity.

## The short version

1. **Never sleep for an inter-trial interval. Count frames.**
2. **Fire the trigger synchronously, on the flip thread, immediately after the
   flip returns** — not from a goroutine.
3. **Discard the first several trials.** They are measurably different.
4. **Run at real-time priority.** Measured worth: it halves the jitter, 2.34 ms
   to 1.32 ms over five minutes. Read the warning about busy-waits in
   [Setting priority under Linux](SettingPriorityUnderLinux.md).
5. **One TTL line per event type.** A multi-bit code on a DLP-IO8 takes ~610 µs
   to settle and will be sampled mid-transition.
6. **Measure it on your own rig.** The numbers below are one host and two
   panels; yours will differ.

Done properly, and at real-time priority, this gives **sd 1.3 ms over a
five-minute run** on the hardware tested — against 2.3 ms for the same run at
normal priority. None of it is the display, which put every one of 1188 stimuli
across both runs on an exact frame boundary. It is the host being late to notice
the flip. See "The jitter is in the timestamp, not in the display".

## Where the time actually goes

Measured on a Linux 7.0 laptop, 60 Hz panel, DLP-IO8 trigger and a photodiode on
the screen, both recorded in one acquisition on an Analog Discovery 3 so the
instrument's clock cancels:

| stage | contribution |
|---|---|
| `ShowTS` returns → TTL write issued | **25 µs** (p95 33 µs) |
| DLP-IO8 host write → edge on the wire | tens of µs (bounded ≤ 0.79 ms) |
| flip → photons on the panel | **20–40 ms**, and this is where all the variance is |
| panel black→white transition (10–90%) | **5.5–6.5 ms** |

**The software path is three orders of magnitude below the display.** Nothing in
your experiment code, the Go runtime, or the trigger box is what limits you.

## Why sleeping is the mistake

An inter-trial interval written as `time.Sleep(300 * time.Millisecond)` is not a
whole number of frames: at 60 Hz it is 18.01 of them. Each trial therefore starts
at a slightly different point in the frame cycle, and the stimulus flip lands at
a drifting phase. Measured over 35 trials with a sleep-based ITI:

| | sleep ITI | frame-counted ITI |
|---|---|---|
| trigger→light spread | **14.1 ms** | **3.4 ms** |
| sd | 2.80 ms | 1.13 ms |
| host clock vs display clock | drifts 0.27 ms per trial | — |

(Both from 35-trial runs. Over five minutes the frame-counted figure is worse —
sd 2.3 ms — for the reason in "Measure over a realistic run length" below. The
comparison between the two ITI styles stands; the absolute value does not.)

The failure mode is worth recognising because it does **not** look like noise. The
trigger-to-light delay walks smoothly across a run — 27 ms at the start, 41 ms in
the middle, back to 37 ms — as the host's trial period slides past the display's
frame period. Two things follow:

- **A short pilot will not reveal it.** Ten trials sample a small arc of the walk
  and look tight.
- **Within-channel checks will not reveal it either.** Trigger-to-trigger and
  flash-to-flash intervals are both perfectly stable while their *relative*
  phase drifts. If your timing check measures only interval regularity, it will
  pass.

Count frames instead:

```go
// Inter-trial interval as 18 blank frames -- exactly 300 ms at 60 Hz, and
// exactly a whole number of frames at any refresh rate.
for f := 0; f < 18; f++ {
    exp.Screen.ClearAndUpdate()   // Update blocks on VSYNC
}
```

This keeps the display pipeline continuously fed, so the stimulus flip stays
vsync-locked. `tests/Timing-Tests` has always done this; it is why its numbers
have always been good.

## Fire the trigger synchronously

```go
onset, err := exp.ShowTS(stim)   // presents and timestamps the flip
if err != nil { return err }
trig.SetHigh(line)               // next statement, same thread, nothing between
```

Measured this way the gap between the flip timestamp and the trigger write is
**25 µs**. Launching the same call as `go triggers.FireTrigger(...)` hands it to
the Go scheduler instead; at normal priority under CPU load that path has been
measured at **+0.73 ms with about 1 ms of spread**. That is forty times the
synchronous figure, for no benefit at the scale that matters here.

`tests/Timing-Tests` fires from a goroutine deliberately, to keep its own
measurement loop unblocked. Do not copy that pattern into an experiment.

## What is left, and why it is quantised

After the two fixes above, the residual is not a smear — it is **whole frames**.
Measured over 34 consecutive intervals in a frame-counted run, every single one
was an exact multiple of the frame period: mostly 20 frames, occasionally 21.
None were in between.

So the display is genuinely vsync-locked, and the remaining variability is a
trial occasionally slipping one frame later. At 60 Hz that is 16.7 ms when it
happens; at 120 Hz, 8.3 ms.

This matters because a quantised error is **detectable**, and a smeared one is
not. `ShowTS` returns the flip timestamp, so an experiment can log it and find
the slips afterwards:

```go
onset, _ := exp.ShowTS(stim)
if prev != 0 {
    frames := float64(onset-prev) / float64(frameDur)
    if math.Abs(frames-math.Round(frames)) > 0.25 || math.Round(frames) != expected {
        // this trial's stimulus was a frame late: mark it, or drop it
    }
}
prev = onset
```

For EEG and MEG this is the difference between an unknown error and a known one.
Log the flip timestamp on every trial and put it in the data file.

## Measure over a realistic run length, not a pilot

This is the trap that caught the author of this page, twice.

Trial to trial the delay is extremely stable: the median step between
consecutive trials is **0.06 ms**. Sample thirteen trials and the sd comes out
at 0.38 ms, which looks superb. Sample 598 — five minutes, a realistic block —
and it is **2.34 ms**, six times worse.

Nothing changed except the observation window. The delay sits on a plateau,
drifting by tens of microseconds per trial, and then jumps:

| trial-to-trial step | value |
|---|---|
| median | 0.06 ms |
| p95 | 3.01 ms |
| max | 16.47 ms (one frame) |
| steps > 1 ms | 82 of 597 |
| steps > 3 ms | 30 of 597 |

Over 598 trials the delay ranged from 14 to 27 ms, with two excursions to 38 ms.
The jumps are not all whole frames — 3.3, 4.0, 6.5, 10.5 and 16.5 ms all occur —
so a dropped frame is not the explanation. The next section is.

**A short pilot will tell you the timing is excellent, and it will be wrong.**
Measure over the length of a real block.

## The jitter is in the timestamp, not in the display

This is the most useful thing on this page, and it took a five-minute run to
see. Recording the trigger and the photodiode on one instrument gives three
event trains, and asking whether each one falls on a whole number of frame
periods separates them completely:

| train, intervals between consecutive trials | off a whole frame by > 1 ms |
|---|---|
| **photons on the panel** | **0 of 597 (0.0 %)** |
| TTL edge on the wire | ~5 % of trials, up to 6.8 ms |
| the host's own `ShowTS` timestamp | **82 of 638 (12.9 %)**, up to 6.7 ms |

The panel is exact. Photodiode intervals sit on a whole number of frames with a
median error of **5 µs** and a worst case of 169 µs, across 553 intervals of
exactly 30 frames, 37 of 31 and 7 of 32. The implied frame period, 16.6557 ms,
is **60.0395 Hz** against the 60.0400 Hz SDL reports for the panel — agreement
to 8 ppm, from an instrument that knows nothing about the display.

What wanders is the moment the software believes the flip happened. And it
wanders in one direction: across the 594 intervals that were exactly 30 frames,
the host timestamp was never early (minimum −0.01 ms) and was late by as much as
**+6.12 ms**. `Update()` returned, `ShowTS` stamped the clock, and the photons
had already been on their way for several milliseconds.

The trigger inherits that error, because it is fired off the flip's return. So
the 2.3 ms of trigger-to-stimulus jitter is not the display and not the trigger
box:

> **The stimulus appears on an exact frame boundary every time. The TTL that is
> supposed to mark it is occasionally several milliseconds late.**

Two things follow, and they point in opposite directions from the usual advice:

- **Do not treat `ShowTS`'s return as a photon timestamp.** It is right to
  within microseconds most of the time and several milliseconds wrong about one
  trial in eight, with no indication of which.
- **The error is bounded and reconstructible.** Because the photons are on an
  exact grid, the true onsets of a whole block can be recovered by fitting that
  grid to the flip timestamps, which is not possible for genuinely random noise.

### Real-time priority halves it — and that is measured, not assumed

A one-sided, several-millisecond lateness in returning from a vsync wait is what
preemption looks like. The run above was at normal priority, so the same
protocol was repeated changing nothing but `chrt -f 50`:

| five minutes, same rig, same night | SCHED_OTHER | SCHED_FIFO 50 |
|---|---|---|
| trigger→light sd, **AD3** | 2.342 ms | **1.320 ms** |
| trigger→light sd, **BBTK v3** | 2.33 ms | **1.32 ms** |
| p05–p95 spread (BBTK) | 7.2 ms | **3.0 ms** |
| `ShowTS` timestamps > 1 ms off the frame grid | 12.85 % | **6.43 %** |
| TTL edges > 1 ms off the frame grid | 13.07 % | **6.28 %** |
| **photons > 1 ms off the frame grid** | **0.00 %** | **0.00 %** |
| trial-to-trial steps > 3 ms | 30 of 597 | **10 of 589** |
| mean delay | 21.18 ms | 20.96 ms |

Real-time scheduling **halves the jitter** and leaves the mean where it was,
which is the signature of removing a delay that only ever happened sometimes.
Both instruments agree on the improved figure to three significant figures, as
they did on the worse one.

The off-grid fractions for `ShowTS` and for the TTL fall together — 12.85 → 6.43
and 13.07 → 6.28 — which confirms the trigger is simply following the flip
timestamp and contributes nothing of its own. And the photons stay exactly where
they were: perfectly frame-locked in both conditions. Scheduling never touched
the display, only the software's knowledge of it.

**It is not a complete fix.** 6.4 % of flips are still late, and 1.32 ms is not
1.32 µs. Whatever remains is not answered here.

⚠️ **`tests/Timing-Tests` does not request real-time priority**, so its own
numbers — including the SCHED_OTHER column above — are at normal priority unless
you launch it under `chrt`. It builds its experiment with
`control.NewExperiment`, and the elevation lives in
`control.NewExperimentFromFlags`, so nothing is attempted and nothing is
logged. Experiments built the normal way, through `NewExperimentFromFlags`, do
ask. Check `sched_policy` in your data file's header rather than assuming
either way, and see [Setting priority under
Linux](SettingPriorityUnderLinux.md).

## Discard the first trials — they are genuinely different

The first few trials after a run starts have a longer and more variable delay
than everything that follows, and they dominate the summary statistics if left
in. Same recording, the only difference being how many leading trials are
excluded:

| | all trials | discarding 10 |
|---|---|---|
| trigger→light sd | 1.127 ms | **0.384 ms** |
| spread | 3.44 ms | **1.15 ms** |

Three times the sd, from the first handful of trials. On the BBTK recording of
the same test the raw sequence starts at 32.25 ms and settles to about 25 ms
within four trials, so the transient is real and not an artifact of one
instrument.

`tests/Timing-Tests` already discards ten cycles (`-warmup`). An experiment
should do the equivalent: present some warm-up trials before the first one that
counts, or mark the early trials in the data so the analysis can drop them.

## Two instruments agree

Everything above was measured with an Analog Discovery 3. The same
`Timing-Tests` run was also recorded with a Black Box ToolKit v3 — a different
sensor, a different front end and its own clock — with both instruments on the
same trigger line:

| five-minute run, ~598 trials | AD3 (1 µs resolution) | BBTK v3 (250 µs) |
|---|---|---|
| trigger→light **sd** | **2.340 ms** | **2.327 ms** |
| spread | 24.74 ms | 24.75 ms |
| p05 – p95 | 17.1 – 24.5 ms | 19.8 – 27.0 ms |
| mean | 21.18 ms | 23.84 ms |

The two instruments agree on the variability to three decimal places while
differing by 2.7 ms in the mean. That is exactly the expected pattern: the
offset depends on where each instrument's threshold sits on the panel's 5.5 ms
ramp, and is constant; the jitter does not depend on the threshold at all. It is
also the strongest evidence available that the number is real, since the two
share nothing but the signal.

Over a short window the BBTK looks worse than the AD3 (sd 0.79 against 0.38 ms
across thirteen trials) because a 0.43 ms mean step is only 1.7 of its 250 µs
quanta. Over a realistic run that resolution difference is irrelevant — the
quantity being measured is ten times its resolution.

## How this compares with other packages

Bridges et al. (2020) measured this same quantity — trigger pulse to pixels
changing — for PsychoPy, PsychToolBox, Presentation, E-Prime, OpenSesame and
Expyriment, on a 60 Hz panel. Their best is 0.18 ms and their worst is 4.82 ms.
The 1.32 ms here is worse than thirteen of their fourteen lab configurations,
and the lag identifies why: ours is one frame longer than every Linux and
Windows configuration they tested, which is the signature of a compositor
holding a buffer. See
[the mega-study comparison](TimingMegastudyComparison.md).

## The floor you cannot code around

**Frame quantisation.** A stimulus can only appear when the panel scans it out.
The only way to reduce this is a faster panel: 16.7 ms of quantisation at 60 Hz
becomes 4.2 ms at 240 Hz.

**Panel transition time.** Measured 10–90% on two unrelated LCDs, one external
4K and one laptop panel: **6500 µs and 5571 µs**. A multi-millisecond
black-to-white transition appears to be a property of the technology, not of one
bad monitor. It is also why quoting an onset to better than a millisecond is not
meaningful without saying which point on the ramp you mean.

**Scanout position.** The top of the panel lights nearly a frame before the
bottom. Put the photodiode — and the stimulus that matters — near the top, and
record where it was.

## The honest assessment for EEG / MEG

With a frame-counted ITI, a synchronous trigger and warm-up trials discarded,
the trigger-to-stimulus delay on the hardware tested has **sd 2.3 ms across a
five-minute block**, on a mean of about 21 ms that is panel-specific. Two
independent instruments agree on that figure.

Where that jitter lives matters more than its size. The panel puts the stimulus
on an exact frame boundary on every one of 1188 trials measured; the software is
late to notice on 6 % of them at real-time priority and 13 % at normal priority.
The jitter is in the timestamp, not in the photons.

**Whether this is good enough depends on the paradigm.** For ERP components with
latencies of tens of milliseconds, averaged over many trials, 1.3 ms of onset
jitter is a small smear. For anything resolving fine temporal structure — early
auditory components, phase measures, single-trial latency — measure it on your
own rig before relying on it.

Unlike a genuinely noisy display, this one has somewhere to go. Real-time
priority is worth a factor of two and is one flag. Beyond that, the frame grid
is exact enough — 6 µs — to reconstruct the true onsets after the fact, which is
possible precisely because the residual is not random.

The three ways to make it worse are all in this page and all avoidable: sleeping
for the ITI costs a factor of twelve in spread, firing the trigger from a
goroutine costs a factor of forty in the host term, and keeping the warm-up
trials costs a factor of three in sd. None of them announce themselves in the
data.

The robust answer for both cases is the same one used in MEG labs generally:
**record a photodiode alongside the TTL** and use the photodiode as the onset in
analysis. That converts every term on this page — offset, quantisation, panel
rise — into a measured per-trial quantity rather than an assumption.

## Verify it on your own hardware

```bash
# Does this display block on VSYNC, and at what rate?
go run ./tests/test_vsync_blocking

# Trigger against a photodiode, one AD3 acquisition, per-trial gap logged
go run ./tests/test_photodiode_latency -s 1 -isi-frames 18 -diode all

# The same with the established harness
go run ./tests/Timing-Tests -test av -no-sound
```

`tests/test_photodiode_latency` logs the flip timestamp, the trigger timestamp
and the gap between them for every trial, so the host-side contribution is in
the data rather than assumed. Its README works through the arithmetic of
converting an instrument's trigger-to-light interval into the quantity you
actually want.
