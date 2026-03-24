# goxpyriment

`goxpyriment` is a high-level Go framework for building behavioral and psychological experiments. 



**Just want to run demo experiments?** → Jump to [Demos](#demos).

**Want to write your own experiment?** 

1. Install Go on your machine (see <https://go.dev/doc/install>).
2. Clone this repository (`git clone https://github.com/chrplr/goxpyriment.git` or [download ZIP](https://github.com/chrplr/goxpyriment/archive/refs/heads/main.zip)).
3. Browse [./examples/](./examples/) to see source code of experiments and the documentation :
   * [Getting Started](docs/GettingStarted.md) — Tutorial for psychologists (Python/Expyriment users)
   * [User Manual](docs/UserManual.md) — Core concepts explained in depth
   * [API Reference](docs/API.md) — Complete function and type reference
4. Create a folder for your experiment and write its source code in `main.go`, which you can test by running `go run main.go`. 
 *vibe-coding:* You can start an AI coding agent, e.g., Claude, Gemini, etc., inside the `goxpyriment` folder and ask it to add a new experiment to the `examples` folder (this will lead it to read the existing examples) and describe the experiment (stimuli, design, etc.) in plain language.
5. Once satisfied with the code, compile your experiment into an executable with `go build .`, Voilà!
6. If you want to distribute your code to colleagues using another operating system (Windows, macOS, Linux) and/or architecture (x86_64 or arm64), you can cross-compile, for example to produce code running on a Raspberry-Pi:
```
env GOOS=linux GOARCH=arm64 go build .
```

---

* [Github.io Page](https://chrplr.github.io/goxpyriment)
* [Github repository](https://github.com/chrplr/goxpyriment)
* Report bugs and suggestions at <https://github.com/chrplr/goxpyriment/issues>

Goxpyriment relies on the [libsdl](http://libsdl.org) library through the [go-sdl3](https://github.com/Zyko0/go-sdl3) bindings. Its API is largely inspired by [expyriment.org](http://expyriment.org):

> Krause, F., & Lindemann, O. (2014). Expyriment: A Python library for cognitive and neuroscientific experiments. *Behavior Research Methods*, 46(2), 416–428. <https://doi.org/10.3758/s13428-013-0390-6>

See also [gostim2](http://github.com/chrplr/gostim2) for a simpler, no-code experiment generator.

Christophe Pallier, March 2026

---

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
- **Timing:** millisecond-precision clock, `exp.Wait()`, VSYNC-locked frame cadence via SDL3.

## Installation

### Demos

* **Windows:** Download [`goxpyriment-examples-windows-x86_64-setup.exe`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-windows-x86_64-setup.exe). Execute it; Defender may block you: click "more info" then "Run anyway". By default, experiments are installed in `AppData\Local\Goxpyriment examples\bin` in your user folder. As `AppData` is a hidden folder, select `View > Show > Hidden items` in File Explorer to navigate there.
* **macOS:** Download [`goxpyriment-examples-macos-arm64-app.zip`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-macos-arm64-app.zip), extract it, and drag the `.app` files into a folder of your choice (e.g. `Applications/goxpyriment-examples`).

  > [!WARNING]
  > macOS may show a security warning the first time you open each app. See [macOS installation and security](https://chrplr.github.io/note-about-macos-unsigned-apps) for an explanation and step-by-step instructions to bypass it.
* **Linux:** Download [`goxpyriment-examples-linux-x86_64-appimages.tar.gz`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-linux-x86_64-appimages.tar.gz) and untar it (`tar xzf`). The applications are ready to run.

The source code of [these demos](examples/README.md) can be browsed at  [./examples/](./examples/).

* Report bugs and suggestions at <https://github.com/chrplr/goxpyriment/issues>


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
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperimentFromFlags("Hello World", control.Black, control.White, 32)
	defer exp.End()

	instr  := stimuli.NewTextBox("Press any key to start.", 600, control.FPoint{X: 0, Y: 0}, control.DefaultTextColor)
	hello  := stimuli.NewTextBox("Hello World!", 600, control.FPoint{X: 0, Y: 0}, control.DefaultTextColor)
	finish := stimuli.NewTextBox("Done — press any key to exit.", 600, control.FPoint{X: 0, Y: 0}, control.DefaultTextColor)

	err := exp.Run(func() error {
		exp.Show(instr)
		exp.Keyboard.Wait()
		exp.Show(hello)
		exp.Keyboard.Wait()
		exp.Show(finish)
		exp.Keyboard.Wait()
		return control.EndLoop
	})
	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
```

Run or build examples from within this repository:

```bash
cd examples/hello_world
go run .            # fullscreen by default
go run . -d         # windowed 1024×768 (developer mode)
go run . -d -s 1    # windowed, subject ID = 1
go build .          # build a standalone binary
```

Run any example directly from the repository root:

```bash
go run ./examples/parity_decision/ -d -s 1
```

Most examples accept `-d` (windowed 1024×768 developer mode) and `-s <id>` (subject ID written to the `.xpd` data file).

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

