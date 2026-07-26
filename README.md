<!-- DO NOT EDIT. Generated from docs/index.md by cmd/gen-readme. Edit docs/index.md, then run `make readme`. -->

# goxpyriment

* [Homepage](https://chrplr.github.io/goxpyriment)
* [GitHub repository](https://github.com/chrplr/goxpyriment)

 
  Goxpyriment is a software framework for programming behavioral and
  cognitive experiments using the Go programming language. It was
  designed to address some limitations of existing
  Python-based experiment tools, particularly the runtime environment
  complexity that frequently complicates deployment across
  laboratories. Because Go is a compiled language that can natively
  embed assets (e.g., graphics, audio files, and stimulus lists),
  Goxpyriment compiles entire experiments into single, self-contained
  executable binaries with zero runtime dependencies. This drastically
  simplifies distribution to collaborators and testing computers.  The
  library includes an array of visual stimuli (text, shapes, images,
  Gabor patches, motion clouds, ...) and audio capabilities (WAV
  playback and tone generation).  A lot of effort was devoted to
  ensure timing reliability. For instance, input events are
  timestamped by the operating system at hardware-interrupt time, so
  reaction times are computed by subtracting two OS-level timestamps
  rather than relying on continuous polling. The framework also
  supports sending and receiving TTL signals to synchronize with
  various external equipment. Go's garbage collector can be disabled, greatly
  reducing the probability of unpredictable pauses that could corrupt
  stimulus timing.  Finally, a set of over forty psychology
  experiments implemented in Goxpyriment are provided that promote not
  only learning by humans but also improve the ability of modern
  AI-assisted coding tools to help program experiments. The framework
  is released under the Apache License 2.0.


Remark: If you are looking for a simple experiment generator - with
fixed stimulus presentation schedule, check out
[Gostim2](https://chrplr.github.io/gostim2/) which does not require
any coding.


---

## Documentation

The top navigation bar provides access to various documentation guides.


**Start here — install and build your first experiment**

1. **[Install](docs/Installation.md)** Go and build the bundled examples. [[PDF](docs/Installation.pdf)]
2. **[Creating Your Own Experiment](docs/CreatingYourOwnExperiment.md)** — a
   step-by-step beginner guide from an empty folder to a shareable
   executable.
3. **[Getting Started](docs/GettingStarted.md)** — worked tutorials: trials,
   data logging, RSVP streams, reaction times. [[PDF](docs/GettingStarted.pdf)]

**[User Manual](docs/UserManual.md)** — core concepts explained in depth:
rendering model, timing, input, data, streams, audio, and experimental
design. [[PDF](docs/UserManual.pdf)]

**[API Reference](docs/API.md)** — complete function and type reference,
organized by package. [[PDF](docs/API.pdf)]

**Misc** — focused how-to guides: [migrating from PsychoPy / Expyriment /
Psychtoolbox](docs/MigrationGuide.md), [checking your computer's
timing](docs/TimingTests.md), [video playback](docs/MediaMovies.md), [packaging &
sharing binaries](docs/How_to_package_your_experiment.md), Linux tuning, and
more, all found under the **Misc** tab in the navigation bar at the top
of the page.


In addition, if you clone [goxpyriment](https://github.com/chrplr/goxpyriment), you can run `pkgsite` from the root folder to view the source code documentation.

---

## Gallery of ready-to-run experiments

Browse the [gallery](docs/GalleryOfExamples.md) of over fifty example
experiments and demos ([pre-built binaries](docs/pre-built-examples.md)
(ready-to-run apps) are available for Windows, macOS, and Linux).

---

## Support

* [Google group](https://groups.google.com/a/pallier.org/g/goxpyriment) — Forum
* Report bugs at <https://github.com/chrplr/goxpyriment/issues>

---

## Background

The framework is described in a short paper:
[*Goxpyriment: A Go Framework for Behavioral and Cognitive
Experiments*](https://github.com/chrplr/goxpyriment/blob/main/paper/goxpyriment_paper.pdf).

Goxpyriment is written in [Go](https://go.dev) and relies on
the [SDL3 library](https://libsdl.org) through the [go-sdl3](https://github.com/Zyko0/go-sdl3) bindings.


The original inspiration was
[expyriment.org](https://github.com/expyriment/expyriment), a
lightweight Python library for cognitive and neuroscientific
experiments (Krause & Lindemann, 2014. *Behavior Research Methods*,
46(2), 416–428. <https://doi.org/10.3758/s13428-013-0390-6>).


---

## License & citation

Apache License 2.0 — see [LICENSE](https://github.com/chrplr/goxpyriment/blob/main/LICENSE.txt) and [NOTICE](https://github.com/chrplr/goxpyriment/blob/main/NOTICE).

Please cite as:
> Christophe Pallier (2026) chrplr/goxpyriment. Zenodo. https://doi.org/10.5281/zenodo.19200598


[Christophe Pallier](http://github.com/chrplr), 2026.


---

![](docs/icon_512.png)
