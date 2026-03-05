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

A simple "Hello World" experiment that waits for a key press:

```go
package main

import (
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"log"
)

func main() {
	// 1. Initialize the experiment
	exp := control.NewExperiment("Hello World", 800, 600, false)
	if err := exp.Initialize(); err != nil {
		log.Fatal(err)
	}
	defer exp.End()

	// 2. Prepare a stimulus
	text := stimuli.NewTextLine("Hello, GoXpyriment!", 0, 0, control.DefaultTextColor)

	// 3. Run the logic
	exp.Run(func() error {
		// Present the text and wait for any key
		text.Present(exp.Screen, true, true)
		exp.Keyboard.Wait()
		return nil
	})
}
```

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

