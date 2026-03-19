# goxpyriment

`goxpyriment` is a high-level Go framework for building behavioral and psychological experiments.

* [HTML version](https://chrplr.github.io/goxpyriment) of this document
* [Github repository](https://github.com/chrplr/goxpyriment) of the project
* [Examples of experiments](https://github.com/chrplr/goxpyriment/releases) using this framework.

![](assets/icon_512.png)


**Goxpyriment is well suited to "vibe-code" psychology experiments. (But humans can code with it too)** 

Here is how to proceed: 

1. Install Go on your machine (see <https://go.dev/doc/install>). 
2. Clone this repository  (`git clone https://github.com/chrplr/goxpyriment.git` or [download ZIP](https://github.com/chrplr/goxpyriment/archive/refs/heads/main.zip))
3. Fire your favorite AI coding agent (gemini-cli, claude, cursor...) inside goxpyriment folder and ask it to review the provided examples; then prompt it to program your experiment, describing it in plain language (stimuli, design, etc.)
4. Once the code is created, say in `my_crazy_exp`, you can test it, typically running  `go run my_crazy_exp/main.go` in the terminal.
5. Optionaly, you can create an executable and installers for Windows, MacOS and Linux to disribute to your colleagues. They will not need to install anything beyond your executable on their machine: no need to install Python and whatever libraries, things should work out of the box (see an example at <https://github.com/chrplr/retinotopy-go>)

---

* You can download ready-to-run [examples of experiments](#demos) and look at their [source code](examples/)
* Goxpy relies on the [libsdl](http://libsdl.org) library through the [go-sdl3](https://github.com/Zyko0/go-sdl3) bindings. Its API is largely inspired from [expyriment.org](http://expyriment.org): 

> Krause, F., & Lindemann, O. (2014). Expyriment: A Python library for cognitive and neuroscientific experiments. Behavior Research Methods, 46(2), 416-428. <https://doi.org/10.3758/s13428-013-0390-6>.

* Check out [gostim2](http://github.com/chrplr/gostim2) for a less flexible but very simple (no-code!) experiment generator.**
* This software is a beta version. Please report bugs and suggestions for improvement at <https://github.com/chrplr/goxpyriment/issues>.


Christophe Pallier, March 2026

---

## Features

- **Experimental Design:** Easily define Experiments, Blocks, and Trials with support for factors and randomization.
- **Rich Stimuli Library:**
  - **Visual:** Text (lines and boxes), shapes (rectangles, circles), images, fixation crosses, and Gabor patches.
  - **Audio:** Playback of WAV files and synthetic tones.
- **Hardware Acceleration:** Seamless integration with SDL3 for smooth, high-performance stimulus presentation.
- **Input Handling:** Simplified interfaces for Keyboard and Mouse events.
- **Data Collection:** Automatic logging of trial data to `.xpd` files for later analysis.
- **Timing:** High-precision timing utilities for stimulus duration and reaction time measurement.

## Prerequisites

- Zero, Nada, None (!) to run precompiled experiments.
- [Go](http://go.dev), version 1.25 or higher, if you want to modify or program experiments.

By the way, while Python is easy, Go is simple, which is [a good thing](gemini-go-vs-python.md)



## Installation

### Demos

You can download [here](https://github.com/chrplr/goxpyriment/releases) an installer of ready-to-run [examples of experiments](examples/README.md) created with goxpyeriment (and check their [source code](examples/) if you want).

* **Windows:** Download  `goxpyriment-examples-setup.exe`. Execute it; Defender may block you: then just click on "more info" and click "Run anyway". By default, the experiments are installed in `AppData\Local\Goxpyriment examsples\bin`, in your user folder. As `AppData` is an hidden folder, you have to select `View/Show/Hidden items` in File Exporer to be able to see it. Navigate in the subfolder and click on an App to execute it.
* **MacOS:** Download goexpyriment-examples.dmg', open it and drag all the files into a single `goxpyriment-examples` folder in your Applications folder. Then provide correct permissions with:

        chmod -R +x /Applications/goxpyriment-examples/*

   If `settings > confidentiality & security` indicates that an application is blocked, click on "open anyway".
* **Linux**: Download `goxpyriment-example-appimages.tar.gz` and untar it (`tar xzf`). The applications are ready to run.
 

### Install the library on your computer to compile source code

```bash
go get github.com/chrplr/goxpyriment
```

## Quick Start

Here is the code of a simple "Hello World" experiment (also at `examples/hello_world/main.go`):

```go
package main

import (
	_ "embed"  // to embed stimuli files, fonts, etc. inside the executable
	"flag"
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

//go:embed assets/bonjour.wav
var bonjourWav []byte

func main() {
	develop := flag.Bool("d", false, "Developer mode (windowed 1024x1024)")
	subject := flag.Int("s", 0, "Subject ID")
	flag.Parse()

	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 1024, 1024, false
	}

	exp := control.NewExperiment(
		"My First Go Experiment",
		width, height, fullscreen,
		control.Black, // background color
		control.White, // foreground (text) color
		32,            // default font size in points
	)
	exp.SubjectID = *subject
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	greetings := stimuli.NewTextBox("Hello World !", 600, control.FPoint{X: 0, Y: 100}, control.DefaultTextColor)
	instr := stimuli.NewTextBox("Press any key to start the experiment", 600, control.FPoint{X: 0, Y: 100}, control.DefaultTextColor)
	finish := stimuli.NewTextBox("Experiment Finished!\nPress any key to exit.", 600, control.FPoint{X: 0, Y: 100}, control.DefaultTextColor)

	sound := stimuli.NewSoundFromMemory(bonjourWav)
	if err := sound.PreloadDevice(exp.AudioDevice); err != nil {
		log.Printf("Warning: failed to load sound: %v", err)
	}

	exp.Run(func() error {
		if err := stimuli.PlayPing(exp.AudioDevice); err != nil {
			log.Printf("Warning: failed to play ping: %v", err)
		}
		instr.Present(exp.Screen, true, true)
		exp.Keyboard.Wait()

		sound.Play()
		greetings.Present(exp.Screen, true, true)
		exp.Keyboard.Wait()

		finish.Present(exp.Screen, true, true)
		exp.Keyboard.Wait()

		return control.EndLoop
	})
}
```

To run it directly from within this repository:

```bash
cd examples/hello_world
go run .            # fullscreen by default
go run . -d         # windowed 1024×1024 (developer mode)
go run . -d -s 1    # windowed, subject ID = 1
```

To build a standalone binary:

```bash
cd examples/hello_world
go build -o hello_goxpy .
./hello_goxpy -d
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

## Building and Running Examples

Run any example directly from the repository root:

```bash
go run ./examples/parity_decision/ -d -s 1
```

Or build and run the binary:

```bash
cd examples/parity_decision
go build .
./parity_decision -d -s 1
```

Most examples accept:
- `-d` — windowed 1024×1024 developer mode (default is exclusive fullscreen)
- `-s <id>` — subject ID written into the `.xpd` data file

To build all examples at once:

```bash
cd examples
./build.sh
```


## License

This project is licensed under the GNU Public License v3 - see the [LICENSE](LICENSE.txt) file for details.

Christophe Pallier, 2026


