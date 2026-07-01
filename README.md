<!-- DO NOT EDIT. Generated from docs/index.md by cmd/gen-readme. Edit docs/index.md, then run `make readme`. -->

# goxpyriment

`goxpyriment` is a framework for building behavioral and psychological experiments (currently, as Desktop App).

* [Homepage](https://chrplr.github.io/goxpyriment)
* [GitHub repository](https://github.com/chrplr/goxpyriment)

## Features

1. **Zero-dependency deployment.** Your experiments compile to standalone executables. No Python, no conda, no Font issue, no DLL hell on lab computers.
2. **AI-friendly API.** The consistent API is well suited to AI assisted coding: describe your paradigm in plain language to Claude, Gemini, or ChatGPT and the generated code is usually ready to run immediately.
3. **Rich stimuli.** Beyond text and images and audio, present video, generate visual patterns (gratings, Gabor patches, clouds of dots,...), and run high-precision RSVP streams.
4. **Response recording.** Collect responses from keyboard, mouse, and gamepads. Records vocal responses.
5. **Timing precision.** The stimulus loop runs VSYNC-locked with GC pauses disabled, giving sub-millisecond frame jitter on typical hardware.
6. **Hardware synchronization.** Send TTL triggers to synchronize with external devices (EEG, MEG, eye-trackers, fMRI) over parallel port, USB (DLP-IO8), or generic serial.

Remark: If you are looking for a simple experiment generator - with
fixed stimulus presentation schedule, check out
[Gostim2](https://chrplr.github.io/gostim2/) which does not require
any coding.


---

## Documentation

| Document | | PDF |
|---|---|---|
| [Presentation](https://github.com/chrplr/goxpyriment/blob/main/paper/goxpyriment_paper.pdf) | short paper | | 
| [Getting Started](docs/GettingStarted.md) | Tutorials | [↓](docs/GettingStarted.pdf) |
| [Installation](docs/Installation.md) | Install Go and build the examples | [↓](docs/Installation.pdf) |
| [Creating Your Own Experiment](docs/CreatingYourOwnExperiment.md) | Beginner guide: new project, embedding assets, sharing binaries | |
| [User Manual](docs/UserManual.md) | Core concepts explained in depth | [↓](docs/UserManual.pdf) |
| [Migration Guide](docs/MigrationGuide.md) | Coming from Expyriment, PsychoPy, or Psychtoolbox? | [↓](docs/MigrationGuide.pdf) |
| [API Reference](docs/API.md) | Complete function and type reference | [↓](docs/API.pdf) |
| [Gallery of Examples](docs/GalleryOfExamples.md) | Ready-to-run experiments and demos | |
| [Timing-Tests](docs/TimingTests.md) | Check the timing of your computer | [↓](docs/TimingTests.pdf) |


**Support:** 

* [Google group](https://groups.google.com/a/pallier.org/g/goxpyriment) — Forum
* Report bugs at <https://github.com/chrplr/goxpyriment/issues>



--- 

## Quick Start

1. **[Install](docs/Installation.md)** Go and build the bundled examples.
2. **[Create your own experiment](docs/CreatingYourOwnExperiment.md)** — a
   step-by-step beginner guide from an empty folder to a shareable
   executable.
3. **[Getting Started](docs/GettingStarted.md)** walks through worked
   tutorials (trials, data logging, RSVP, reaction times).

---

## Gallery of ready-to-run experiments

[Pre-built binaries](docs/pre-built-examples.md) (ready-to run apps) of
many experiments are available for Windows, macOS, and Linux.

---

## Background

Goexpy is written in [Go](https://go.dev) and relies on
the [SDL3 library](https://libsdl.org) through to the [go-sdl3](https://github.com/Zyko0/go-sdl3) bindings.


The original inspiration was
[expyriment.org](https://github.com/expyriment/expyriment), a
lightweight Python library for cognitive and neuroscientific
experiments (Krause & Lindemann, 2014. *Behavior Research Methods*,
46(2), 416–428. <https://doi.org/10.3758/s13428-013-0390-6>).


---

## License & citation

GNU GPL v3 — see [LICENSE](https://github.com/chrplr/goxpyriment/blob/main/LICENSE.txt).

Please cite as:
> Christophe Pallier (2026) chrplr/goxpyriment. Zenodo. https://doi.org/10.5281/zenodo.19200598


[Christophe Pallier](http://github.com/chrplr), 2026.


---

![](docs/icon_512.png)
