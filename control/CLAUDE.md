// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

# control package

Top-level experiment orchestration package. Every experiment imports only `control` for day-to-day work; other packages are accessed via `Experiment` fields.

## Experiment lifecycle

`NewExperimentFromFlags` handles flag parsing (`-w` windowed mode, `-d N` display index, `-s` subject ID), SDL/TTF init, window creation, audio device, font, and data file in one call. It takes optional trailing `InfoField`s, appended to the session-setup dialog it opens when `-s` is absent and returned in `exp.Info` — that is how an experiment asks for one more setting (which protocol to run, which response box) without building its own dialog; `exp.Info` is nil whenever the dialog is skipped. Use the lower-level `NewExperiment(...) + Initialize()` only when you need non-standard initialization order.

**Real-time priority is requested by `Initialize()`, so both paths get it.** It
used to be requested only inside `NewExperimentFromFlags`, which meant the choice
of constructor silently decided the scheduling policy — `tests/Timing-Tests` uses
the plain one and ran the whole 2026 campaign at `SCHED_OTHER` as a result. The
decision now travels on `Experiment.RealTimePriority`; the flags only set it. Set
it to 0 before `Initialize()` to decline (see `docs/SettingPriorityUnderLinux.md`).

When `-s` is **absent** (e.g. the binary was launched by double-clicking its icon), `NewExperimentFromFlags` opens an automatic setup dialog (subject code + display + fullscreen + results folder) via `GetParticipantInfo` before `Initialize()`. It is skipped when `-s N` is passed, under `-headless`, or if the program already called `GetParticipantInfo` itself (guarded by the package-level `participantInfoCollected`). Cancelling exits via `os.Exit(0)`. Programs that collect custom participant fields still use `GetParticipantInfo` + `NewExperiment` and never reach this path.

**Browser (GOOS=js):** there is no command line, so `platformPrepareFlags` (`platform_js.go`) synthesizes `os.Args` from the page URL's query string before `flag.Parse` — `?s=3&w` behaves like `-s 3 -w`, and experiment-specific flags work too. Unknown query keys are ignored with a console note. The setup dialog never opens (`platformInteractiveSetup` returns false); without `s` the subject ID defaults to 0 like `-headless`. Audio works: `platformInitAudio` opens the default device like desktop (the browser keeps the AudioContext suspended until the first click/keypress, which SDL auto-resumes — the "press SPACE" screen satisfies this); if the device cannot be opened the experiment continues with a silent no-op `AudioManager` instead of crashing. See `docs/WASM.md` for the full browser story.

`exp.Run` wraps the SDL event loop. User code panicked with `exitPanic` is recovered there; callers never see it directly. Return `control.EndLoop` (or `sdl.EndLoop`) to exit cleanly.

## Convenience methods

- `exp.ShowFrames(stim, n)` — holds the stimulus for exactly `n` display frames, returning the first flip's timestamp (the onset). Redraws every frame; that is mandatory, not an optimisation (see `apparatus/CLAUDE.md`, "There is no 'wait for n VSYNCs' call").
- `exp.FittedTextBox(text)` — the layout behind both: a centered `TextBox` wrapped to `exp.DrawArea()` and rendered at the largest point size (never above `DefaultFontSize`) at which the whole block fits, preferring a size that leaves the author's own line breaks intact. Never wrap a text screen at a fixed fraction of the window: that turns a pixel width into a column count that varies with the display, so hand-wrapped text is re-broken into orphan lines on a narrower screen, and nothing checks the height at all. The fitted font is owned by the experiment and closed by `End()`.
- `exp.DrawArea()` — the logical drawing space (`Screen.LogicalSize`, else `Screen.Width/Height`). Layout code wants this, not the window size.
- `exp.HandleEvents()` — Returns `(lastKey, lastMouseButton, error)`. Prefer `PollEvents` for new code.

## Calibrating a gaze tracker (eyetracker_calib.go)

```go
err := exp.CalibrateTracker(tracker, eyetracker.CalibrationOptions{Points: 9})
```

One call for both shapes of vendor API, so an experiment need not know which
make it has:

- A tracker that runs its own setup routine (an EyeLink, through pylink) is
  simply asked to. Its Host PC's calibration screen is better tested than
  anything we can draw.
- A tracker implementing `eyetracker.StepwiseCalibrator` (a Tobii, whose SDK
  draws *nothing*) is driven target by target from here: each target goes up
  with `ShowTS`, is held for `opts.DwellMs` (default 700), and only then is the
  tracker told to sample it. That ordering is the point — the target onset is on
  goxpyriment's own flip clock, in its own window, on one machine.

