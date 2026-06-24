# How `control/experiment.go` works

`control/experiment.go` defines the **`Experiment` facade** — the single object an experiment program interacts with. It owns every subsystem (screen, input, audio, data, design) and exposes thin convenience methods over them. Here's how it works, following the lifecycle:

## 1. The `Experiment` struct (the facade)

`Experiment` (lines 70–108) bundles pointers to the focused subsystem packages:

- `Screen` (`apparatus.Screen`) — window + renderer
- `Keyboard` / `Mouse` (`apparatus`) — input
- `Audio` / `AudioDevice` — sound
- `Data` (`results.DataFile`) — CSV logging
- `Design` (`design.Experiment`) — trial/block structure

Plus config (`BackgroundColor`, `Fullscreen`, `ScreenNumber`, fonts) and internal state: `event EventState`, `quitFlag` (atomic, set by the signal handler), and `endOnce` (ensures the data footer is written exactly once).

The doc comment (lines 47–54) states the design intent: **most methods are thin delegations** to subsystem packages, keeping the user API simple.

## 2. Construction & initialization

- **`NewExperiment(...)`** (148) — just fills the struct; no SDL yet.
- **`NewExperimentFromFlags(...)`** (177) — the standard entry point. Parses `-w` / `-d N` / `-s N`, builds the struct, then calls `Initialize()` and `log.Fatal`s on failure.
- **`Initialize()`** (476) does the real setup, in order:
  1. Loads the embedded SDL/TTF/IMG binaries (reusing loaders cached by `GetParticipantInfo` to avoid a double-load crash on macOS).
  2. `sdl.Init` + `ttf.Init`, audio device, then `apparatus.NewScreen`.
  3. Wires `Keyboard`/`Mouse` with **poll closures** (lines 522–549) — these call back into `e.PollEvents`, so all input flows through the experiment's single event drain.
  4. Loads the default Inconsolata font, creates the `DataFile`, writes system/display/participant metadata into its header.
  5. Spawns a **signal handler goroutine** (595) for Ctrl-C/SIGTERM: it sets `quitFlag`, calls `finalizeData()`, and force-exits after 3s. It deliberately never calls SDL teardown (concurrent CGo from two threads → SIGSEGV).

## 3. The run loop

**`Run(logic func() error)`** (936) is the heart:

- `runtime.LockOSThread()` pins the goroutine — SDL video/render/events must all stay on one thread (the **single-thread contract**).
- It calls `sdl.RunLoop`, and each frame: recovers from `exitPanic`, calls `pumpFrame()`, then your `logic()` callback.
- Return `control.EndLoop` from `logic` to exit cleanly.

**`pumpFrame()`** (428) checks `quitFlag`, pumps OS events, and checks for QUIT/ESC — **without draining the queue**, so keyboard/mouse events keep their original hardware timestamps for `GetKeyEventTS`.

## 4. Event handling

**`PollEvents(handler)`** (630) is the one place SDL events are drained. It resets transient state, loops over events, records the first key/mouse/keyup of the cycle plus their nanosecond timestamps, and sets the **sticky** `QuitRequested` on ESC or window-close. The `Keyboard`/`Mouse` closures and `WaitAnyEventTS` all sit on top of this.

## 5. Convenience methods (the ergonomic layer)

These compose the common patterns so experiment code stays short:

- `Show` / `ShowTS` (208, 222) — clear+draw+flip; `ShowTS` returns the VSYNC flip timestamp for precise RT.
- `ShowTimed`, `ShowAndGetRT`, `ShowInstructions`, `ShowEndMessage`, `Blank`, `Wait` — each replaces a 2–3 line idiom (documented inline).
- `WaitAnyEventTS` (252) — blocks for any device event with a hardware timestamp.

The `exitPanic` sentinel (136) lets these methods abort to `Run`'s recover on ESC/quit without threading an error through every line of user code.

## 6. Teardown

**`End()`** (877) — deferred by the caller. Calls `finalizeData()` (writes the end-time footer + saves, guarded by `endOnce`), closes font/screen/audio/microphone, then `ttf.Quit`/`sdl.Quit`, and unloads the embedded library blobs in reverse order. `Fatal()` (910) is the post-init replacement for `log.Fatalf` that cleans up first.

The rest of the file is **delegation wrappers** — `AddBlock`, `ShuffleBlocks`, `AddBWSFactor`, etc. forward straight to `e.Design`; `SetVSync`, `Flip`, `LoadFont` forward to `e.Screen`. That's the facade pattern the doc comment describes: a simple surface, with the real logic living in each subsystem package.
