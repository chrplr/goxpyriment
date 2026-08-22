// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

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

> ⚠️ **Axis convention — +Y is UP.** Positive Y is *higher* on the screen
> (maths/vision-science convention), the opposite of SDL's Y-down pixel space —
> `CenterToSDL` computes `height/2 - y`. To stack items above→below on screen,
> give them *decreasing* Y (e.g. header `+90`, target `+35`, box `-27`). Using
> negative Y for "up" mirrors the whole layout vertically — a recurring bug.

### Key methods

| Method | Description |
|---|---|
| `Clear()` | Fill with background color |
| `Update()` / `Flip()` | Present backbuffer and hold to the frame boundary, so one call = one display frame (in the browser: parks until the next requestAnimationFrame — see below) |
| `FlipTS()` | `Flip` + the SDL nanosecond timestamp of the flip |
| `CalibrateRefresh(n)` | Measure the actual frame period over n frames, bypassing pacing |
| `ClearAndUpdate()` | Clear + Present in one call |
| `Size() (w, h int32)` | Current renderer output size |
| `FrameDuration() time.Duration` | Nominal frame time (1 / refresh rate) |
| `VSync() int` | Current VSYNC state (1=on, 0=off, -1=adaptive) |
| `SetVSync(vsync int)` | Change VSYNC mode |
| `SetLogicalSize(w, h int32)` | Device-independent logical resolution with letterboxing. Updates `Screen.Width`/`Height` too — they are the *drawing* space, not the window, and layout code reads them. For physical pixels use `Renderer.CurrentOutputSize()`. |
| `MousePosition() (float32, float32)` | Cursor in center-based coords (HiDPI-corrected) |
| `DisplayInfo() DisplayInfo` | Native resolution, refresh rate, pixel density, format |

### SystemInfo — which GPU actually rendered

`RendererName` is SDL's *backend* ("opengl", "vulkan", "metal"); it never says
which GPU ran it. `GLRenderer` does: it reads OpenGL's `GL_RENDERER` string via
`SDL_GL_GetProcAddress` + purego (pure Go, no CGo — see `glrenderer_notjs.go`),
and lands in every data file as `sys gl_renderer`.

It returns `""` rather than guessing when there is no current GL context, which
is normal for the Vulkan and software renderers, and always on js.

This matters on hybrid-graphics laptops. Measured on Intel + NVIDIA RTX 2000
Ada: rendering defaults to `Mesa Intel(R) Arc(tm) Graphics (MTL)`, and
`__NV_PRIME_RENDER_OFFLOAD=1` with `__GLX_VENDOR_LIBRARY_NAME=nvidia` (or
`__EGL_VENDOR_LIBRARY_FILENAMES=.../10_nvidia.json`) switches it to
`NVIDIA RTX 2000 Ada Generation Laptop GPU/PCIe/SSE2`. Frame timing was
identical on both. Do not infer the GPU from which `/dev/dri/renderD*` node the
process has open — that varies between identical runs.

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
| `GetKeyEventTS(keys, timeoutMS)` | Returns `(key, eventTimestampNS, error)` — hardware-precision SDL3 timestamp |
| `GetKeyEventsTS(keys, timeoutMS)` | Returns `([]InputEvent, error)` — first key + 50 ms simultaneity window; for bilateral responses |
| `CollectKeyEventsTS(keys, durationMS)` | Returns `([]InputEvent, error)` — all keys pressed during the full fixed window |
| `IsPressed(key)` | Returns `true` if key is physically held right now (scancode state, no queue) |
| `WaitKeyReleaseTS(key, timeoutMS)` | Blocks until KEY_UP; returns hardware timestamp for duration measurement |
| `Check()` | Non-blocking poll; returns first key or 0 |
| `Clear()` | Drain SDL event queue |

`PollKeys` is a function injected by the `Experiment`; it drains the SDL queue and returns `(firstKey, quitRequested)`.

## Mouse

```go
m := &apparatus.Mouse{PollButtons: pollFunc}  // injected by control.Experiment
```

| Method | Description |
|---|---|
| `ShowCursor(show bool)` | Toggle cursor visibility. `NewScreen` leaves the cursor visible, but `control.Experiment.Initialize` then hides it — see `control/CLAUDE.md` |
| `Position() (x, y float32)` | Current cursor position in **window pixels** (not center-based) |
| `WaitPress()` | Block until any mouse button pressed |
| `WaitPressRT(timeoutMS)` | Returns `(button, rtMs, error)` |
| `GetPressEventTS(timeoutMS)` | Returns `(button, eventTimestampNS, error)` — hardware-precision SDL3 timestamp |
| `Check()` | Non-blocking poll; returns first button or 0 |

