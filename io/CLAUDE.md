// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

# io package

SDL window/renderer, keyboard, mouse, gamepad, and experiment data file I/O.

## Screen

```go
screen, err := io.NewScreen("My Experiment", 1024, 768, bgColor, false)
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

`io` re-exports common SDL types so stimuli code only imports `io`:

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
kb := &io.Keyboard{PollKeys: pollFunc}  // injected by control.Experiment
```

| Method | Description |
|---|---|
| `Wait()` | Block until any key; returns keycode or `sdl.EndLoop` |
| `WaitKeys(keys []sdl.Keycode, timeoutMS int64)` | Block for one of the listed keys or timeout (0 = no timeout) |
| `WaitKey(key sdl.Keycode)` | Convenience for single key |
| `WaitKeysRT(keys, timeoutMS)` | Returns `(key, rtMs, error)` |
| `Check()` | Non-blocking poll; returns first key or 0 |
| `Clear()` | Drain SDL event queue |

`PollKeys` is a function injected by the `Experiment`; it drains the SDL queue and returns `(firstKey, quitRequested)`. Keyboard instances are not useful without it.

## Mouse

```go
m := &io.Mouse{PollButtons: pollFunc}  // injected by control.Experiment
```

| Method | Description |
|---|---|
| `ShowCursor(show bool)` | Toggle cursor visibility |
| `Position() (x, y float32)` | Current cursor position in **window pixels** (not center-based) |
| `WaitPress()` | Block until any mouse button pressed |
| `Check()` | Non-blocking poll; returns first button or 0 |

Note: `Position()` returns window-pixel coordinates, unlike `Screen.MousePosition()` which returns center-based coordinates. Use `Screen.MousePosition()` when comparing against stimulus positions.

## GamePad

```go
pads, err := io.GetGamePads()  // returns []GamePad
defer pads[0].Close()
button := pads[0].WaitPress()  // block until button pressed
```

## DataFile

```go
df, err := io.NewDataFile(directory, subjectID, expName)
```

Creates `<directory>/<expName>_<subjectID>_<timestamp>.xpd`. Directory is created if absent. A metadata header is written automatically with start time, hostname, OS, and framework version.

| Method | Description |
|---|---|
| `AddVariableNames(names []string)` | Write CSV header row |
| `Add(...interface{})` | Append a data row (subject_id prepended; fields quoted when needed) |
| `WriteDisplayInfo(DisplayInfo)` | Write display metadata as comment block |
| `WriteParticipantInfo(map[string]string)` | Write participant metadata (keys sorted) |
| `WriteEndTime()` | Write session duration |
| `Save()` | Flush buffer to disk |

Fields containing the delimiter or quotes are automatically escaped. The subject_id column is always the first column, prepended by `Add`.

### Constants

| Constant | Value |
|---|---|
| `OutputFileCommentChar` | `"#"` |
| `OutputFileEOL` | `"\n"` |
| `DataFileDirectory` | `"goxpy_data"` |
| `DataFileDelimiter` | `","` |

## OutputFile

Lower-level buffered text file, used as the base of `DataFile`.

```go
f, err := io.NewOutputFile(directory, filename, commentChar)
f.Write(content)
f.WriteLine(content)    // content + EOL
f.WriteComment(text)    // commentChar + text + EOL
f.Save()                // flush to disk
```

The `Save()` method is defined in `output_file_desktop.go` (build tag: non-wasm). A no-op stub exists in `output_file_wasm.go` for WebAssembly targets.

## Version

`io.Version` is a `string` var set from build info at init time:
- Returns the git tag when the library is used as a versioned module dependency.
- Returns `"(devel)"` when built from source via `go.work`.

## Key conventions

- `Clear()` + `Update()` on `Screen` maps to SDL clear + present; `Update()` blocks on VSYNC.
- Mouse `Position()` is in window pixels; use `Screen.MousePosition()` for center-based comparison with stimuli.
- Always call `df.Save()` after the experiment loop ends — the buffer is not flushed automatically.
- `DataFile.Add` prepends subject_id automatically; do not include it in `AddVariableNames`.
