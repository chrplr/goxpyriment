# goxpyriment against the timing mega-study

Bridges D, Pitiot A, MacAskill MR, Peirce JW (2020). *The timing mega-study:
comparing a range of experiment generators, both lab-based and online.*
PeerJ 8:e9414. [DOI 10.7717/peerj.9414](https://doi.org/10.7717/peerj.9414)

Their Table 2 gives visual onset, visual duration, audio onset and audiovisual
synchrony for PsychoPy, PsychToolBox, Presentation, E-Prime, OpenSesame and
Expyriment across three operating systems. This page puts goxpyriment's measured
numbers next to it, on the visual measures.

**Short answer: the display stack decides this, not the package. In a Wayland
session goxpyriment is worse than thirteen of their fourteen lab
configurations. On bare Xorg — the stack they actually measured — it is better
than all fourteen, by a factor of two, at sd 0.083 ms. The same binary, the
same machine, the same night.**

**Caveat: We ran out tests on a different machine as the one on which
the MegaStudy was ran on, so, like the paper's authors, we decline to
generalise across machines.**

The protocol mapping — which flags reproduce their trial structure — lives in
`paper/megastudy/megastudy_timing_tests.md`.

See also [Minimising trigger-to-stimulus jitter](TriggerJitterForEEGandMEG.md),
which is where the mechanism is worked out in detail.

## Conditions

Built-in 2560×1600 laptop panel at **60.0400 Hz** (SDL) / **60.0395 Hz**
(measured from the photodiode train — 8 ppm apart), Mesa, Intel Arc (MTL).
12 frames on, 18 off, 640 cycles, fullscreen, photodiode on the top-left patch.
An Analog Discovery 3 at 200 kS/s recorded the TTL and the photodiode in **one
acquisition**, so the instrument's clock cancels; a Black Box ToolKit v3 was on
the same trigger line throughout. `onset` is the photodiode's 10 % crossing.

Five runs. A→C vary the software while holding the stack fixed; C, D and E vary
only the stack.

| | harness / stack | n | **sd, AD3** | mean, AD3 | steps > 1 ms | TTL > 1 ms off the frame grid |
|---|---|---|---|---|---|---|
| **A** | Wayland, normal priority, goroutine trigger | 598 | 2.342 ms | 21.18 | 82 | 13.07 % |
| **B** | Wayland, FIFO 50, goroutine | 590 | 1.320 ms | 20.96 | 37 | 6.28 % |
| **C** | Wayland, FIFO 50, synchronous | 590 | 1.344 ms | 21.75 | 43 | 7.13 % |
| **D** | **KMS/DRM**, no display server | 591 | **0.113 ms** | 18.91 | **0** | **0.00 %** |
| **E** | **Bare Xorg + openbox**, exclusive fullscreen | 581 | **0.083 ms** | 35.74 | **0** | **0.00 %** |

Photons were more than 1 ms off a whole frame period in **0.00 %** of intervals
in all five runs — median error 6 µs, worst case 169 µs, 2951 trials. The panel
was never the variable.

Three software hypotheses were tested against the fixed Wayland stack. Real-time
priority is worth 1.8×. Firing the trigger synchronously rather than from a
goroutine is worth **nothing** — 1.344 against 1.320, in the wrong direction;
`test_photodiode_latency` logs its own flip-to-trigger gap and puts the entire
host-side path at **median 11 µs, max 38 µs**. Changing the display stack is
worth **16×**.

## The stack decides it, and it is not a single axis

| only the stack differs (all FIFO 50, synchronous trigger) | mean | sd | full range |
|---|---|---|---|
| Wayland session | 21.75 ms | 1.344 ms | 18.83–36.74 |
| KMS/DRM, no display server | 18.91 ms | 0.113 ms | 18.58–19.13 |
| **Bare Xorg + openbox** | **35.74 ms** | **0.083 ms** | **35.52–35.95** |

Xorg is the **steadiest and the latest**. The lag difference is not approximate:

    Xorg − KMS/DRM = 16.826 ms = 1.010 frames

One whole extra buffer in the pipeline, and dead constant. Neither Xorg nor
KMS/DRM puts a single flip more than 1 ms off the frame grid, in 580 and 590
intervals respectively; Wayland misses on one trial in fifteen.

So the answer to "where does X11 fall between the other two" is: neither
between nor beyond. It is a different trade — best-in-class stability bought
with an extra frame of latency.

## Against Table 2

Their "visual onset" is *"the difference between the occurrence of the trigger
pulse and the pixels changing on the LCD screen"* — the same quantity, their
monitor is also 60 Hz, and **bare Xorg is the stack they ran**, which makes E
the honest row to compare.

| configuration | onset Var (ms) |
|---|---|
| **goxpyriment — bare Xorg (E)** | **0.083** |
| **goxpyriment — KMS/DRM console (D)** | **0.113** |
| PsychToolBox Ubuntu · E-Prime Win10 | 0.18 |
| PsychToolBox Win10 · Expyriment Win10 | 0.19 |
| PsychoPy Ubuntu · Presentation Win10 | 0.34 |
| PsychoPy Win10 | 0.35 |
| PsychToolBox macOS | 0.41 |
| OpenSesame Ubuntu | 0.50 |
| PsychoPy macOS | 0.55 |
| OpenSesame Win10 · Expyriment Ubuntu | 0.72 / 0.73 |
| OpenSesame macOS | 0.79 |
| **goxpyriment — Wayland (B, C)** | **1.32 / 1.34** |
| **goxpyriment — Wayland, normal priority (A)** | **2.34** |
| Expyriment macOS | 4.82 |

On the stack they measured, goxpyriment has **the best onset precision in the
table**, twice as good as the best published figure. In a Wayland session the
same binary is worse than thirteen of the fourteen. Nobody's row in that column
is a property of their package alone.

The lag tells the opposite story and should be reported alongside:

| | visual onset lag |
|---|---|
| PsychToolBox / PsychoPy / E-Prime, Linux & Windows | 2.35 – 7.10 ms |
| PsychoPy macOS · PsychToolBox macOS (the 10.13 buffering bug) | 18.24 / 21.52 ms |
| Expyriment macOS — their worst | 29.02 ms |
| **goxpyriment, bare Xorg** | **35.74 ms** |

PsychToolBox's flip returns essentially *at* scanout. goxpyriment's returns two
frames early, and on KMS/DRM one frame early. **That is a software property, not
a hardware one, and it is the one number here that looks reducible.** It costs
nothing scientifically — a constant offset subtracts out of any analysis — but
it is a real difference in how deep the swap chain runs. See `TODO.md`.

**Visual duration Var:** 3.30 / 3.25 ms under Wayland, from 2.6–3.0 % of trials
running exactly one frame long and nothing else; excluding those, 0.15 ms.
Against their Ubuntu rows (PTB 0.15, PsychoPy 1.19, Expyriment 8.31, OpenSesame
9.16) that is mid-pack under a compositor.

## Threshold sensitivity, and where the second instrument stops being usable

The BBTK's Opto1 threshold was left at its default and never calibrated against
this panel. Sweeping the AD3's onset level across the panel's whole rise shows
what a threshold can and cannot affect:

| onset level | Wayland (C): mean / sd | KMS/DRM (D): mean / sd |
|---|---|---|
| 5 % | 21.14 / 1.344 | 18.31 / 0.116 |
| 10 % | 21.75 / 1.344 | 18.91 / 0.113 |
| 50 % | 24.70 / 1.344 | 21.86 / 0.102 |
| 90 % | 27.22 / 1.345 | 24.38 / 0.094 |

A 6 ms sweep moves the **mean by 6 ms and the sd by at most 0.02 ms**. So every
precision figure here is threshold-insensitive and every lag figure is not —
quote a lag only with its level attached, which is why `extract-onsets.py`
records it in the file.

For A–D the BBTK independently reproduced the AD3's sd to three significant
figures (2.33/2.342, 1.32/1.320, 1.35/1.344, 0.17/0.113 — the last at its 250 µs
quantisation floor). **For E it does not, and should not be quoted.** It reports
sd 1.55 ms against the AD3's 0.083, because seven trials read 24–28 ms where the
AD3 saw nothing below 35.52 ms in any of 581. An uncalibrated threshold sitting
high on a 5.5 ms ramp crosses erratically when the final luminance wobbles, and
this is the condition where that finally showed. Calibrating Opto1 against this
panel would be needed before the BBTK can corroborate E, and before any BBTK lag
here can be set beside a published BBTK lag.

## What this licenses saying

That goxpyriment's own timing code is not a limiting factor anywhere: the
host-side trigger path is under 40 µs in every run, and on the stack the
published study used, the trigger-to-photon interval is stable to **83 µs over
five minutes** — better than any configuration in that study.

That a Wayland session costs a factor of sixteen in onset precision on this
machine, and is the dominant term for anyone doing EEG or MEG with a compositor
running, whatever package they use.

That goxpyriment carries one to two frames more presentation latency than
PsychToolBox does. Constant, correctable, and worth fixing anyway.

One machine, one panel. The study's own authors decline to generalise across
machines, and so should this.