It **blocks**, for seconds per target, and it draws — so it must run on the
`exp.Run` goroutine like every other SDL call, and never inside a frame loop.
Esc ends the run as it does everywhere else (see `Wait`); the tracker is taken
out of calibration mode on the way out regardless, because one left in it will
not record.

The result, including any target that yielded no usable data, goes into the
run's `-info.txt`. A monocular partial success is logged as a warning: a run
quietly recorded on one eye and later analysed as binocular is the failure that
warning exists to prevent. See `eyetracker/CLAUDE.md`.

## Mouse cursor: hidden by default

`Initialize` calls `HideCursor` once the screen exists (`apparatus.NewScreen`
deliberately *shows* the cursor — it also installs a cursor shape, which SDL
does not supply on the KMS/DRM backend — so the order matters). A pointer left
on a stimulus surface is an unintended distractor.

Mouse-driven paradigms opt back in with `exp.ShowCursor()` immediately after
creation: `examples/Mouse-tracking`, `Multiple-Object-Tracking`, `LoT-geometry`,
`Finger-Tracking`, `Rush-Hour`. Failing to hide is cosmetic, so it warns rather
than aborting the session.

`GetParticipantInfo` is independent of all this: its dialog is clicked, so it
shows the cursor for the lifetime of its window and restores the previous state
on exit — correct whether it runs before `Initialize` (the usual case) or
mid-session between blocks.

## AudioManager

`exp.Audio` coordinates playback so callers don't touch SDL audio streams directly.

Audio stimuli still need `sound.PreloadDevice(exp.AudioDevice)` before first play.

## Participant info dialog (GetParticipantInfo)

```go
info, err := control.GetParticipantInfo("Session Setup", control.StandardFields)
if errors.Is(err, control.ErrCancelled) { return }
exp.Info = info
```

`GetParticipantInfo` opens its own SDL window, loads/saves `~/.cache/goxpyriment/last_session.json` (subject_id is always reset to empty on load), and returns a `map[string]string`. It shuts down SDL internally; `exp.Initialize()` re-initialises cleanly afterwards. Call it **before** `exp.Initialize()`.

## Event queue for hand-rolled input loops (defaults.go)

Enough of the SDL event API is re-exported to build a custom input loop — e.g. a
text-input/typing loop with per-keystroke hardware timestamps and a blinking
cursor — without importing `go-sdl3`:

- **Types:** `Event`, `EventType`, `KeyboardEvent`, `TextInputEvent`
- **Event constants:** `EVENT_QUIT`, `EVENT_KEY_DOWN`, `EVENT_KEY_UP`, `EVENT_TEXT_INPUT`
- **Functions:** `PollEvent(*Event) bool`, `TicksNS() uint64`

Start/stop IME text input via `exp.Screen.Window.StartTextInput()` /
`StopTextInput()`; read events with `ev.KeyboardEvent()` / `ev.TextInputEvent()`;
their `Timestamp` fields share the `TicksNS()` / `Screen.FlipTS()` reference
frame. `examples/Typing-Speed` is a complete worked example. Prefer the
`Keyboard` helpers (`GetKeyEventTS`, …) for ordinary key responses — reach for
the raw queue only when you need text input or bespoke event handling.


## Audio latency tuning

Call **before** `NewExperiment` or `Initialize`:

```go
control.SetAudioSampleFrames(256) // lower = less latency
// The default is the DRIVER's, not goxpyriment's (measured 1024 frames =
// 23.2 ms on PipeWire @ 44.1 kHz). Read it back with exp.AudioDevice.Format()
// or snd.OutputLatency() — do not assume a value.
```

## Key conventions for this package

- Never import `go-sdl3` directly in experiment code; use the re-exports in `defaults.go`.
- `exp.End()` must always be deferred to clean up SDL, TTF, and the audio device.
- `QuitRequested` in `EventState` is sticky — once true it stays true for that `PollEvents` result. Check it immediately.
- `exitPanic` is an internal sentinel; never compare against it directly. Use `IsEndLoop`.
- **Single-thread contract:** all SDL/stimulus/event work must run on the goroutine that calls `exp.Run` (which pins itself with `runtime.LockOSThread`). Never call `exp.Screen.*`, `stim.Draw`, `exp.Show`, or input polling from a goroutine you spawn — SDL3 video is single-threaded and will crash or corrupt state otherwise. Background goroutines (e.g. the audio manager, the signal handler) deliberately avoid SDL video calls.
