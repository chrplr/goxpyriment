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
4. **Run at real-time priority**, and read the warning about busy-waits in
   [Setting priority under Linux](SettingPriorityUnderLinux.md).
5. **One TTL line per event type.** A multi-bit code on a DLP-IO8 takes ~610 µs
   to settle and will be sampled mid-transition.
6. **Measure it on your own rig.** The numbers below are one host and two
   panels; yours will differ.

Done properly this gives **sd 0.38 ms** on the hardware tested. Each of the
first three is worth a factor of three to twelve, and none of them is visible
in the data if you get it wrong.

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

(Those frame-counted figures include the warm-up trials. Discard them and it
improves to sd 0.38 ms — see below.)

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

| after warm-up | AD3 (1 µs resolution) | BBTK v3 (250 µs) |
|---|---|---|
| trigger→light sd | 0.384 ms | 0.794 ms |
| spread | 1.15 ms | 2.50 ms |
| mean trial-to-trial step | 0.274 ms | 0.425 ms |

The BBTK reads slightly worse throughout, which is what its 250 µs quantisation
predicts: a 0.425 ms mean step is 1.7 of its quanta, so it is measuring its own
resolution floor as much as the display's behaviour. The two are consistent, and
the agreement is worth more than either alone because nothing is shared between
them but the signal.

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
the trigger-to-stimulus delay on the hardware tested is **stable to sd 0.38 ms
over a 1.15 ms range**, on top of a constant offset of 20–40 ms that is
panel-specific and must be measured per display. Occasional whole-frame slips
sit on top of that, and are detectable from the flip timestamps.

That is good enough for EEG and MEG. Sub-millisecond consistency is what the
recording needs, and it is what the measurement shows, from two independent
instruments.

The three ways to lose it are all in this page and all avoidable: sleeping for
the ITI costs a factor of twelve in spread, firing the trigger from a goroutine
costs a factor of forty in the host term, and keeping the warm-up trials costs a
factor of three in sd. None of them announce themselves in the data.

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
