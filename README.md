# goxpyriment

`goxpyriment` is a high-level Go framework for building behavioral and psychological experiments. 

* [Getting Started](docs/GettingStarted.md) — Tutorial for psychologists
* [User Manual](docs/UserManual.md) — Core concepts explained in depth
* [API Reference](docs/API.md) — Complete function and type reference
* [Demos](./examples/README.md) — source code, descriptions, and pre-built binaries for many experiments
* [Github.io Page](https://chrplr.github.io/goxpyriment)
* [Github repository](https://github.com/chrplr/goxpyriment)

If you are looking for a simpler, *no-code experiment generator*, check [Gostim2](https://chrplr.github.io/gostim2/). 


## Features

- **Visual stimuli:** text (single-line `TextLine` and word-wrapped `TextBox`), shapes (rectangles, circles, lines, filled polygons), fixation crosses, images (`Picture`), Gabor patches, drifting sinusoidal gratings, random-dot clouds, random-dot stereograms, off-screen canvases, visual masks, thermometer displays, stimulus circles, visual multiple-choice grids (`ChoiceGrid`), and text input boxes.
- **Audio stimuli:** WAV playback (from file or embedded bytes), procedurally generated pure and complex tones with linear ramps, time-windowed segment playback with fade-in/fade-out, and embedded feedback sounds (buzzer, ping).
- **Video playback:** MPEG video files and `.gv` (LZ4-compressed RGBA) sequences, both VSYNC-locked.
- **Stimulus streams:** high-precision RSVP presentation of image, text, or audio sequences with per-stimulus timing logs and user-event capture.
- **Animated stimuli:** VSYNC-locked loops for moving dot clouds, drifting gratings, and drifting Gabor patches; GC disabled during loops for stable frame timing.
- **Experimental design:** Experiments → Blocks → Trials with arbitrary string factors; block shuffling; between-subjects Latin-square counterbalancing.
- **Randomization:** shuffled sequences, random draws, truncated normal sampling, and constrained shuffling (maximum consecutive repetitions, minimum gap between repetitions).
- **Input handling:** keyboard (blocking/non-blocking, multi-key, timeout, reaction-time measurement) and mouse (position, button detection); hardware trigger devices (serial/USB DLP-IO8, parallel port) via the separate `triggers` package.
- **Data collection:** trial data logged to `.xpd` files (CSV with metadata header) with automatic subject ID, timestamp, and display-info fields.

---

## How to write your own experiment, in a nutshell 

1. Install [Git](https://git-scm.com/install/) and [Go](https://go.dev/doc/install) on your machine (See [detailed instructions](docs/Installing-a-development-environment.md)).
2. Open a Terminal (`Git Bash` under Windows), clone the goxpyrimentrepository (`git clone https://github.com/chrplr/goxpyriment.git` or [download ZIP](https://github.com/chrplr/goxpyriment/archive/refs/heads/main.zip)).
3. Browse [./examples/](./examples/) to see the source code of various experiments and the [/.docs](./docs/) 
4. Create a folder for your experiment and start coding a `main.go` file. You can test it by running `go run main.go`. 
> [!TIP]
> *Vibe-coding:* Launch an AI coding agent (Claude, Gemini, etc.) inside the `goxpyriment` folder and ask it to add a new experiment to the `examples` folder — this leads the agent to read the existing examples for context. Describe the experiment (stimuli, design, etc.) in plain language and enjoy.
> Recommendation: save your prompt in a `description.md` file alongside the code.
5. Once satisfied with the code, compile your experiment into an executable with `go build .`. This executable will run on any machine with the same OS and architecture. 
6. If you need to distribute your experiment to colleagues who use another operating system or architecture, you can [cross-compile](https://golangcookbook.com/chapters/running/cross-compiling/). For example, to produce code for a [Raspberry-Pi](https://en.wikipedia.org/wiki/Raspberry_Pi):
```
env GOOS=linux GOARCH=arm64 go build .
```

> [!WARNING] 
> This software is in beta-testing, that is, I am waiting for more reports from the battleground before releasing v1.0.0.  
> Although it is certainly possible to use it to implement real experiments in the lab, users should (as always) very carefully check their behavior, for example with a [bbtk](https://chrplr.github.io/bbtkv3/).
> Please report bugs and suggestions at <https://github.com/chrplr/goxpyriment/issues>

## License

This project is licensed under the GNU Public License v3 - see the [LICENSE](LICENSE.txt) file for details.

If you use it, please cite as:

* Christophe Pallier (2026) chrplr/goxpyriment: Goxpyriment examples v0.7.13 (v0.7.13). Zenodo. https://doi.org/10.5281/zenodo.19200598
*(updating the version if you use a more recent one!)*

## Origin of the project

Goxpyriment relies on the [libsdl](http://libsdl.org) library through the [go-sdl3](https://github.com/Zyko0/go-sdl3) bindings. 
(While Python is easy, Go is simple: read [Go-vs-Python](gemini-about-go-vs-python.md)). The code was mostly written using Claude Sonnet 4.6, with some input from Gemini 2.5 flash.

As its name suggests, goxpyriment was inspired by [expyriment.org](https://github.com/expyriment/expyriment?tab=readme-ov-file), a nice, light-weight Python library for cognitive and neuroscientic experiments (See Krause, F., & Lindemann, O. (2014). *Behavior Research Methods*, 46(2), 416–428. <https://doi.org/10.3758/s13428-013-0390-6>). The API should feel very familiar to expyriment users.


[ChrPlr](https://github.com/chrplr), March 2026.


---

![](assets/icon_512.png)

