// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

# apparatus package

SDL window/renderer, keyboard, mouse, gamepad, gamma corrector, and unified input abstractions.

## Screen

```go
screen, err := apparatus.NewScreen("My Experiment", 1024, 768, bgColor, false, 0)
defer screen.Destroy()
```

Passing `fullscreen=true` or `width==0 && height==0` opens an exclusive fullscreen window at native resolution. Windowed screens are hidden at creation and shown after setup.

### Coordinate system

All stimulus positions and the mouse cursor use a **center-based** coordinate system: (0, 0) = screen center. `CenterToSDL(x, y)` converts to SDL's top-left origin for drawing calls.

```go
sdlX, sdlY := screen.CenterToSDL(posX, posY)
```

### Key methods

| Method | Description |
|---|---|
| `Clear()` | Fill with background color |
| `Update()` / `Flip()` | Present backbuffer; blocks on VSYNC |
| `ClearAndUpdate()` | Clear + Present in one call |
| `Size() (w, h int32)` | Current renderer output size |
| `FrameDuration() time.Duration` | Nominal frame time (1 / refresh rate) |
| `VSync() int` | Current VSYNC state (1=on, 0=off, -1=adaptive) |
| `SetVSync(vsync int)` | Change VSYNC mode |
| `SetLogicalSize(w, h int32)` | Device-independent logical resolution with letterboxing |
| `MousePosition() (float32, float32)` | Cursor in center-based coords (HiDPI-corrected) |
| `DisplayInfo() DisplayInfo` | Native resolution, refresh rate, pixel density, format |

### DisplayInfo

```go
type DisplayInfo struct {
    ID             sdl.DisplayID
    Name           string
    NativeW, NativeH int32
    PixelDensity   float32
    RefreshRate    float32
    BitsPerPixel   int
    BitsPerChannel int
    PixelFormat    sdl.PixelFormat
}
```

### CanvasOffset

`screen.CanvasOffset` is an optional `*sdl.FPoint` that temporarily shifts the coordinate origin. Used internally by `stimuli.Canvas.Blit`; do not set it in experiment code unless implementing custom offscreen rendering.

### Type re-exports

`apparatus` re-exports common SDL types so stimuli code only imports `apparatus`:

```go
type FRect      = sdl.FRect
type FPoint     = sdl.FPoint
type Color      = sdl.Color
type Texture    = sdl.Texture
type Surface    = sdl.Surface
type PixelFormat = sdl.PixelFormat
type TextureAccess = sdl.TextureAccess
type BlendMode  = sdl.BlendMode
```

## Keyboard

```go
kb := &apparatus.Keyboard{PollKeys: pollFunc}  // injected by control.Experiment
```

| Method | Description |
|---|---|
| `Wait()` | Block until any key; returns keycode or `sdl.EndLoop` |
| `WaitKeys(keys []sdl.Keycode, timeoutMS int64)` | Block for one of the listed keys or timeout (-1 = no timeout) |
| `WaitKey(key sdl.Keycode)` | Convenience for single key |
| `WaitKeysRT(keys, timeoutMS)` | Returns `(key, rtMs, error)` |
| `WaitKeysEventRT(keys, timeoutMS)` | Returns `(key, eventTimestampNS, error)` — hardware-precision SDL3 timestamp |
| `Check()` | Non-blocking poll; returns first key or 0 |
| `Clear()` | Drain SDL event queue |

`PollKeys` is a function injected by the `Experiment`; it drains the SDL queue and returns `(firstKey, quitRequested)`.

## Mouse

```go
m := &apparatus.Mouse{PollButtons: pollFunc}  // injected by control.Experiment
```

| Method | Description |
|---|---|
| `ShowCursor(show bool)` | Toggle cursor visibility |
| `Position() (x, y float32)` | Current cursor position in **window pixels** (not center-based) |
| `WaitPress()` | Block until any mouse button pressed |
| `WaitPressRT(timeoutMS)` | Returns `(button, rtMs, error)` |
| `WaitPressEventRT(timeoutMS)` | Returns `(button, eventTimestampNS, error)` — hardware-precision SDL3 timestamp |
| `Check()` | Non-blocking poll; returns first button or 0 |

Note: `Position()` returns window-pixel coordinates, unlike `Screen.MousePosition()` which returns center-based coordinates.

## GamePad

```go
pads, err := apparatus.GetGamePads()  // returns []GamePad
defer pads[0].Close()
button := pads[0].WaitPress()  // block until button pressed
```

## GammaCorrector

```go
gc := apparatus.NewGammaCorrectorUniform(2.2)
corrected := gc.CorrectColor(sdl.Color{R: 128, G: 128, B: 128, A: 255})
// corrected.R ≈ 186 — the physical digital value for 50% luminance on γ=2.2

// Per-channel gamma (from photometer measurements)
gc = apparatus.NewGammaCorrector(2.1, 2.2, 2.3)
```

## Input abstraction (DeviceKind, InputEvent)

```go
type DeviceKind int
const (
    DeviceKeyboard DeviceKind = iota
    DeviceMouse
    DeviceGamepad
    DeviceTTL
)

type InputEvent struct {
    Device      DeviceKind
    Key         sdl.Keycode   // DeviceKeyboard
    Button      uint32        // DeviceMouse or DeviceGamepad
    TimestampNS uint64        // SDL3 nanosecond hardware timestamp
}
```

## ResponseDevice interface

Unified input abstraction for device-agnostic experiment code.

```go
type ResponseDevice interface {
    WaitResponse(ctx context.Context) (Response, error)
    DrainResponses(ctx context.Context) error
}

type Response struct {
    Source  DeviceKind
    Code    uint32
    RT      time.Duration
    Precise bool  // true = SDL3 nanosecond accuracy; false = poll-interval accuracy
}
```

Construct wrappers:

```go
rd := &apparatus.KeyboardResponseDevice{KB: exp.Keyboard}
rd := &apparatus.MouseResponseDevice{M: exp.Mouse}
rd := &apparatus.GamepadResponseDevice{GP: pad}
rd := apparatus.NewTTLResponseDevice(box, 5*time.Millisecond)
```

## Key conventions

- `Clear()` + `Update()` on `Screen` maps to SDL clear + present; `Update()` blocks on VSYNC.
- Mouse `Position()` is in window pixels; use `Screen.MousePosition()` for center-based comparison with stimuli.
- `apparatus` is rarely imported directly in experiment code — access is through `exp.Screen`, `exp.Keyboard`, etc. Direct import is needed only when writing custom stimulus types.
