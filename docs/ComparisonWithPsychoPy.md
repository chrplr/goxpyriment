---
title: goxpyriment compared with PsychoPy
author: Claude Opus and Christophe Pallier
date: 2026-07-21
---

# goxpyriment compared with PsychoPy

[PsychoPy](https://www.psychopy.org) is the established, most widely used
platform for creating and running psychological experiments. This page compares
it with goxpyriment feature by feature, so that you can judge whether
goxpyriment fits your paradigm — and, where it does not, what you would be
giving up.

The comparison is based on the PsychoPy documentation sources
([psychopy/psychopy-docs](https://github.com/psychopy/psychopy-docs), `release`
branch) and on the current state of this repository.

**Summary in one sentence:** PsychoPy optimises for *accessibility and breadth*
(a GUI builder, online deployment, support for nearly every laboratory device);
goxpyriment optimises for *deployability and timing determinism* (one static
binary, no interpreter, no garbage collection inside the presentation loop).

If you are porting existing PsychoPy code, read the
[Migration Guide](MigrationGuide.md) — it gives a concept map and side-by-side
code for a representative trial sequence.

---

## The structural difference

PsychoPy is three layers stacked on top of each other:

1. **Builder** — a graphical editor in which an experiment is assembled from
   routines, loops, and ~45 drag-and-drop components. It compiles to Python
   *or* to JavaScript.
2. **Coder** — the `psychopy.*` Python library, which the Builder generates
   calls into and which you can use directly.
3. **PsychoJS / Pavlovia** — a JavaScript runtime plus a hosting platform, with
   integrations for Prolific, MTurk, Qualtrics, and Sona.

goxpyriment is only the middle layer: a Go library. There is no graphical
builder and no hosting platform. That single fact explains most of the gaps
listed below — and most of the advantages.

---

## Where goxpyriment matches or has an edge

| Aspect | PsychoPy | goxpyriment |
|---|---|---|
| Deployment | Python environment plus wheels, or Pavlovia hosting | A single self-contained binary. SDL3, SDL3_ttf and SDL3_image are embedded as compressed blobs and loaded at startup — nothing to install on the target machine |
| Presentation loop | Python, subject to the GIL and to interpreter overhead; Psychtoolbox/ioHub back-ends are used for precision | VSYNC-locked loops with the garbage collector disabled (`debug.SetGCPercent(-1)`) for the duration of the critical section |
| Clock | `psychopy.clock`, `core.getTime` | `clock` package built on SDL's high-resolution performance counter |
| RSVP and rapid streams | Written by hand as a frame-by-frame loop | A dedicated high-precision path, `stimuli.PresentStreamOfImages`, with variants for images, text, tones, mixed streams, and per-frame callbacks |
| Adaptive procedures | `StairHandler`, `QuestHandler` | `staircase.UpDown` (Levitt 1971) and `staircase.Quest` (Watson & Pelli 1983) — the same two families |
| Unit conversion | `psychopy.monitors`, `deg`/`cm`/`norm` units | `units` package: pixels ↔ degrees ↔ cm through a `Monitor` struct |
| Counterbalancing | `CounterbalanceRoutine` (a recent addition) | `design.AddBWSFactor` / `GetPermutedBWSFactorCondition` — Latin-square between-subject assignment, present from the start |
| Gamma correction | `monitors` calibration | `apparatus.GammaCorrector` — per-channel exponents, `exp.SetGamma` / `exp.CorrectColor` |
| Data output | `.csv` / `.psydat` / Excel | Plain `.csv` (no comment lines, imports straight into a spreadsheet) plus a companion `-info.txt` holding the session metadata; both written through a buffered output file |
| System reporting | `psychopy.info` | `sysinfo` package: CPU, GPU, audio, memory, machine — including a WMI path on Windows |
| Hardware triggers | Parallel, serial, LabJack, Cedrus, EGI, BrainProducts, TriggerBox | Parallel port (Linux LPT), DLP-IO8, FT232H, Linux GPIO (Raspberry Pi and other SBCs), LabJack T4 over Modbus TCP, NeuroSpin MEG TTL box, generic serial |
| Networked recording control | Egi/`iohub`, `emulator`, MQTT/OSC components | EGI NetStation event markers over TCP/IP (ECI), plus a "BEL_video" participant-video recorder client |
| Worked examples | A demos folder | over 50 complete paradigms in `examples/` (plus ~30 feature demos), plus a `tests/` suite of programs whose output is analysed to verify timing and hardware behaviour |

The core of an experiment — window, text, images, shapes, gratings, dot fields,
keyboard and mouse input, sound, trial structure, randomisation, staircases,
data files — is covered by both.

---

## Where PsychoPy is ahead

### 1. Eye tracking

This is the single largest gap. PsychoPy's **ioHub** provides a vendor-neutral
API over EyeLink, Tobii, GazePoint and Pupil Labs, with Builder routines for
calibration and validation and a region-of-interest component.

goxpyriment has no eye-tracking support at all — no gaze stream, pupillometry,
calibration, or region-of-interest components. (It *does* ship a
`triggers.VideoRecorder` client for a networked participant-video recorder, but
that only records and labels footage of the participant; it is not gaze
tracking and returns no eye position.)

### 2. Breadth of visual stimuli

PsychoPy's `visual` module documents 42 classes. Notable ones without a
goxpyriment equivalent:

- **`ElementArrayStim`** — thousands of elements drawn in a single call.
  Required for Glass patterns, large-scale multiple-object tracking, and
  texture-defined stimuli. Probably the most consequential omission after eye
  tracking.
- **`Aperture`** — restricting drawing to an arbitrary region.
- **`RadialStim`**, **`NoiseStim`**, **`SecondOrder`** — specialised
  psychophysical textures.
- **`Form`**, **`Slider`**, **`RatingScale`** — questionnaire and rating
  widgets. goxpyriment offers `stimuli.ChoiceGrid`,
  `stimuli.ThermometerDisplay` and `stimuli.TextInput`, which cover some but
  not all of this ground.
- **`BufferImageStim`** — snapshotting a rendered screen for fast redisplay.
- **`windowWarp`** — spherical and cylindrical warping for domes and curved
  screens.
- The entire 3D/VR branch: `rift`, `SphereStim`, `ObjMeshStim`,
  `PhongMaterial`, `LightSource`, `SceneSkybox`.

### 3. Online experiments

PsychoPy's `online/` documentation covers Pavlovia synchronisation,
participant-recruitment integrations (Prolific, MTurk, Qualtrics, Sona),
credit and statistics dashboards, and compatibility checking.

goxpyriment compiles to WebAssembly and runs in the browser (see
[WASM.md](WASM.md)), so the *runtime* half of this exists. What does not exist
is the surrounding service: hosting, recruitment, and server-side data
collection are left to you.

### 4. Video

PsychoPy plays whatever ffmpeg can decode. goxpyriment plays MPEG-1 directly
(pure Go, with MP2 audio) and requires conversion to `.gv` for timing-critical
clips — `cmd/gv-convert` does this, needing no external tools for MPEG-1 input
and ffmpeg for anything else. See [UserManual §15](UserManual.md#15-video).

The `.gv` requirement is a deliberate trade: independent per-frame blocks remove
the decode-cost asymmetry between I- and B-frames, which is what makes H.264
onset timing jitter periodically. But it is still friction, the files are roughly
an order of magnitude larger, and it rules out playing arbitrary
participant-supplied media at measured onsets.

### 5. Ecosystem and tooling

- A **plugin architecture** for third-party components.
- The **`Session`** API for running several experiments under one process.
- **Alerts and linting** that catch common design errors before a run.
- A **logging framework** with configurable levels.
- A substantial **teaching corpus** and community.

### 6. Camera and speech

PsychoPy has a `CameraComponent`, Google Speech integration, and audio/visual
validator routines. goxpyriment provides `apparatus.Microphone`,
`apparatus.VoiceKey` and WAV writing, but no camera capture and no speech
recognition.

### 7. The Builder itself

For readers who do not program, the Builder is not a convenience — it is the
only way in. goxpyriment requires writing Go.

---

## Choosing between them

**Reach for PsychoPy when** you need eye tracking; when the paradigm depends on
large element arrays, apertures, or 3D/VR; when the study runs online with
recruitment-platform integration; when arbitrary video must be played; or when
the person building the experiment does not program.

**Reach for goxpyriment when** the experiment must be distributed as a binary
that runs on a machine you do not control and cannot install software on; when
timing determinism matters more than stimulus breadth (rapid serial
presentation, tight audio-visual synchronisation, hardware-triggered
acquisition); when Python environment drift has been a recurring problem; or
when you are already comfortable in Go and want the compiler to catch design
errors before the participant arrives.

The two are not mutually exclusive. A common pattern is to prototype in
PsychoPy and re-implement the timing-critical parts in goxpyriment once the
design is settled.

---

## See also

- [Migration Guide](MigrationGuide.md) — concept map and side-by-side code for
  Expyriment, PsychoPy and Psychtoolbox
- [Timing Tests](TimingTests.md) — measured presentation timing on real hardware
- [Gallery of Examples](GalleryOfExamples.md) — the full catalogue of examples
  and tests
- [WASM build](WASM.md) — running experiments in the browser
