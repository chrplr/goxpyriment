// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

# control package

Top-level experiment orchestration package. Every experiment imports only `control` for day-to-day work; other packages are accessed via `Experiment` fields.

## Experiment lifecycle

```go
exp := control.NewExperimentFromFlags("My Experiment", control.Black, control.White, 32)
defer exp.End()
exp.Run(func() error {
    // trial loop body — return control.EndLoop to exit, nil to continue
})
```

`NewExperimentFromFlags` handles flag parsing (`-w` windowed mode, `-d N` display index, `-s` subject ID), SDL/TTF init, window creation, audio device, font, and data file in one call. Use the lower-level `NewExperiment(...) + Initialize()` only when you need non-standard initialization order.

**Real-time priority is requested by `Initialize()`, so both paths get it.** It
used to be requested only inside `NewExperimentFromFlags`, which meant the choice
of constructor silently decided the scheduling policy — `tests/Timing-Tests` uses
the plain one and ran the whole 2026 campaign at `SCHED_OTHER` as a result. The
decision now travels on `Experiment.RealTimePriority`; the flags only set it. Set
it to 0 before `Initialize()` to decline (see `docs/SettingPriorityUnderLinux.md`).

When `-s` is **absent** (e.g. the binary was launched by double-clicking its icon), `NewExperimentFromFlags` opens an automatic setup dialog (subject code + display + fullscreen + results folder) via `GetParticipantInfo` before `Initialize()`. It is skipped when `-s N` is passed, under `-headless`, or if the program already called `GetParticipantInfo` itself (guarded by the package-level `participantInfoCollected`). Cancelling exits via `os.Exit(0)`. Programs that collect custom participant fields still use `GetParticipantInfo` + `NewExperiment` and never reach this path.

**Browser (GOOS=js):** there is no command line, so `platformPrepareFlags` (`platform_js.go`) synthesizes `os.Args` from the page URL's query string before `flag.Parse` — `?s=3&w` behaves like `-s 3 -w`, and experiment-specific flags work too. Unknown query keys are ignored with a console note. The setup dialog never opens (`platformInteractiveSetup` returns false); without `s` the subject ID defaults to 0 like `-headless`. Audio works: `platformInitAudio` opens the default device like desktop (the browser keeps the AudioContext suspended until the first click/keypress, which SDL auto-resumes — the "press SPACE" screen satisfies this); if the device cannot be opened the experiment continues with a silent no-op `AudioManager` instead of crashing. See `docs/WASM.md` for the full browser story.

`exp.Run` wraps the SDL event loop. User code panicked with `exitPanic` is recovered there; callers never see it directly. Return `control.EndLoop` (or `sdl.EndLoop`) to exit cleanly.

## Experiment fields

| Field | Type | Description |
|---|---|---|
| `Screen` | `*apparatus.Screen` | Window + renderer |
| `Keyboard` | `*apparatus.Keyboard` | Blocking/non-blocking key input |
| `Mouse` | `*apparatus.Mouse` | Mouse button + position input |
| `AudioDevice` | `sdl.AudioDeviceID` | Passed to `Sound.PreloadDevice` |
| `Audio` | `*AudioManager` | High-level audio playback |
| `Data` | `*results.DataFile` | `.csv` experiment data file |
| `Design` | `*design.Experiment` | Trial/block structure |
| `Info` | `map[string]string` | Participant metadata (from `GetParticipantInfo`) |
| `SubjectID` | `int` | Set by `-s` flag or `GetParticipantInfo` |
| `DefaultFont` | `*ttf.Font` | Passed to stimuli that omit an explicit font |
| `DefaultFontSize` | `int` | Font size used at init |
| `CursorVisible` | `bool` | Mouse pointer over the experiment window. **Defaults to false — `Initialize` hides the cursor.** Set true before `Initialize`, or call `exp.ShowCursor()` after, for mouse-driven paradigms. `ShowCursor`/`HideCursor` keep it in sync |
| `BackgroundColor` | `sdl.Color` | Screen background |
| `ForegroundColor` | `sdl.Color` | Default text color |
| `OutputDirectory` | `string` | Where `.csv` files are written |
| `RealTimePriority` | `int` | SCHED_FIFO priority `Initialize()` requests; `DefaultRealTimePriority` (50), or 0 to decline. `NewExperimentFromFlags` sets it from `-no-realtime` / `-realtime-priority` |

## Convenience methods

