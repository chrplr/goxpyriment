# Timing measurement campaign — plan proposal

Drafted 2026-11-24, after the paper's timing results were cleared (both the
"Timing Validation" section and the appendix results now read "in progress...").

Kit: a photodiode, a DLP-IO8, a Digilent Analog Discovery 3 (AD3).

Rule for the paper: **every number must be traceable to a named capture file,
and every feature claim must be checked against the code.**

## Decide first

1. **Do the 2026-08-08 laptop runs still count?** They were ruled campaign data
   then, but the paper's timing results are now all cleared. Simplest:
   re-measure everything.
2. **Linux only, or Windows and macOS too?** The paper's Limitations section says
   timing is characterised on Linux only. One Windows and one macOS run would
   answer it. The DLP-IO8 works on all three, and the AD3 can run from a second
   machine.
3. **The RT-accuracy experiment needs one extra part** if it should cover the
   keyboard path: an optocoupler across one key of a cheap USB keyboard. Without
   it, only the TTL-input path (into the DLP-IO8) can be tested.
4. **PsychoPy comparison:** on one configuration, or every one?

## Before the first capture

- **Freeze the software.** Tag a release and run every machine from that commit,
  so every `-info.txt` names the version the paper cites.
- **Check the open vblank issue, or leave `GOXPY_VBLANK` out.** A 2026-11-24 code
  audit suggests that with `GOXPY_VBLANK=on`, a driver whose present blocks holds
  two frames per `Update`. It is unmeasured. A 2-minute check on the Pi settles
  it: `GOXPY_VBLANK=on go run ./tests/Timing-Tests -test display`, then read the
  pacing stats. The default path is unaffected.
- **Write the analysis before the data exists.** Fix the onset criterion now
  (e.g. 50 % of each trial's own dark→bright step), plus the statistics. That is
  exactly what went wrong in the old tables, whose medians disagreed with their
  captions.
- **Two small tools are missing:**
  - **AD3 response generator:** for the RT test, the AD3 has to close a switch at
    a fixed delay after the photodiode edge, triggered internally with no host
    involved. `ad3-capture` only records today.
  - **GC load option:** `Timing-Tests` has `-gc`, but its loop allocates almost
    nothing, so GC on vs off would measure nothing. It needs an option that
    allocates N MB per frame.

## Wiring

The AD3 records two analog channels at a time. Its digital inputs are 3.3 V, so
the DLP-IO8's 5 V TTL goes on an analog channel. Captures use
`--rate 1e5 --save-raw`.

| Run type | CH1 | CH2 |
|---|---|---|
| Visual | photodiode, top-left patch | DLP-IO8 TTL |
| Audio-visual | photodiode | audio line out |
| RT | photodiode | the AD3's own response output, wired back |

Rig rules from August:

- AD3 on a root USB port, never through a dock (it stalls at about 20 samples/s).
- A probe on each analog input: the inputs are differential, and a bare lead
  reads mains hum.
- Warm each machine up for about 10 minutes first (the panel rate moved 23 ppm
  in 6 minutes).
- Record the photodiode's actual position.
- Confirm the physical setup (diode on the panel, leads attached) before every
  run.

## The matrix

**Use one monitor everywhere,** the Dell 1905FP, so that machine comparisons do
not also compare panels. Separately, measure one machine on both the 1905FP and
its built-in panel, to quantify the panel's own contribution.

| Machine | Stacks |
|---|---|
| Precision 5490 (`is158520`) | KMS/DRM, Wayland |
| AMD W5700 workstation (`pcstim7`) | X11 exclusive fullscreen, KMS/DRM |
| Raspberry Pi 4 | KMS/DRM (TTL from GPIO instead of the DLP-IO8) |
| Optional | one Windows and one macOS machine |

## What each experiment measures

1. **Durations and dropped frames.**
   - `Timing-Tests -test av -no-sound`: 1010 cycles of 12 frames on / 18 off,
     about 8.5 min, with the first 10 discarded.
   - Two runs per configuration.
   - Report the median, SD, p5–p95, and the count of trials more than 1 ms off.
2. **Trigger→light,** from the same runs. This is the number EEG/MEG analysis
   needs, and it is directly measurable because both edges are on the AD3's one
   timebase.
3. **Flip timestamp→photons: only drift (ppm) and de-trended scatter.**
   - The absolute offset cannot be measured this way: the clock fit cancels any
     constant trigger latency. That is what made the old "within 20 µs" claim
     unsupported.
   - A loopback (DLP-IO8 output wired to one of its inputs, read back by the
     host) bounds the DLP-IO8's own latency; state it as a bound.
4. **Audio.**
   - `-test av -audio-frames N` with N = 256, 512, 1024, 2048, 500 cycles each
     (about 4 min per buffer size).
   - One machine per sound device: the 5490's on-board audio and `pcstim7`'s
     Scarlett.
   - Play a silent warm-up tone first: PipeWire suspends an idle sink, and the
     first real tone goes missing.
5. **RT accuracy (new).** The AD3 "responds" a fixed delay after each photodiode
   edge, through the keyboard (needs the optocoupler) or through a DLP-IO8 input.
   `Timing-Tests -test rt`, 500 trials. The error is the recorded RT minus the
   known delay.
6. **GC on/off (new).** On one configuration, with the GC-load option.
   Interleave the runs A B A B, never A A B B, so thermal drift does not
   masquerade as an effect.
7. **PsychoPy head-to-head (new).** Same machine, monitor and wiring. Ideally
   reuse Bridges et al.'s published scripts, so the comparison follows their
   protocol.
8. **Onset marker (`exp.Marker`).** It has never been validated on a rig. One
   short run with `-photodiode -ttl dlpio8` checks that the square and the TTL
   code land on the same flip.

## Rough bench time per configuration

About 10 min warm-up, plus 2 × 9 min visual runs, plus about 17 min of audio on
the two machines that carry it.

| Item | Time |
|---|---|
| Core Linux matrix (5 configurations) | about 4–5 h |
| GC experiment | about 40 min |
| RT experiment | about 30 min per input path |
| PsychoPy comparison | about 20 min per configuration |

## Analysis rules

- **Traceability:** every number traceable to a named capture file.
- **Labels:** the configuration label comes from the `sys video_driver:` line in
  each `-info.txt`, never from a directory name.
- **Pairing and fitting:** pair events by nearest neighbour and fit periods by
  regression. Both traps bit in August.
- **Sanity checks:** check every result against physical bounds before believing
  it. Durations must be whole frames, lags cannot be negative, and audio scatter
  cannot exceed one buffer period. Impossible numbers are what caught every
  analysis bug in August.

## Note on the old Apparatus text

The Phase-2 runbook (2026-08) says the Dell 1905FP was on the Pi and `pcstim7`.
That matches `docs/TimingTests.md` and contradicts the paper's Apparatus
paragraph ("shared between pcstim7 and is158520"), which should be rewritten
from the new campaign anyway.
