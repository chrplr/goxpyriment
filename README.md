# goxpyriment


goxpyriment` is a high-level Go framework for building behavioral and psychological experiments.

It is directly inspired from [expyriment.org](http://expyriment.org) ; see Krause, F., & Lindemann, O. (2014). Expyriment: A Python library for cognitive and neuroscientific experiments. Behavior Research Methods, 46(2), 416-428. <https://doi.org/10.3758/s13428-013-0390-6>.

Actually, to start, I gave Gemini 3 the link to [expyriment's API documentation](https://docs.expyriment.org/expyriment.html) and asked it to try and implement it in Go using the [go-sdl3](https://github.com/Zyko0/go-sdl3) bindings for [SDL3]{libsld.org)


**NOTE: This software in an alpha version, a proof of concept that without any doubt has some bugs. It need thourough tessting and robustification. Also, future versions may not be compatible at the API level! So if you want to try and use it, I recommand you to clone this repository. Check out [expe3000-go](http://github.com/chrplr/expe3000-go) for a less ambitious but efficient, no-code, experiment generator ** 


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

- **Go:** Version 1.25 or higher.
- **SDL3:** The SDL3 development libraries must be installed on your system.
  - On Linux: Use your package manager (e.g., `sudo apt install libsdl3-dev`).
  - On macOS/Windows: Follow the official SDL3 installation guides.

## Installation

```bash
go get github.com/chrplr/goxpyriment
```

## Quick Start

Here is the code `main.go` of a simple "Hello World" experiment.

```go
package main

import (
	_ "embed"   # to embed stimuli files, font, ... inside the executable
	"flag"
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"

	"github.com/Zyko0/go-sdl3/sdl"
)

//go:embed assets/Inconsolata.ttf
var inconsolataFont []byte

//go:embed assets/bonjour.wav
var bonjourWav []byte

func main() {
	fullscreen := flag.Bool("F", false, "Launch in fullscreen display mode")
	flag.Parse()

	exp := control.NewExperiment("My First Go Experiment", 1368, 1024, *fullscreen)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	if err := exp.LoadFontFromMemory(inconsolataFont, 32); err != nil {
		log.Printf("Warning: failed to load font: %v. Using fallback.", err)
	}

	greetings := stimuli.NewTextBox("Hello World !", 600, sdl.FPoint{X: 0, Y: 100}, control.DefaultTextColor)
	instr := stimuli.NewTextBox("Press any key to start the experiment", 600, sdl.FPoint{X: 0, Y: 100}, control.DefaultTextColor)
	finish := stimuli.NewTextBox("Experiment Finished!\n Press any key to exit.", 600, sdl.FPoint{X: 0, Y: 100}, control.DefaultTextColor)

	sound := stimuli.NewSoundFromMemory(bonjourWav)
	if err := sound.PreloadDevice(exp.AudioDevice); err != nil {
		log.Printf("Warning: failed to load sound: %v", err)
	}

	// Run the experiment logic
	exp.Run(func() error {
		instr.Present(exp.Screen, true, true)
		exp.Keyboard.Wait()
		sound.Play()

		greetings.Present(exp.Screen, true, true)
		exp.Keyboard.Wait()

		finish.Present(exp.Screen, true, true)
		exp.Keyboard.Wait()

		return sdl.EndLoop
	})

}
```

Assuming Go is installed on your computer (<https://go.dev/doc/install>), you can generate an executable program from the above code:

```bash
# Note: the files are in `examples/hello_word` in this repository 
cp main.go assets/Inconsolata.ttf assets/bonjour.wav  ~/tmp
cd ~/tmp
go mod init hello
go mod tidy
go build . -o hello_goxpy
```

You can then execute `hello_goxpy` from any location on your computer.
Moreover, as cross-compiling is [trivial](https://golangcookbook.com/chapters/running/cross-compiling/) in Go, one can easily generate apps for Windows, MacOS and Linux, on intel or arm architecture, from any computer.
 
 




, read [this](examples/hello_world/README.md)


## Project Structure

- `control/`: Experiment lifecycle and state management.
- `design/`: Tools for building the experimental structure (Trials, Blocks).
- `stimuli/`: A comprehensive library of visual and auditory stimuli.
- `io/`: Screen, Keyboard, and Mouse handling.
- `misc/`: Timing and geometry utilities.
- `examples/`: Ready-to-run examples (Stroop task, Lexical Decision, etc.).

## Building and Running Examples

You can build all examples at once using the provided script:

```bash
cd examples
./build.sh
```

To run a specific example, navigate to its directory and use `go run`:

```bash
cd examples/parity_decision
go run .
```

Alternatively, you can build and run the binary:

```bash
cd examples/parity_decision
go build .
./parity_decision
```

### Example Highlights

The repository includes several experimental paradigms in the `examples/` directory:

#### Retinotopy Mapping
A high-performance implementation of Retinotopic Mapping stimuli (wedges, rings, and bars) using 15 Hz dynamic alpha-masking.
```bash
# Run a specific run (1-6) for a subject
go run examples/retinotopy/main.go -s 0 -r 1
```

#### Stroop Task
A classic Stroop interference task defaulting to 1920x1080 resolution.
```bash
go run examples/stroop_task/main.go
```

#### Mental Logic Card Game
An experiment testing mental logic and inference through card presentation and manipulation.
```bash
go run examples/card_game/main.go
```

*Note: Most examples support a `-d` flag for windowed development mode.*

## License

This project is licensed under the GNU Public License v3 - see the [LICENSE](LICENSE.txt) file for details.

Christophe Pallier, 2026

