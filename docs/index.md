# goxpyriment

`goxpyriment` is a high-level Go framework for building behavioral and psychological experiments with precise, VSYNC-locked stimulus timing.

* [GitHub repository](https://github.com/chrplr/goxpyriment)
* Report bugs and suggestions at <https://github.com/chrplr/goxpyriment/issues>

---

## Why goxpyriment?

1. **Zero-dependency deployment.** A finished experiment compiles to a single binary — an `.exe` on Windows, an AppImage on Linux, a `.app` on macOS. No Python, no conda, no DLL hell on lab computers.
2. **Timing precision.** The stimulus loop runs VSYNC-locked with GC pauses disabled, giving sub-millisecond frame jitter on typical hardware.
3. **AI-friendly API.** The linear, consistent API is well suited to "vibe-coding" — describe your paradigm in plain language to Claude, Gemini, or ChatGPT and the generated code is usually 90 % ready to run immediately.

---

## Documentation

| Document | Read online | Download |
|---|---|---|
| Getting Started | [HTML](GettingStarted.md) | [PDF](GettingStarted.pdf) |
| User Manual | [HTML](UserManual.md) | [PDF](UserManual.pdf) |
| API Reference | [HTML](API.md) | [PDF](API.pdf) |

---

## Quick Start

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

    hello := stimuli.NewTextBox("Hello, World!", 600, control.FPoint{}, control.White)

    err := exp.Run(func() error {
        exp.Show(hello)
        exp.Keyboard.Wait()
        return control.EndLoop
    })
    if err != nil && !control.IsEndLoop(err) {
        log.Fatalf("experiment error: %v", err)
    }
}
```

```bash
go run . -w        # windowed mode
go run . -w -s 1   # windowed, subject ID = 1
```

---

## Installation

Download and install Go from <https://go.dev>, then:

```bash
git clone https://github.com/chrplr/goxpyriment.git
cd goxpyriment
go build ./...
```

Run any example:

```bash
go run examples/Stroop_task/main.go -w -s 1
```

---

## Ready-to-run Demos

Pre-built binaries for Windows, macOS, and Linux are available on the
[Releases page](https://github.com/chrplr/goxpyriment/releases).

---

Christophe Pallier, 2026 — GNU GPL v3
