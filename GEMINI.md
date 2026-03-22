# GEMINI.md - goxpyriment

## Project Overview
`goxpyriment` is a Go-based framework designed for creating behavioral and psychological experiments. It provides a high-level API for managing experimental designs, stimuli presentation, and data collection, leveraging SDL3 for cross-platform hardware-accelerated rendering and event handling.

### Key Technologies
- **Language:** Go (1.25+)
- **Graphics & I/O:** SDL3 (via `github.com/Zyko0/go-sdl3`)
- **Media Decoding:** FFmpeg (via `github.com/asticode/go-astiav`)
- **Bindings:** `purego` for C-interop without CGO requirements in many cases (though `go-astiav` requires CGO).

## Architecture

### Concurrency & Execution Model
`goxpyriment` uses a **multi-threaded architecture** to ensure cross-platform responsiveness and future-proof WebAssembly (Wasm) support:
- **Main Thread (UI/Event Loop)**: Dedicated to pumping SDL events and executing rendering tasks. It manages the `sdl.RunLoop` dispatcher.
- **Logic Goroutine (Experiment Thread)**: Where the user's experiment logic (passed to `exp.Run`) executes. It can block (e.g., during waits or input collection) without freezing the UI.
- **Synchronization**: Thread-safe communication is handled internally via a task queue for rendering and a "sticky event" mechanism for inputs (keys, mouse buttons).

### Core Modules
- **`control/`**: Contains the `Experiment` manager (facade), task dispatcher, and lifecycle management.
- **`design/`**: Provides structures for experimental logic:
  - `Experiment`: Top-level structure holding blocks and factors.
  - `Block`: A collection of trials.
  - `Trial`: The basic unit of an experiment, containing factors and associated stimuli.
- **`io/`**: Manages low-level system interfaces:
  - `Screen`: Handles the SDL window and renderer (thread-safe via `Experiment`).
  - `Keyboard`/`Mouse`: Input event handling (thread-safe, non-destructive polling).
  - `DataFile`: Logging experimental results to `.xpd` files.
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
- **`exp.Show(stimulus)`**: Presents a stimulus, clearing the screen and updating the display. Thread-safe.
- **`exp.Wait(ms)`**: Blocks the logic thread for `ms` milliseconds while keeping the OS responsive. Aborts instantly on `ESC`.
- **`exp.Blank(ms)`**: Clears the screen and waits. Equivalent to `exp.Screen.Clear()`, `exp.Screen.Update()`, `exp.Wait(ms)`.

### Data Logging
Use `exp.Data.Add(...)` to log trial data. Headers should be defined early using `exp.Data.AddVariableNames(...)`.

### Coding Style
- Follow standard Go idioms.
- **Avoid blocking the main thread**: Never perform long-running operations outside of `exp.Run` after initialization.
- **Facade Methods**: Prefer `Experiment` methods (`exp.Show`, `exp.Wait`) over direct calls to `stimuli` or `clock` for better thread safety and automatic error handling.
