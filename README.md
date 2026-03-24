# goxpyriment

`goxpyriment` is a high-level Go framework for building behavioral and psychological experiments. 

* [Getting Started](docs/GettingStarted.md) — Tutorial for psychologists
* [User Manual](docs/UserManual.md) — Core concepts explained in depth
* [API Reference](docs/API.md) — Complete function and type reference
* [Source code](./examples/) of example experiments (To try them → Jump to [Demos](#demos))
* [Github.io Page](https://chrplr.github.io/goxpyriment)
* [Github repository](https://github.com/chrplr/goxpyriment)

If you are looking for a simpler, *no-code experiment generator*, check [Gostim2](https://chrplr.github.io/gostim2/). 


<!--
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
-->

Goxpyriment relies on the [libsdl](http://libsdl.org) library through the [go-sdl3](https://github.com/Zyko0/go-sdl3) bindings. 
(While Python is easy, Go is simple: read [Go-vs-Python](gemini-about-go-vs-python.md)). The code was mostly written with Claude Sonnet 4.6, with some input from Gemini 2.5 flash.

As its name suggests, goxpyriment was inspired by [expyriment.org](https://github.com/expyriment/expyriment?tab=readme-ov-file), a nice, light-weight Python library for cognitive and neuroscientic experiments (See Krause, F., & Lindemann, O. (2014). *Behavior Research Methods*, 46(2), 416–428. <https://doi.org/10.3758/s13428-013-0390-6>). The API should feel very familiar to expyriment users.

> :warning: This software is in beta-testing: although it is certainly possible to use it to implement real experiments in the lab, users should (as always) very carefully check their behavior, for example with a [bbtk](https://chrplr.github.io/bbtkv3/).
> Please report bugs and suggestions at <https://github.com/chrplr/goxpyriment/issues>

If you use in a scientific paper, please cite as:

> Christophe Pallier (2026) chrplr/goxpyriment: Goxpyriment examples v0.7.13 (v0.7.13). Zenodo. https://doi.org/10.5281/zenodo.19200598
> :warn: update the version if you use a more recent one!


---

## How to write your own experiment, in a nutshell 

1. Install Go on your machine (see <https://go.dev/doc/install>).
2. Clone this repository (`git clone https://github.com/chrplr/goxpyriment.git` or [download ZIP](https://github.com/chrplr/goxpyriment/archive/refs/heads/main.zip)).
3. Browse [./examples/](./examples/) to see source code of experiments and the documentation :
4. Create a folder for your experiment and start coding in a `main.go` file. You can test it by running `go run main.go`. 
> [!TIP]
> *Vibe-coding:* Launch an AI coding agent (Claude, Gemini, etc.) inside the `goxpyriment` folder and ask it to add a new experiment to the `examples` folder — this leads the agent to read the existing examples for context. Describe the experiment (stimuli, design, etc.) in plain language and enjoy.
> Recommendation: save your prompt in a `description.md` file alongside the code.
5. Once satisfied with the code, compile your experiment into an executable with `go build .`. This executable will run on any machine with the same OS and architecture. 
6. If you need to distribute your experiment to colleagues who use another operating system or architecture, you can [cross-compile](https://golangcookbook.com/chapters/running/cross-compiling/). For example, to produce code for a [Raspberry-Pi](https://en.wikipedia.org/wiki/Raspberry_Pi):
```
env GOOS=linux GOARCH=arm64 go build .
```


### Demos

#### Binaries

* **Windows:** Download [`goxpyriment-examples-windows-x86_64-setup.exe`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-windows-x86_64-setup.exe). Execute it; Defender may block you: click "more info" then "Run anyway". By default, experiments are installed in `AppData\Local\Goxpyriment examples\bin` in your user folder. As `AppData` is a hidden folder, select `View > Show > Hidden items` in File Explorer to navigate there.
* **macOS:** Download [`goxpyriment-examples-macos-arm64-app.zip`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-macos-arm64-app.zip), extract it, and drag the `.app` files into a folder of your choice (e.g. `Applications/goxpyriment-examples`).

  > [!WARNING]
  > macOS may show a security warning the first time you open each app. See [macOS installation and security](https://chrplr.github.io/note-about-macos-unsigned-apps) for an explanation and step-by-step instructions to bypass it.
* **Linux:** Download [`goxpyriment-examples-linux-x86_64-appimages.tar.gz`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-linux-x86_64-appimages.tar.gz) and untar it (`tar xzf`). The applications are ready to run.

There are many programs, my suggestion is to start with `Memory_span`, `Change_Blindess`, `retinotopy`,... 

Most examples accept `-d` (windowed 1024×768 developer mode) and `-s <id>` (subject ID written to the `.xpd` data file).

#### Source code

The source code of [these demos](examples/README.md) can be browsed at  [./examples/](./examples/).

If [Go](https://go.dev) is installed on your computer, and you have run `go get github.com/chrplr/goxpyriment`, you can run any example directly from a clone of the goxpyriment repository, for example:

```bash
go run ./examples/parity_decision/ -d -s 1
```

Or:

```bash
cd examples/hello_world
go run .            # fullscreen by default
go run . -d         # windowed 1024×768 (developer mode)
go run . -d -s 1    # windowed, subject ID = 1
go build .          # build a standalone binary
```

To build all examples at once:

```bash
cd examples
./build.sh
```

> :memo: Report bugs and suggestions at <https://github.com/chrplr/goxpyriment/issues>

## License

This project is licensed under the GNU Public License v3 - see the [LICENSE](LICENSE.txt) file for details.

---

![](assets/icon_512.png)

