# goxpyriment against the timing mega-study

Bridges D, Pitiot A, MacAskill MR, Peirce JW (2020). *The timing mega-study:
comparing a range of experiment generators, both lab-based and online.*
PeerJ 8:e9414. [DOI 10.7717/peerj.9414](https://doi.org/10.7717/peerj.9414)

Their Table 2 gives visual onset, visual duration, audio onset and audiovisual
synchrony for PsychoPy, PsychToolBox, Presentation, E-Prime, OpenSesame and
Expyriment across three operating systems. This page puts goxpyriment's measured
numbers next to it, on the visual measures.

**Short answer: on visual onset precision, goxpyriment as measured here is worse
than thirteen of their fourteen lab configurations, and the mechanism is one
frame of compositor buffering rather than anything in the experiment code.**

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
(measured from the photodiode train — 8 ppm apart), Wayland, Mesa, Intel Arc
(MTL). 12 frames on, 18 off, 640 cycles, fullscreen, photodiode on the top-left
patch. Both instruments recorded every run simultaneously: an Analog Discovery 3
at 200 kS/s with the TTL and the photodiode in **one acquisition** (so the
instrument's clock cancels), and a Black Box ToolKit v3 on the same trigger
line. `onset` is the photodiode's 10 % crossing.

| | **A**: normal priority, trigger from a goroutine | **B**: SCHED_FIFO 50, goroutine | **C**: SCHED_FIFO 50, **synchronous** trigger |
|---|---|---|---|
| harness | `Timing-Tests -test av -no-sound` | same, under `chrt -f 50` | `test_photodiode_latency` |
| n | 598 | 590 | 590 |
| **trigger→light sd, AD3** | 2.342 ms | **1.320 ms** | **1.344 ms** |
| **trigger→light sd, BBTK** | 2.33 ms | **1.32 ms** | **1.35 ms** |
| p05–p95, AD3 | 7.36 ms | 3.10 ms | 3.12 ms |
| mean, AD3 | 21.18 ms | 20.96 ms | 21.75 ms |
| mean, BBTK | 23.8 ms | 23.8 ms | 24.4 ms |
| TTL edges > 1 ms off the frame grid | 13.07 % | 6.28 % | 7.13 % |
| **photons > 1 ms off the frame grid** | **0.00 %** | **0.00 %** | **0.00 %** |

Real-time priority is worth a factor of 1.8. **Firing the trigger synchronously
instead of from a goroutine is worth nothing measurable** — 1.344 against 1.320,
in the wrong direction and well inside run-to-run variation. That was a
prediction and it failed; the goroutine was not the problem.

`test_photodiode_latency` logs its own flip-to-trigger gap, which settles it
directly rather than by inference: **median 14.9 µs, p95 32.1 µs, max 40.2 µs**.
The entire host-side trigger path contributes at most 0.04 ms of a 1.34 ms
figure. Whatever the jitter is, it is not the trigger code.

## Against Table 2

Their "visual onset" is *"the difference between the occurrence of the trigger
pulse and the pixels changing on the LCD screen"* — the same quantity, and their
monitor is also 60 Hz, so frame quantisation matches.

**Visual onset Var (inter-trial sd), Table 2 sorted, with these runs inserted:**

| configuration | Var (ms) |
|---|---|
| PsychToolBox Ubuntu · E-Prime Win10 | 0.18 |
| PsychToolBox Win10 · Expyriment Win10 | 0.19 |
| PsychoPy Ubuntu · Presentation Win10 | 0.34 |
| PsychoPy Win10 | 0.35 |
| PsychToolBox macOS | 0.41 |
| OpenSesame Ubuntu | 0.50 |
| PsychoPy macOS | 0.55 |
| OpenSesame Win10 · Expyriment Ubuntu | 0.72 / 0.73 |
| OpenSesame macOS | 0.79 |
| **goxpyriment, B and C** | **1.32 / 1.34** |
| **goxpyriment, A** | **2.34** |
| Expyriment macOS | 4.82 |

**Worse than thirteen of their fourteen lab configurations.** Seven times
PsychToolBox on Ubuntu, four times PsychoPy on Ubuntu. Only Expyriment on macOS
is worse. There is no reading of this table that puts goxpyriment on par.

**Visual duration Var:** 3.30 / 3.25 ms, but the shape matters — it is 2.6–3.0 %
of trials running exactly one frame long, and nothing else. Excluding those,
0.15 ms, which is PsychToolBox Ubuntu's figure exactly. Against their Ubuntu
rows (PTB 0.15, PsychoPy 1.19, Expyriment 8.31, OpenSesame 9.16) this is
mid-pack, and clearly ahead of Expyriment and OpenSesame.

## Where the gap is, and where it is not

**Not in the display.** Photodiode intervals are a whole number of frames with a
median error of **6 µs**, in all three runs, 1776 trials, zero exceptions. The
panel is exact.

**Not in the trigger.** ≤ 40 µs, measured per trial, above.

**In the flip timestamp.** 6–7 % of `ShowTS` returns are more than 1 ms off the
frame grid, one-sided late, by up to 6 ms. The software learns about the flip
after the photons have left.

The lag says where that comes from. Every Linux and Windows package in Table 2
has a visual onset lag of 2.35–7.10 ms. Ours is 21.7 ms (AD3) / 24.4 ms (BBTK) —
which does not sit with the Linux group at all, but with the macOS group
(PsychoPy 18.24, PsychToolBox 21.52). The difference from the Linux packages is
16.5 ms, one frame at 60 Hz to two decimal places. And the paper's account of
macOS ≥ 10.13 describes what our frame-grid analysis found independently:

> "when the experimental software regards the framebuffer as having 'flipped',
> it has actually just progressed to the next buffering stage and is not yet
> visible on the screen."

They tested Ubuntu 18.04 on the proprietary NVIDIA driver — X11, fullscreen,
unredirected. These runs are Wayland on Mesa. The one-frame signature is what a
compositor holding a buffer looks like.

## What this does and does not license saying

It does **not** show that goxpyriment is a worse-timed package than PsychoPy or
PsychToolBox. It shows that **goxpyriment on Wayland/Mesa, on this machine, has
worse visual onset precision than any Linux or Windows configuration they
measured**, and that the mechanism is one frame of compositor buffering rather
than anything in the experiment code — three separate measurements put the code
at under 40 µs.

Separating the package from the stack needs one run that has not been done:
**the same test under Xorg**, ideally on this machine, and failing that
alongside a same-machine PsychoPy control. Until then the honest statement is
about a configuration, not about a package.

Two things this bench cannot do that the paper did: 1000 trials rather than 590,
and their exact monitor. Neither is likely to matter at this effect size.