Note: `Position()` returns window-pixel coordinates, unlike `Screen.MousePosition()` which returns center-based coordinates.

## GamePad

```go
pads, err := apparatus.GetGamePads()  // returns []GamePad
defer pads[0].Close()
button := pads[0].WaitPress()  // block until button pressed

// Analog sticks/triggers, −32768..32767. Standardized by SDL's controller
// mapping DB, so LEFTX/LEFTY are always the left stick regardless of device.
x := pads[0].Axis(sdl.GAMEPAD_AXIS_LEFTX)
y := pads[0].Axis(sdl.GAMEPAD_AXIS_LEFTY)
```

Prefer `GamePad` over the low-level `Joystick` (`joystick.go`) for analog input: raw joystick axis numbers are device-specific and axes 0/1 are often a digital D-pad (only 8 directions), whereas the gamepad axes are standardized and properly analog. `GetGamePads()` only returns controllers SDL recognizes; fall back to `GetJoysticks()` for unrecognized devices. See `examples/demo_joystick` for the prefer-gamepad-with-joystick-fallback pattern.

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

### Browser (GOOS=js) presentation

The present path is platform-split (`screen_present_notjs.go` /
`screen_present_js.go`). On js, `present()` submits to the canvas and then
parks until the browser's next requestAnimationFrame tick
(`sdl.WaitAnimationFrame` in the go-sdl3 fork) — required both for pacing
(RAF = the browser's VSYNC) and for correctness: canvas updates only
composite when the page yields, and the desktop busy-wait never
yields, so its wait is a no-op on js. Measured: 60.00 Hz, SD ≈ 0.12 ms, no
dropped frames (see `docs/WASM.md`).

## Update always holds to the frame boundary

`SDL_RenderPresent` cannot be trusted to block until the retrace. Under
triple/mailbox buffering it queues the frame and returns immediately, and the
per-frame loop then runs faster than the display — stimuli are replaced before
the panel scans them out. This is not an exotic configuration: measured on
Intel i915 + Wayland driving a well-behaved 120 Hz panel, unaided presents
still came back as little as **6.95 ms** apart against an 8.33 ms frame.

So `Update` presents *and then busy-waits* to the expected frame boundary
(`paceToFrame`, `screen_present_notjs.go`). Where the driver does block
correctly no hold runs at all — the present had already covered the frame, so
`paceToFrame` keeps its stamp and returns (see the anchor table below).

There is deliberately **no per-platform switch**. Whether Present blocks
depends on driver + compositor + window mode + GPU, not on `GOOS` or the SDL
video driver name — the same box behaves differently windowed vs fullscreen.
Since the spin is free when unneeded, pacing unconditionally is both simpler
and safer than predicting. This replaced the old `PacedFlip`/`PacedFlipTS`
pair, which existed only because the caller had to choose.

Pacing is skipped when VSync is off (`pacingEnabled`), since a caller who
disabled VSync wants frames as fast as the GPU produces them. `SetVSync`
refreshes that cached state.

### What the flip timestamp is anchored to

`lastFlipNS` — the value `FlipTS` returns — is **not** always the instant
`SDL_RenderPresent` returned. `paceToFrame` picks the anchor per frame:

| driver behaviour | `FlipTS` returns | next target derives from |
|---|---|---|
| present returned at/after the boundary | the present return (present()'s own stamp) | the hardware, re-anchored every frame |
| present returned early by ≤ `frameDur/hwAnchorSlackDiv` | the present return, and no hold runs | the hardware, re-anchored every frame |
| present returned early by more | the **scheduled** frame boundary | the previous target, +`frameDur` |
| a kernel vblank was available (`GOXPY_VBLANK`) | the vblank stamp | the vblank, +whole frames to the next boundary |

The second row is the common case on a well-behaved driver and it is not a
rounding convenience. A blocking present returns one *panel* period after the
last one, while the boundary is one *nominal* period after, so it lands slightly
inside — measured on a Precision 5490 (Intel/Mesa, Wayland) 2026-08-16: mean
0.676 ms, max 1.14 ms of a 16.661 ms frame, i.e. present had covered 15.99 ms of
every frame. Sending that to the hold replaced a hardware stamp with a schedule
on 98.8 % of frames, which is the construction that drifts. The shortfall is
tallied as `Early` instead; `hwAnchorSlackDiv` documents the threshold and its
one known false positive.

The paced branch must anchor on the scheduled boundary, not on the spin exit.
The spin exits at `target + ε` (one clock-read iteration); feeding that back
made the schedule ratchet, and since ε ≥ 0 the slide is one-signed and never
averages out. Measured on a Pi 4 (V3D/kmsdrm) with a BBTK photodiode against a
GPIO TTL: 0.467 µs/frame of ε put the framework's flip timestamps **14 ms adrift
from the actual photons over an 8-minute run**, growing linearly, while an
Intel/Mesa laptop at the console showed no drift at all because there present
blocks and every frame re-anchors. Details and figures in `paceToFrame`'s
comment.

Residual after that fix is `|true refresh − nominal refresh|`, since the paced
branch advances by exactly `frameDur` (taken from the display mode).

**Do not seed `frameDur` from `CalibrateRefresh` to close it — that makes it
worse.** Against the panel's true frame period, recovered by regression over the
photodiode trains of the two runs above (`cmd/timing-drift`, 1000 cycles each):

| | Pi 4 (V3D) | Precision 5490 (Intel/Mesa) |
|---|---|---|
| true panel rate | 60.0000 Hz | 60.0385 Hz |
| nominal display mode | 60.0000 Hz (**−0.1 ppm**) | 60.0400 Hz (**−25 ppm**) |
| `CalibrateRefresh(60)` | 60.0043 Hz (−72 ppm) | 60.0228 Hz (+261 ppm) |

The display mode wins by a factor of ten on both. `CalibrateRefresh` takes the
median of 59 intervals from a deliberately unpaced loop, so on a driver that does
not block it measures the loop, not the panel.

**Those "nominal display mode" figures were SDL's `RefreshRate` float, which SDL3
rounds to two decimals.** `FrameDuration` now reads the mode's exact rational
(`RefreshRateNumerator/Denominator`) instead — on the 5490 that is 60.038 Hz, not
60.0400, so the −25 ppm above becomes about −8 ppm (recomputed from the mode, not
re-measured). Measured error of the rounded value: +33.3 ppm on the 5490,
+4.3 ppm on a Pi 4 whose kmsdrm mode is 108000 kHz / (1688 × 1066) =
60.019740 Hz against a reported 60.02. What is left on the 5490 is Wayland
reporting refresh in whole mHz — 16.7 ppm of quantisation at 60 Hz — which no
reading of the mode can recover; kmsdrm derives the rational from the timing and
is exact. It remains the right tool for the
job it documents below; it is not a rate reference. The kernel's vblank timestamp
is — consecutive `DRM_IOCTL_WAIT_VBLANK` stamps on the 5490 give 60.0384 Hz,
**1.3 ppm** from the photodiode truth, and `vblank/drm_linux.go` already
reads them for the movie player.

`Screen.PacingStats()` reports which branch the presents took — `Blocked`,
`Early` (a subset of `Blocked`), `Paced`, `VblankHeld`, `Presents()`, the wait
across the paced ones (`WaitMean`/`WaitMax`) and the shortfall across the early
ones (`EarlyMean`/`EarlyMax`) — with `ResetPacingStats()` to exclude warm-up.
Re-exported as `control.PacingStats`. `tests/Timing-Tests -test display`,
`tests/test_vsync_blocking` and `tests/test_vblank_drift` all print it.

### Validated against photons, 2026-08-17

A BBTK v3 photodiode, 1010 cycles per run, the same protocol that measured the
failure in the first place:

| | before the anchor change | after |
|---|---|---|
| Raspberry Pi 4 (V3D/kmsdrm) | 14 ms over 8 min | **+0.01 ppm** (0.006 ms / 8.3 min) |
| Radeon Pro W5700 (radeonsi/X11) | **−4.73 ppm**, mechanism confirmed | **+0.13 ppm** (0.066 ms / 8.3 min) |

The W5700 is the row that establishes the mechanism rather than the symptom. Its
GPU and CPU keep independent crystals, so its loop cadence sits **−4.20 ppm** off
nominal — measured this same day, and unchanged by any of this. While presents
were held, the timestamps advanced on the CPU clock at the nominal rate while the
panel ran on the GPU clock, and that crystal difference *was* the drift: −4.73 ppm.
Anchoring on present's return moves the difference inside the loop, so the cadence
and the drift now disagree by a factor of 32 — which is the evidence that the
timestamps are following the panel and not the schedule.

On a Pi the two clocks derive from one SoC oscillator, which is why its cadence
came out at +0.64 ppm and why the same fix looks less dramatic there.

**Grade a run on `WaitTotal/(Presents × frameDur)`, not on the paced share.**
The share alone called the machine above non-blocking while present was blocking
for 96 % of every frame; weighting the hold by the count collapses that to 4 %
and separates it from a driver that really does not block (90 %+). It also
handles the opposite shape — a blocking driver with one rare buffered frame,
whose per-paced-frame mean wait is nearly a whole frame. `classifyPacing` in
`tests/Timing-Tests/main.go` is the reference implementation.

**The branch counts are the verdict; the frame-interval medians are not.**
Pacing exists to make the median interval come out right, so it does. Measured
on the 5490 under Wayland, windowed: nominal 16.656 ms, unaided 16.653 ms, paced
16.656 ms — three medians agreeing to 3 µs while **99.7 % of presents returned a
mean 6.5 ms early**. `test_vsync_blocking` reported BLOCKING on that
configuration until it was changed to count branches instead of compare medians.

**Pacing enforces a minimum frame time, not a maximum.** It cannot recover a
frame the compositor dropped. `CalibrateRefresh` is how you tell the two apart:
it presents directly, bypassing the spin, so its median interval is the unaided
driver behaviour. `control.Experiment.Initialize` runs it over 60 frames at
startup and writes both rates into the data file (`sys refresh_nominal_hz`,
`sys refresh_measured_hz`), warning on the log if they disagree by >10%.
`tests/test_vsync_blocking` reports all three numbers interactively.

## Never present a frame with no draw calls

A frame whose entire content is a clear — `Renderer.Clear()` then present, with no
drawing in between — **is not reliably scanned out under a compositor**. The panel
keeps showing a stale frame, for seconds at a time, while the client's flip
timestamps and the compositor both report every frame presented on schedule. It is
invisible without a photodiode.

Reproduced on GNOME/Mutter under both native Wayland and Xwayland, on Intel Meteor
Lake / i915, with the `opengl`, `vulkan` *and* `software` SDL renderers, on two
different panels (internal eDP and external DP). It does **not** occur on the
kmsdrm backend, where no compositor is involved. Ruled out along the way: audio
backend, flip pacing (`SDL_RenderPresent` blocks correctly at 16.65 ms), HiDPI
scaling, GNOME idle dimming, PSR/Panel Replay, the SDL video backend, and the
renderer. `SDL_FlushRenderer` before the present does not help either — see the
note in `screen_present_notjs.go`. The only remedy found is to give the frame real
draw work.

**What the library guarantees.** `Screen.Clear()` emits a full-screen
`RenderFillRect` after the clear (`fillWholeTarget`). Everything routed through
the `Screen` API is therefore safe: `Clear`, `ClearAndUpdate`, `Blank`, `Show`,
and every `stimuli` presentation path.

**What it does not guarantee.** `Screen.Renderer` is public and is the documented
extension point for custom stimulus types, so this stays reachable:

```go
screen.Renderer.SetDrawColor(0, 0, 0, 255)
screen.Renderer.Clear()   // no draw call
screen.Update()           // may not reach the panel
```

Closing that off would mean un-exporting `Renderer`, which breaks custom stimuli.
So the rule for any code that drives the renderer directly: **every frame must
contain at least one draw call.** If a frame is meant to be a uniform colour, use
`Screen.Clear()` rather than `Renderer.Clear()`.

`tests/test_clear_only_frames` is the regression test. Unguarded it drives the
renderer directly and *should* fail on an affected system; `-guarded` goes through
`Screen.Clear()` and must always pass. If the guarded run ever fails, the
guarantee has regressed.

**A frame with no commands at all is worse still.** `stimuli/stream.go` used to
present one, holding external content by re-presenting in the hope the front
buffer would persist; under a compositor that could freeze the display on a stale
frame for the whole held slot. It now clears instead. The general rule: never
present without drawing, and do not rely on backbuffer persistence — SDL
invalidates the backbuffer on every present, so there is also no way to read back
what is currently displayed (`RenderReadPixels` would capture an already-
invalidated buffer).

## There is no "wait for n VSYNCs" call — a hold redraws every frame

SDL3 has no `SDL_WaitVBlank`: the retrace is not observable on its own, so the
only way to stay locked to the display is to present a frame. Combined with the
rule above — a frame with no draw calls may never reach the panel — this means a
"hold this for n frames" primitive *must* redraw its content once per frame.

`Screen.WaitFrames(n)` used to offer the shortcut for solid-colour frames: it
re-cleared the backbuffer with the renderer's **current draw colour** and
flipped. After any stimulus that set its own colour (every `Draw` does), the
hold painted the whole screen in that colour — a white rectangle held for 10
frames turned the screen white for 9 of them. It has been removed. The
replacements are `control.Experiment.ShowFrames(stim, n)` and
`control.Experiment.BlankFrames(n)`, which run the explicit loop and return the
first flip's timestamp. Do not reintroduce a hold that skips the redraw.

## Key conventions

- `Clear()` + `Update()` on `Screen` maps to SDL clear + present; `Update()` blocks on VSYNC (browser: until next requestAnimationFrame).
- Mouse `Position()` is in window pixels; use `Screen.MousePosition()` for center-based comparison with stimuli.
- `apparatus` is rarely imported directly in experiment code — access is through `exp.Screen`, `exp.Keyboard`, etc. Direct import is needed only when writing custom stimulus types.
