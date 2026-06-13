# goxpyriment

`goxpyriment` is a high-level Go framework for building behavioral and psychological experiments.

* [GitHub repository](https://github.com/chrplr/goxpyriment)

## Features

1. **Zero-dependency deployment.** Your experiments compile to standalone executables.
2. **Timing precision.** The stimulus loop runs VSYNC-locked with GC pauses disabled, giving sub-millisecond frame jitter on typical hardware (VSYNC can be disabled to support Variable Refresh Rate monitors).
3. **AI-friendly API.** The API is well suited to "vibe-coding": after cloning the repository, describe your paradigm in plain language to Claude, Gemini, or ChatGPT and try the generated code. Given the examples and the constraints afforded by the framework, the experiment is likely to work as expected (but must be checked, of course).

---

## Documentation

| Document | | PDF |
|---|---|---|
| [Presentation](https://github.com/chrplr/goxpyriment/blob/main/paper/goxpyriment_paper.pdf) | short paper | | 
| [Getting Started](GettingStarted.md) | Tutorials | [↓](GettingStarted.pdf) |
| [Installation](Installation.md) | Install Go and build the examples | [↓](Installation.pdf) |
| [Creating Your Own Experiment](CreatingYourOwnExperiment.md) | Beginner guide: new project, embedding assets, sharing binaries | |
| [User Manual](UserManual.md) | Core concepts explained in depth | [↓](UserManual.pdf) |
| [Migration Guide](MigrationGuide.md) | Coming from Expyriment, PsychoPy, or Psychtoolbox? | [↓](MigrationGuide.pdf) |
| [API Reference](API.md) | Complete function and type reference | [↓](API.pdf) |
| [Gallery of Examples](GalleryOfExamples.md) | Ready-to-run experiments and demos | |
| [Timing-Tests](TimingTests.md) | Check the timing of your computer | [↓](TimingTests.pdf) |


**Support:** 

* [Google group](https://groups.google.com/a/pallier.org/g/goxpyriment) — Forum
* Report bugs at <https://github.com/chrplr/goxpyriment/issues>


Note: If you are looking for a simpler, *no-code experiment generator*, check out [Gostim2](https://chrplr.github.io/gostim2/).


--- 

## Quick Start

1. **[Install](Installation.md)** Go and build the bundled examples.
2. **[Create your own experiment](CreatingYourOwnExperiment.md)** — a step-by-step beginner guide from an empty folder to a shareable executable.
3. **[Getting Started](GettingStarted.md)** walks through worked tutorials (trials, data logging, RSVP, reaction times).

---

## Ready-to-run experiments

[Pre-built binaries](pre-built-examples.md) (ready-to run apps) of many experiments are available for Windows, macOS, and Linux.

---

## Background

Goxpyriment relies on [libsdl](http://libsdl.org) via the [go-sdl3](https://github.com/Zyko0/go-sdl3) bindings.

It was inspired by [expyriment.org](https://github.com/expyriment/expyriment), a lightweight Python library for cognitive and neuroscientific experiments (Krause & Lindemann, 2014. *Behavior Research Methods*, 46(2), 416–428. <https://doi.org/10.3758/s13428-013-0390-6>). The API should feel familiar to expyriment users.

---

## License & citation

GNU GPL v3 — see [LICENSE](https://github.com/chrplr/goxpyriment/blob/main/LICENSE.txt).

Please cite as:
> Christophe Pallier (2026) chrplr/goxpyriment. Zenodo. https://doi.org/10.5281/zenodo.19200598


[Christophe Pallier](http://github.com/chrplr), 2026.


---

![](icon_512.png)
