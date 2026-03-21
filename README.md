# goxpyriment

`goxpyriment` is a high-level Go framework for building behavioral and psychological experiments.

* [HTML version](https://chrplr.github.io/goxpyriment) of this document
* [Github repository](https://github.com/chrplr/goxpyriment) of the project
* Report bugs and suggestions at <https://github.com/chrplr/goxpyriment/issues>


**Just want to run demo experiments?** → Jump to [Demos](#demos).

**Want to write your own experiment?** Goxpyriment is well suited to "vibe-coding" psychology experiments — but humans enjoy coding with it too:

1. Install Go on your machine (see <https://go.dev/doc/install>).
2. Clone this repository (`git clone https://github.com/chrplr/goxpyriment.git` or [download ZIP](https://github.com/chrplr/goxpyriment/archive/refs/heads/main.zip)).
3. Fire your favorite AI coding agent (gemini-cli, claude, cursor...) inside the `goxpyriment` folder and ask it to review the provided examples; then prompt it to program your experiment, describing it in plain language (stimuli, design, etc.).
4. Once the code is created, e.g. in `my_experiment/`, test it by running `go run my_experiment/main.go` in the terminal.
5. Optionally, build installers for Windows, macOS, and Linux to distribute to colleagues — no Python or libraries needed on their machines (see <https://github.com/chrplr/retinotopy-go> for an example).

Goxpyriment relies on the [libsdl](http://libsdl.org) library through the [go-sdl3](https://github.com/Zyko0/go-sdl3) bindings. Its API is largely inspired by [expyriment.org](http://expyriment.org):

> Krause, F., & Lindemann, O. (2014). Expyriment: A Python library for cognitive and neuroscientific experiments. *Behavior Research Methods*, 46(2), 416–428. <https://doi.org/10.3758/s13428-013-0390-6>

See also [gostim2](http://github.com/chrplr/gostim2) for a simpler, no-code experiment generator.

Christophe Pallier, March 2026

---

## Features

- **Visual stimuli:** text (single-line and word-wrapped boxes), shapes (rectangles, circles, lines, polygons), fixation crosses, images, Gabor patches, drifting sinusoidal gratings, random-dot clouds, random-dot stereograms, off-screen canvases, and visual masks.
- **Audio stimuli:** WAV playback (from file or embedded bytes), procedurally generated tones (pure or multi-frequency with linear ramps), and time-windowed segment playback with fade-in/fade-out.
- **Video playback:** MPEG video files and `.gv` (LZ4-compressed RGBA) sequences, both VSYNC-locked.
- **Stimulus streams:** high-precision RSVP presentation of image, text, or audio sequences with per-stimulus timing logs and user-event capture.
- **Animated stimuli:** VSYNC-locked loops for moving dot clouds, drifting gratings, and drifting Gabor patches; GC disabled during loops for stable frame timing.
- **Experimental design:** Experiments → Blocks → Trials with arbitrary string factors; block shuffling; between-subjects Latin-square counterbalancing.
- **Randomization:** shuffled sequences, random draws, truncated normal sampling, and constrained shuffling (maximum consecutive repetitions, minimum gap between repetitions).
- **Input handling:** keyboard (blocking/non-blocking, multi-key, timeout, reaction-time measurement) and mouse (position, button detection); serial port for hardware triggers.
- **Data collection:** trial data logged to `.xpd` files (CSV with metadata header) with automatic subject ID, timestamp, and display-info fields.
- **Timing:** millisecond-precision clock, `Wait()` / `WaitUntil()`, VSYNC-locked frame cadence via SDL3.

## Installation

### Demos

You can download [here](https://github.com/chrplr/goxpyriment/releases) ready-to-run [examples of experiments](examples/README.md) created with goxpyriment (and check their [source code](examples/) if you want).

* **Windows:** Download [`goxpyriment-examples-windows-x86_64-setup.exe`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-windows-x86_64-setup.exe). Execute it; Defender may block you: click "more info" then "Run anyway". By default, experiments are installed in `AppData\Local\Goxpyriment examples\bin` in your user folder. As `AppData` is a hidden folder, select `View > Show > Hidden items` in File Explorer to navigate there.
* **macOS:** Download [`goxpyriment-examples-macos-arm64-app.zip`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-macos-arm64-app.zip), extract it, and drag the `.app` files into a folder of your choice (e.g. `Applications/goxpyriment-examples`).

  > [!WARNING]
  > macOS may show a security warning the first time you open each app. See [macOS installation and security](https://chrplr.github.io/note-about-macos-unsigned-apps) for an explanation and step-by-step instructions to bypass it.
* **Linux:** Download [`goxpyriment-examples-linux-x86_64-appimages.tar.gz`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-linux-x86_64-appimages.tar.gz) and untar it (`tar xzf`). The applications are ready to run.

### Install Go and the library to compile source code

Download and install Go from <https://go.dev>. While Python is easy, Go is simple, which is [a good thing](gemini-go-vs-python.md).

```bash
go get github.com/chrplr/goxpyriment
```

## Quick Start

Here is the code of a minimal "Hello World" experiment (the full version with audio is at `examples/hello_world/main.go`):

```go
package main

import (
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperimentFromFlags("Hello World", control.Black, control.White, 32)
	defer exp.End()

	instr  := stimuli.NewTextBox("Press any key to start.", 600, control.FPoint{X: 0, Y: 0}, control.DefaultTextColor)
	hello  := stimuli.NewTextBox("Hello World!", 600, control.FPoint{X: 0, Y: 0}, control.DefaultTextColor)
	finish := stimuli.NewTextBox("Done — press any key to exit.", 600, control.FPoint{X: 0, Y: 0}, control.DefaultTextColor)

	exp.Show(instr)
	exp.Keyboard.Wait()
	exp.Show(hello)
	exp.Keyboard.Wait()
	exp.Show(finish)
	exp.Keyboard.Wait()
}
```

Run or build examples from within this repository:

```bash
cd examples/hello_world
go run .            # fullscreen by default
go run . -d         # windowed 1024×1024 (developer mode)
go run . -d -s 1    # windowed, subject ID = 1
go build .          # build a standalone binary
```

Run any example directly from the repository root:

```bash
go run ./examples/parity_decision/ -d -s 1
```

Most examples accept `-d` (windowed 1024×1024 developer mode) and `-s <id>` (subject ID written to the `.xpd` data file).

To build all examples at once:

```bash
cd examples
./build.sh
```

Cross-compiling is [straightforward](https://golangcookbook.com/chapters/running/cross-compiling/) in Go — you can build binaries for Windows, macOS, and Linux (Intel or ARM) from any machine.

## Project Structure

- `control/`: Experiment lifecycle and state management (window, fonts, colors).
- `design/`: Tools for building the experimental structure (Trials, Blocks).
- `stimuli/`: A comprehensive library of visual and auditory stimuli.
- `io/`: Screen, Keyboard, and Mouse handling.
- `clock/`: Timing utilities.
- `geometry/`: Geometry utilities.
- `examples/`: Ready-to-run examples (Stroop task, Lexical Decision, etc.).

## License

This project is licensed under the GNU Public License v3 - see the [LICENSE](LICENSE.txt) file for details.

Christophe Pallier, 2026

---

![](assets/icon_512.png)

