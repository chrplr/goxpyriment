# How `control/experiment.go` works

`control/experiment.go` defines the **`Experiment` facade** — the single object an experiment program interacts with. It owns every subsystem (screen, input, audio, data, design) and exposes thin convenience methods over them. Here's how it works, following the lifecycle:

## 1. The `Experiment` struct (the facade)

`Experiment` (lines 78–169) bundles pointers to the focused subsystem packages:

- `Screen` (`apparatus.Screen`) — window + renderer
- `Keyboard` / `Mouse` (`apparatus`) — input
- `Audio` / `AudioDevice` — sound
- `Data` (`results.DataFile`) — CSV logging
- `Design` (`design.Experiment`) — trial/block structure

Plus config (`BackgroundColor`, `Fullscreen`, `ScreenNumber`, fonts) and internal state: `event EventState`, `quitFlag` (atomic, set by the signal handler), `endOnce` (ensures the data footer is written exactly once), and `endDone` (set once `End` has finished, so the signal handler stands down).

The doc comment (lines 55–62) states the design intent: **most methods are thin delegations** to subsystem packages, keeping the user API simple.

## 2. Construction & initialization

- **`NewExperiment(...)`** (223) — just fills the struct; no SDL yet.
- **`NewExperimentFromFlags(...)`** (320) — the standard entry point. Parses `-w` / `-d N` / `-s ID`, builds the struct, then calls `Initialize()` and `log.Fatal`s on failure.
- **`Initialize()`** (1132) does the real setup, in order:
  1. Loads the embedded SDL/TTF/IMG binaries (reusing loaders cached by `GetParticipantInfo` to avoid a double-load crash on macOS).
  2. `sdl.Init` + `ttf.Init`, audio device, then `apparatus.NewScreen`.
  3. Wires `Keyboard`/`Mouse` with **poll closures** (lines 1247–1291) — these call back into `e.PollEvents`, so all input flows through the experiment's single event drain.
  4. Loads the default Inconsolata font, creates the `DataFile`, writes system/display/participant metadata into the companion `-info.txt`.
  5. Spawns a **signal handler goroutine** (1381) for Ctrl-C/SIGTERM: it only sets `quitFlag` and waits 3 s, during which the main goroutine sees the quit, unwinds and runs `End`, which saves the data. Only if `End` has not finished by then (`endDone`) — the main goroutine is blocked outside `Run` — does it call `finalizeData()`, drive the marker lines low and exit. It deliberately never calls SDL teardown (SDL calls from two OS threads → SIGSEGV).

## 3. The run loop

**`Run(logic func() error)`** (1804) is the heart:

- `runtime.LockOSThread()` pins the goroutine — SDL video/render/events must all stay on one thread (the **single-thread contract**).
- It calls `sdl.RunLoop`, and each frame: recovers from `exitPanic`, calls `pumpFrame()`, then your `logic()` callback.
- Return `control.EndLoop` from `logic` to exit cleanly.

**`pumpFrame()`** (1067) first checks `quitPending()` (1056) — `quitFlag` or the sticky `QuitRequested` — then pumps OS events, and checks for QUIT/ESC — **without draining the queue**, so keyboard/mouse events keep their original hardware timestamps for `GetKeyEventTS`.

## 4. Event handling

**`PollEvents(handler)`** (1433) is the one place SDL events are drained. It resets transient state, loops over events, records the first key/mouse/keyup of the cycle plus their nanosecond timestamps, and sets the **sticky** `QuitRequested` on ESC, window-close or a caught Ctrl-C/SIGTERM. The cycle stops right after the first key or mouse-button event, leaving the rest queued for the next call, so a second press is never consumed and dropped. The `Keyboard`/`Mouse` closures and `WaitAnyEventTS` all sit on top of this.

## 5. Convenience methods (the ergonomic layer)

These compose the common patterns so experiment code stays short:

- `Show` / `ShowTS` (499, 524) — clear+draw+flip; `ShowTS` returns the VSYNC flip timestamp for precise RT.
- `ShowTimed`, `ShowAndGetRT`, `ShowInstructions`, `ShowEndMessage`, `Blank`, `Wait` — each replaces a 2–3 line idiom (documented inline).
- `WaitAnyEventTS` (647) — blocks for any device event with a hardware timestamp.

The `exitPanic` sentinel (211) lets these methods abort to `Run`'s recover on ESC/quit without threading an error through every line of user code. Because `QuitRequested` is sticky, `Wait`, `Blank`, `Show`, `ShowTS`, `ShowTimed`, `ShowFrames` and `BlankFrames` all consult `quitPending()`: after ESC, a window close or Ctrl-C the next one of them ends the run, even when the experiment discarded the `EndLoop` error a key wait returned.

## 6. Teardown

**`End()`** (1730) — deferred by the caller. Calls `finalizeData()` (writes the end-time footer + saves, guarded by `endOnce`), closes font/screen/audio/microphone, then `ttf.Quit`/`sdl.Quit`, and unloads the embedded library blobs in reverse order. `Fatal()` (1778) is the post-init replacement for `log.Fatalf` that cleans up first.

The rest of the file is **delegation wrappers** — `AddBlock`, `ShuffleBlocks`, `AddBWSFactor`, etc. forward straight to `e.Design`; `SetVSync`, `Flip`, `LoadFont` forward to `e.Screen`. That's the facade pattern the doc comment describes: a simple surface, with the real logic living in each subsystem package.
