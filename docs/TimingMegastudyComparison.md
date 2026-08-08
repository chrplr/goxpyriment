# goxpyriment against the timing mega-study

Bridges D, Pitiot A, MacAskill MR, Peirce JW (2020). *The timing mega-study:
comparing a range of experiment generators, both lab-based and online.*
PeerJ 8:e9414. [DOI 10.7717/peerj.9414](https://doi.org/10.7717/peerj.9414)

Their Table 2 gives visual onset, visual duration, audio onset and audiovisual
synchrony for PsychoPy, PsychToolBox, Presentation, E-Prime, OpenSesame and
Expyriment across three operating systems. This page puts goxpyriment's measured
numbers next to it, on the visual measures.

**Short answer: with a compositor, goxpyriment is worse than thirteen of their
fourteen lab configurations. Without one — KMS/DRM in a bare console —
it is better than all fourteen. The compositor was the entire jitter.**

The protocol mapping — which flags reproduce their trial structure — lives in
`paper/megastudy/megastudy_timing_tests.md`, which is not tracked because that
directory holds the paper PDF.

See also [Minimising trigger-to-stimulus jitter](TriggerJitterForEEGandMEG.md),
which is where the mechanism is worked out in detail.

One machine, one panel, three five-minute runs. Read the caveats at the end
before quoting any of this: the paper's own authors decline to generalise
across machines, and so should we.

## Conditions

Built-in 2560×1600 laptop panel at **60.0400 Hz** (SDL) / **60.0395 Hz**
(measured from the photodiode train — 8 ppm apart), Mesa, Intel Arc (MTL).
12 frames on, 18 off, 640 cycles, fullscreen, photodiode on the top-left patch.
Both instruments recorded every run simultaneously: an Analog Discovery 3 at
200 kS/s with the TTL and the photodiode in **one acquisition** (so the
instrument's clock cancels), and a Black Box ToolKit v3 on the same trigger
line. `onset` is the photodiode's 10 % crossing.

| | **A** Wayland, normal priority, goroutine trigger | **B** Wayland, FIFO 50, goroutine | **C** Wayland, FIFO 50, **synchronous** | **D** **KMS/DRM bare console**, FIFO 50, synchronous |
|---|---|---|---|---|
| harness | `Timing-Tests -test av -no-sound` | + `chrt -f 50` | `test_photodiode_latency` | same, `SDL_VIDEODRIVER=kmsdrm` |
| n | 598 | 590 | 590 | 591 |
| **sd, AD3** | 2.342 ms | 1.320 ms | 1.344 ms | **0.113 ms** |
| **sd, BBTK** | 2.33 ms | 1.32 ms | 1.35 ms | **0.17 ms** |
| p05–p95, AD3 | 7.36 ms | 3.10 ms | 3.12 ms | **0.38 ms** |
| full range, AD3 | 14.0–38.8 | 18.5–37.6 | 18.8–36.7 | **18.58–19.13** |
| largest trial-to-trial step | 16.5 ms | 18.1 ms | 16.7 ms | **0.275 ms** |
| steps > 1 ms | 82 | 37 | 43 | **0 of 590** |
| TTL > 1 ms off the frame grid | 13.07 % | 6.28 % | 7.13 % | **0.00 %** |
| `ShowTS` > 1 ms off the frame grid | 12.85 % | 6.43 % | — | **0 of 640** |
| **photons > 1 ms off the frame grid** | **0.00 %** | **0.00 %** | **0.00 %** | **0.00 %** |
| mean, AD3 | 21.18 ms | 20.96 ms | 21.75 ms | 18.91 ms |
| mean, BBTK | 23.8 ms | 23.8 ms | 24.4 ms | 21.3 ms |

Three things were tested. Real-time priority is worth a factor of 1.8. Firing
the trigger synchronously rather than from a goroutine is worth **nothing** —
1.344 against 1.320, in the wrong direction. Removing the compositor is worth a
factor of **twelve**.

`test_photodiode_latency` logs its own flip-to-trigger gap, so the host's
contribution is measured rather than inferred: **median 11.0 µs, p95 27.9 µs,
max 37.7 µs**. Under 0.04 ms, in every condition. The trigger code was never
the problem.

## Against Table 2

Their "visual onset" is *"the difference between the occurrence of the trigger
pulse and the pixels changing on the LCD screen"* — the same quantity, and their
monitor is also 60 Hz, so frame quantisation matches. They used a BBTK v2, so
the BBTK column below is the like-for-like instrument.

| configuration | Var (ms) |
|---|---|
| **goxpyriment, KMS/DRM (AD3 0.113)** | **0.17** |
| PsychToolBox Ubuntu · E-Prime Win10 | 0.18 |
| PsychToolBox Win10 · Expyriment Win10 | 0.19 |
| PsychoPy Ubuntu · Presentation Win10 | 0.34 |
| PsychoPy Win10 | 0.35 |
| PsychToolBox macOS | 0.41 |
| OpenSesame Ubuntu | 0.50 |
| PsychoPy macOS | 0.55 |
| OpenSesame Win10 · Expyriment Ubuntu | 0.72 / 0.73 |
| OpenSesame macOS | 0.79 |
| **goxpyriment, Wayland (B, C)** | **1.32 / 1.34** |
| **goxpyriment, Wayland (A)** | **2.34** |
| Expyriment macOS | 4.82 |

On a bare console goxpyriment is at the top of the table; under Wayland it is
near the bottom. Same binary, same machine, same night. **The number in that
column is a property of the display stack far more than of the package**, which
is worth remembering when reading anyone's row in it, including the published
ones.

Note the BBTK's 0.17 ms is at its 250 µs quantisation floor — it is reporting
its own resolution. The AD3, at 1 µs, says 0.113 ms. Several of the published
0.18–0.19 figures are presumably at the same floor.

**Visual duration Var:** 3.30 / 3.25 ms under Wayland, but the shape matters —
it is 2.6–3.0 % of trials running exactly one frame long and nothing else.
Excluding those, 0.15 ms. Against their Ubuntu rows (PTB 0.15, PsychoPy 1.19,
Expyriment 8.31, OpenSesame 9.16) this is mid-pack under a compositor and
best-in-class without one.

## What the compositor did, and the one thing it did not do

**The prediction was half right, and the half that failed is informative.**

Predicted: removing the compositor removes one frame of buffering, so the *mean*
drops by ~16.7 ms to about 5 ms, matching the Linux rows, and the jitter goes
with it.

Observed: the jitter went — completely, 7.13 % of late flips to 0.00 %. The mean
dropped only **2.8 ms**, from 21.75 to 18.91.

So goxpyriment still emits its trigger about one frame before the photons, with
or without a compositor, and that frame is **not** the compositor's doing. What
the compositor added was the *variance* around it. The remaining offset is
consistent with `Update()` returning when the page flip is queued at one vblank
while the content becomes visible at the next: 16.66 ms plus roughly 2 ms of
scanout down to the patch accounts for the 18.9 ms almost exactly.

For EEG and MEG that distinction is everything. A constant 19 ms offset is
subtracted in analysis and costs nothing. The 1.3 ms of scatter around it could
not be.

## Threshold sensitivity, and why the BBTK's calibration does not matter here

The BBTK's Opto1 threshold was left at its default 63 and never calibrated
against this panel, which is why it reads 21.3 ms where the AD3 at 10 % reads
18.9. Sweeping the AD3's onset level across the panel's whole rise shows what
that can and cannot affect:

| onset level | Wayland (C): mean / sd | KMS/DRM (D): mean / sd |
|---|---|---|
| 5 % | 21.14 / 1.344 | 18.31 / 0.116 |
| 10 % | 21.75 / 1.344 | 18.91 / 0.113 |
| 50 % | 24.70 / 1.344 | 21.86 / 0.102 |
| 90 % | 27.22 / 1.345 | 24.38 / 0.094 |

A 6 ms sweep of the threshold moves the **mean by 6 ms and the sd by at most
0.02 ms**. Every precision figure on this page is therefore insensitive to where
either instrument's threshold sits; every lag figure is not. Quote the lag only
with the level attached, which is why `extract-onsets.py` records it in the file.

The lag conclusion survives anyway: even at 5 %, the earliest defensible level,
KMS/DRM gives 18.31 ms against PsychToolBox Ubuntu's 4.53, and no threshold
choice can close 14 ms on a panel whose entire rise is 6 ms.

## What this licenses saying

That goxpyriment's own timing code is not a limiting factor: the trigger path is
under 40 µs, and on a bare console the trigger-to-photon interval is stable to
0.113 ms over five minutes, better than any configuration in the published
table.

That a Wayland session costs a factor of twelve in onset precision on this
machine, and that this is the dominant term for anyone doing EEG or MEG with a
compositor running — whatever package they use.

Not yet tested: **Xorg**, which is the configuration Bridges et al. actually
measured and the one most labs will have. It sits somewhere between these two
and nothing here says where. That needs a different machine.
