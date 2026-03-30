// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

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

`exp.Run` wraps the SDL event loop. User code panicked with `exitPanic` is recovered there; callers never see it directly. Return `control.EndLoop` (or `sdl.EndLoop`) to exit cleanly.

## Experiment fields

| Field | Type | Description |
|---|---|---|
| `Screen` | `*io.Screen` | Window + renderer |
| `Keyboard` | `*io.Keyboard` | Blocking/non-blocking key input |
| `Mouse` | `*io.Mouse` | Mouse button + position input |
| `AudioDevice` | `sdl.AudioDeviceID` | Passed to `Sound.PreloadDevice` |
| `Audio` | `*AudioManager` | High-level audio playback |
| `Data` | `*io.DataFile` | `.csv` experiment data file |
| `Design` | `*design.Experiment` | Trial/block structure |
| `Info` | `map[string]string` | Participant metadata (from `GetParticipantInfo`) |
| `SubjectID` | `int` | Set by `-s` flag or `GetParticipantInfo` |
| `DefaultFont` | `*ttf.Font` | Passed to stimuli that omit an explicit font |
| `DefaultFontSize` | `int` | Font size used at init |
| `BackgroundColor` | `sdl.Color` | Screen background |
| `ForegroundColor` | `sdl.Color` | Default text color |
| `OutputDirectory` | `string` | Where `.csv` files are written |

## Convenience methods

- `exp.Show(stim)` — `Clear()` + `Draw()` + `Update()` in one call. Use for single-stimulus frames.
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
`K_SPACE`, `K_ESCAPE`, `K_RETURN`, `K_UP`, `K_DOWN`, `K_LEFT`, `K_RIGHT`, `K_F`, `K_J`, `K_Q`, `K_Y`, `K_N`, `K_1` … `K_9`, `K_0`, plus others; see `defaults.go` for the full list.

### Mouse
`BUTTON_LEFT`, `BUTTON_RIGHT`

### Type aliases
`Color = sdl.Color`, `FPoint = sdl.FPoint`, `FRect = sdl.FRect`, `Keycode = sdl.Keycode`

### Helper constructors
- `Point(x, y float32) sdl.FPoint`
- `Origin() sdl.FPoint` — (0, 0)
- `RGB(r, g, b uint8) sdl.Color`
- `RGBA(r, g, b, a uint8) sdl.Color`
- `FontFromMemory(data []byte, size int) (*ttf.Font, error)` — load TTF from embedded bytes
- `FontFromFile(path string, size int) (*ttf.Font, error)`

### Loop sentinel
- `EndLoop` — return from `exp.Run` callback to exit cleanly
- `IsEndLoop(err) bool` — distinguish graceful exit from real errors

## Audio latency tuning

Call **before** `NewExperiment` or `Initialize`:

```go
control.SetAudioSampleFrames(256) // lower = less latency; default 4096
```

## Key conventions for this package

- Never import `go-sdl3` directly in experiment code; use the re-exports in `defaults.go`.
- `exp.End()` must always be deferred to clean up SDL, TTF, and the audio device.
- `QuitRequested` in `EventState` is sticky — once true it stays true for that `PollEvents` result. Check it immediately.
- `exitPanic` is an internal sentinel; never compare against it directly. Use `IsEndLoop`.
