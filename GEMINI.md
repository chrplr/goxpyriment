# GEMINI.md - goxpyriment

## Project Overview
`goxpyriment` is a Go-based framework designed for creating behavioral and psychological experiments. It provides a high-level API for managing experimental designs, stimuli presentation, and data collection, leveraging SDL3 for cross-platform hardware-accelerated rendering and event handling.

### Key Technologies
- **Language:** Go (1.25+)
- **Graphics & I/O:** SDL3 (via `github.com/Zyko0/go-sdl3`)
- **Bindings:** `purego` for C-interop without CGO requirements in many cases.

## Architecture

### Core Modules
- **`control/`**: Contains the `Experiment` manager, which handles the lifecycle of an experiment (initialization, the main run loop, and cleanup).
- **`design/`**: Provides structures for experimental logic:
  - `Experiment`: Top-level structure holding blocks and factors.
  - `Block`: A collection of trials.
  - `Trial`: The basic unit of an experiment, containing factors and associated stimuli.
- **`io/`**: Manages low-level system interfaces:
  - `Screen`: Handles the SDL window and renderer.
  - `Keyboard`/`Mouse`: Input event handling.
  - `DataFile`: Logging experimental results to `.xpd` files.
- **`stimuli/`**: A library of reusable components for presentation:
  - Visual: `TextLine`, `TextBox`, `Rectangle`, `Circle`, `Picture`, `FixCross`, `GaborPatch`, etc.
  - Audio: `Sound`, `Tone`.
- **`misc/`**: Utility functions for high-precision timing (`Wait`, `GetTime`) and geometric calculations.


### examples

Examples are provided in the examples folder which has its own go.mod (we use the "sub-module setup" here). This is to avoi go get github.com/chrplr.com/goxpyriment to download the full repository.

## Building and Running

### Prerequisites
- Go 1.25 or higher.
- SDL3 libraries must be available on the system.

### Key Commands
- **Run the main demo:**
  ```bash
  go run main.go
  ```
- **Run specific examples:**
  ```bash
  go run examples/parity_decision/main.go
  go run examples/stroop_task/main.go
  ```
- **Build the project:**
  ```bash
  go build -o goxpyriment .
  ```

## Development Conventions

### Experiment Lifecycle
All experiments should follow this general pattern:
1.  **Creation:** `exp := control.NewExperiment("Name", width, height, fullscreen)`
2.  **Initialization:** `err := exp.Initialize()` (handles SDL and subsystem setup).
3.  **Setup:** Define blocks, trials, and stimuli.
4.  **Execution:** `err := exp.Run(func() error { ... })`
5.  **Cleanup:** `defer exp.End()`

### Stimuli Presentation
Visual stimuli typically implement a `Present(screen *io.Screen, clear, update bool) error` method.
- `clear`: If true, the screen is cleared with the background color before drawing.
- `update`: If true, `SDL_RenderPresent` is called after drawing.

### Data Logging
Use `exp.Data.Add([]interface{}{...})` to log trial data. Headers should be defined early using `exp.Data.AddVariableNames([]string{...})`. Data is saved automatically when `exp.End()` is called.

### Coding Style
- Follow standard Go idioms and `gofmt`.
- Prefer hardware-accelerated rendering via the `io.Screen` renderer.
- Use `misc.Wait()` and `misc.GetTime()` for experiment-critical timing to ensure consistency across platforms.
