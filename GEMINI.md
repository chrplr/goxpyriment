# GEMINI.md - goxpyriment

## Project Overview
`goxpyriment` is a Go-based framework designed for creating behavioral and psychological experiments. It provides a high-level API for managing experimental designs, stimuli presentation, and data collection, leveraging SDL3 for cross-platform hardware-accelerated rendering and event handling.

### Key Technologies
- **Language:** Go (1.25+)
- **Graphics & I/O:** SDL3 (via `github.com/Zyko0/go-sdl3`)
- **Media Decoding:** FFmpeg (via `github.com/asticode/go-astiav`)
- **Bindings:** `purego` for C-interop without CGO requirements in many cases (though `go-astiav` requires CGO).

## Architecture

### Execution Model
`goxpyriment` uses a **single-threaded execution model** for the core experiment logic, aligned with SDL3's requirements:
- **Main Thread (sdl.RunLoop)**: The user's experiment logic (passed to `exp.Run`) executes directly on the main thread inside SDL's run loop. This ensures that all rendering and event-polling calls are thread-safe and performant.
- **Graceful Abort**: The logic thread can be "aborted" at any time (e.g., when the participant presses ESC or closes the window). This is handled via a panic/recover mechanism within `exp.Run`, allowing for a clean exit and data saving without requiring manual error checks on every line of stimulus code.

### Core Modules
- **`control/`**: Contains the `Experiment` manager (facade), lifecycle management, and high-level orchestration helpers.
- **`design/`**: Provides structures for experimental logic:
  - `Experiment`: Top-level structure holding blocks and factors.
  - `Block`: A collection of trials.
  - `Trial`: The basic unit of an experiment, containing factors and associated stimuli.
- **`io/`**: Manages low-level system interfaces:
  - `Screen`: Handles the SDL window and renderer.
  - `Keyboard`/`Mouse`: Input event handling (non-destructive polling).
  - `DataFile`: Logging experimental results to `.csv` files.
- **`stimuli/`**: A library of reusable components for presentation:
  - Visual: `TextLine`, `TextBox`, `Rectangle`, `Circle`, `Picture`, `FixCross`, `GaborPatch`, etc.
  - Audio: `Sound`, `Tone`.
- **`clock/`**: High-precision timing helpers (`Wait`, `GetTime`, `Clock`). Use `exp.Wait()` inside `Run` for responsive waits.
- **`geometry/`**: Geometric helpers (distances, polar/Cartesian transforms).

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
All experiments follow this pattern:
1.  **Creation:** `exp := control.NewExperimentFromFlags(...)` or `NewExperiment(...)`.
2.  **Initialization:** `err := exp.Initialize()`.
3.  **Setup:** Define blocks, trials, and stimuli.
4.  **Execution:** `err := exp.Run(func() error { ... })`.
    - **Simplified Error Handling**: Core methods (`exp.Show`, `exp.Wait`, `exp.Blank`, `exp.ShowInstructions`) automatically handle experiment aborts (on `ESC` or window close). You do **not** need to check errors on every line to ensure a graceful exit.
5.  **Cleanup:** `defer exp.End()`.

### Stimuli Presentation & Timing
- **`exp.Show(stimulus)`**: Presents a stimulus, clearing the screen and updating the display.
- **`exp.Wait(ms)`**: Blocks execution for `ms` milliseconds while keeping the OS responsive (pumping events). Aborts instantly on `ESC`.
- **`exp.Blank(ms)`**: Clears the screen and waits. Equivalent to `exp.Screen.Clear()`, `exp.Screen.Update()`, `exp.Wait(ms)`.

### Data Logging
Use `exp.Data.Add(...)` to log trial data. Headers should be defined early using `exp.Data.AddVariableNames(...)`.

### Coding Style
- Follow standard Go idioms.
- **Run everything inside `exp.Run`**: Never perform rendering or event operations outside the `exp.Run` callback after initialization.
- **Facade Methods**: Prefer `Experiment` methods (`exp.Show`, `exp.Wait`) over direct calls to `stimuli` or `clock` for better automatic error handling.