- `exp.Show(stim)` — `Clear()` + `Draw()` + `Update()` in one call. Use for single-stimulus frames.
- `exp.ShowTS(stim)` — Same as `Show` but returns the SDL3 nanosecond flip timestamp for hardware-precise RT.
- `exp.ShowTimed(stim, durationMs)` — `Show(stim)` + `Wait(durationMs)`. For fixation crosses, cues, and passive stimulus viewing.
- `exp.ShowFrames(stim, n)` — holds the stimulus for exactly `n` display frames, returning the first flip's timestamp (the onset). Redraws every frame; that is mandatory, not an optimisation (see `apparatus/CLAUDE.md`, "There is no 'wait for n VSYNCs' call").
- `exp.BlankFrames(n)` — frame-locked `Blank`: clears and holds blank for `n` frames, returning the first flip's timestamp (the previous stimulus's offset).
- `exp.ShowAndGetRT(stim, keys, timeoutMs)` — Clears stale keyboard events, shows stim with `ShowTS`, waits for a key with `GetKeyEventTS`, returns `(key, rtMs, error)` with hardware-precise RT. `timeoutMs = -1` for no timeout; returns `(0, 0, nil)` on timeout.
- `exp.ShowEndMessage(message)` — Renders a centered completion message and waits for any key. For end-of-experiment screens.
- `exp.ShowInstructions(text)` — Renders centered text, waits for spacebar.
- `exp.Blank(ms)` — Clears screen, flips, sleeps `ms` milliseconds.
- `exp.PollEvents(handler)` — Drains SDL queue; `handler` may be nil. Returns `EventState`.
- `exp.HandleEvents()` — Returns `(lastKey, lastMouseButton, error)`. Prefer `PollEvents` for new code.

## EventState

Returned by `PollEvents`. Summarises the current SDL queue drain:

```go
type EventState struct {
    LastKey            sdl.Keycode
    LastMouseButton    uint32
    LastKeyTimestamp   uint64
    LastMouseTimestamp uint64
    QuitRequested      bool  // sticky — stays true once ESC or window-close seen
}
```

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

| Method | Behaviour |
|---|---|
| `PlaySync(snd)` | Blocks until playback complete |
| `PlayAsync(snd)` | Starts playback; goroutine managed internally |
| `PlayMemorySync/Async([]byte)` | One-shot from raw bytes |
| `PlayBuzzer()` / `PlayCorrect()` | Embedded feedback sounds |
| `Shutdown()` | Called by `exp.End()` automatically |

Audio stimuli still need `sound.PreloadDevice(exp.AudioDevice)` before first play.

## Participant info dialog (GetParticipantInfo)

```go
info, err := control.GetParticipantInfo("Session Setup", control.StandardFields)
if errors.Is(err, control.ErrCancelled) { return }
exp.Info = info
```

`GetParticipantInfo` opens its own SDL window, loads/saves `~/.cache/goxpyriment/last_session.json` (subject_id is always reset to empty on load), and returns a `map[string]string`. It shuts down SDL internally; `exp.Initialize()` re-initialises cleanly afterwards. Call it **before** `exp.Initialize()`.

### Predefined field sets

| Constant | Fields |
|---|---|
| `ParticipantFields` | subject_id, age, gender, handedness |
| `MonitorFields` | screen width/cm, viewing distance/cm, refresh rate |
| `FullscreenField` | fullscreen checkbox |
| `StandardFields` | ParticipantFields + MonitorFields |

Custom fields use `InfoField{Name, Label, Default, Type}` where `Type` is `FieldText` or `FieldCheckbox`.

## EventLog

Optional structured session metadata. `exp.CollectEventLog()` gathers SDL/OS/display/audio info:

```go
log := exp.CollectEventLog()
// log.SDLVersion, log.Platform, log.Hostname, log.VideoDriver, log.DisplayMode …
```

## SDL type re-exports (defaults.go)

Import only `control` — do not import `go-sdl3` directly in experiment code.

### Colors
`Black`, `White`, `Red`, `Green`, `Blue`, `Yellow`, `Magenta`, `Cyan`, `Gray`, `DarkGray`, `LightGray`

### Key codes
Navigation/control (`K_SPACE`, `K_ESCAPE`, `K_RETURN`, `K_BACKSPACE`, `K_TAB`, `K_UP`, `K_DOWN`, `K_LEFT`, `K_RIGHT`, `K_HOME`, `K_END`, `K_DELETE`), the full alphabet `K_A` … `K_Z`, the digit row `K_0` … `K_9`, the numeric keypad (`K_KP_0` … `K_KP_9`, `K_KP_ENTER`, `K_KP_PLUS`, `K_KP_MINUS`), and punctuation (`K_MINUS`, `K_PLUS`, `K_EQUALS`, `K_LEFTBRACKET`, `K_RIGHTBRACKET`). See `defaults.go` for the authoritative list.

### Mouse
`BUTTON_LEFT`, `BUTTON_RIGHT`

### Event queue (for hand-rolled input loops)
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

### Type aliases
`Color = sdl.Color`, `FPoint = sdl.FPoint`, `FRect = sdl.FRect`, `Keycode = sdl.Keycode`, `Event = sdl.Event`, `EventType = sdl.EventType`, `KeyboardEvent = sdl.KeyboardEvent`, `TextInputEvent = sdl.TextInputEvent`

### Helper constructors
- `Point(x, y float32) sdl.FPoint`
- `Origin() sdl.FPoint` — (0, 0)
- `RGB(r, g, b uint8) sdl.Color`
- `RGBA(r, g, b, a uint8) sdl.Color`
- `FontFromMemory(data []byte, size float32) (*ttf.Font, error)` — load TTF from embedded bytes
- `FontFromFile(path string, size float32) (*ttf.Font, error)`

### Loop sentinel
- `EndLoop` — return from `exp.Run` callback to exit cleanly
- `IsEndLoop(err) bool` — distinguish graceful exit from real errors

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
