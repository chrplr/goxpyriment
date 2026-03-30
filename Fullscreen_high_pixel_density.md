# Fullscreen + HIGH_PIXEL_DENSITY issue

## Symptom

On **macOS** (Retina) and possibly **Raspberry Pi**, running an experiment in fullscreen mode shows a gray screen after the initial setup UI. Windowed mode (`-w`) works correctly. The issue was reported for the `retinotopy` example but likely affects any experiment that uses hardcoded pixel dimensions.

## Root cause analysis

### The asymmetry in `apparatus/NewScreen`

In `apparatus/screen.go`, the fullscreen and windowed paths differ in one critical flag:

**Fullscreen path:**
```go
window, err := sdl.CreateWindow(title, 0, 0, sdl.WINDOW_HIGH_PIXEL_DENSITY)
// ...
w, h, _ := window.SizeInPixels()
return &Screen{..., Width: int(w), Height: int(h)}
```

**Windowed path:**
```go
window, renderer, err := sdl.CreateWindowAndRenderer(title, width, height, sdl.WINDOW_HIDDEN)
// no WINDOW_HIGH_PIXEL_DENSITY
return &Screen{..., Width: width, Height: height}
```

### What `SDL_WINDOW_HIGH_PIXEL_DENSITY` does

Without it, SDL3 follows the OS logical coordinate space:
- On a macOS Retina display at 2560×1600 logical points, SDL gives a 2560×1600 renderer — the OS compositor silently doubles to 5120×3200 physical pixels.
- On standard Linux (pixel density = 1), there is no difference.

With it, SDL3 bypasses logical scaling and gives raw physical pixels:
- Same Retina display: you get a 5120×3200 renderer directly.
- All sizing and coordinate code must work in physical pixels.

### Consequence

| Mode | Flag present | `Screen.Width/Height` | `CenterToSDL` |
|---|---|---|---|
| Windowed | No | logical pixels (e.g. 1024×768) | correct |
| Fullscreen | Yes | physical pixels (e.g. 5120×3200 on Retina) | correct |

`CenterToSDL` uses `renderer.RenderOutputSize()`, so it is always internally consistent. The problem is **any code that uses hardcoded pixel counts**.

In `retinotopy/main.go`:
```go
const WindowWidth  = 768
const WindowHeight = 768

// Texture created at hardcoded logical size:
tex, err := r.Exp.Screen.Renderer.CreateTexture(
    apparatus.PIXELFORMAT_RGBA32,
    apparatus.TEXTUREACCESS_STREAMING,
    WindowWidth, WindowHeight,   // ← wrong in fullscreen HiDPI
)
r.PixelBuffer = make([]byte, WindowWidth*WindowHeight*4)
// ...
r.CombinedTexture.Update(nil, r.PixelBuffer, WindowWidth*4)
```

On a Retina Mac in fullscreen the renderer expects physical pixels (e.g. 5120×3200) but the texture and pixel buffer are 768×768. The stimulus either renders at a tiny fraction of the screen or lands outside the viewport entirely.

## Why the Pi issue was different

The Pi gray screen was caused by a different bug: `binsdl.Load()` tried to extract and load an x86-64 `libSDL3.so` on an ARM64 system, which fails immediately with:

```
binsdl: couldn't sdl.LoadLibrary: /tmp/.../libSDL3.so.0: cannot open shared object file
```

This was fixed by adding build-tag-selected loader files:
- `control/sdlload_embedded.go` (`//go:build !linux || !arm64`) — uses `binsdl/binttf/binimg.Load()`
- `control/sdlload_system.go` (`//go:build linux && arm64`) — calls `sdl.LoadLibrary(sdl.Path())` against the system-installed SDL3

After that fix the Pi works correctly in both windowed and fullscreen — confirming the Pi had no HiDPI issue (pixel density = 1 on a standard monitor).

## The Mac issue — implemented fix

The Mac fullscreen gray screen has been addressed with a framework-level fix in `apparatus/screen.go`.

### Approach — `SetLogicalPresentation` (implemented)

`WINDOW_HIGH_PIXEL_DENSITY` is **kept** (it is needed to get correct centering on Linux). After the renderer is created, the framework immediately queries the logical (OS) pixel dimensions via `window.Size()` and calls:

```go
renderer.SetLogicalPresentation(logW, logH, sdl.LOGICAL_PRESENTATION_STRETCH)
```

SDL3 then maps all drawing commands from logical coordinates (e.g. 2560×1600 on a Retina display) to the full physical resolution (e.g. 5120×3200) transparently. The `Screen` struct stores the logical `Width`/`Height`, which is what experiment code should use for coordinate math. `CenterToSDL` and `MousePosition` already use the `LogicalSize` field, so no changes are needed in experiment code.

| Platform | `window.Size()` | `window.SizeInPixels()` | Renderer logical space |
|---|---|---|---|
| Linux standard (Pi, desktop) | 1920×1080 | 1920×1080 | 1920×1080 — no change |
| macOS Retina 2560×1600 | 2560×1600 | 5120×3200 | 2560×1600 — SDL upscales |

Because `Screen.Width/Height` are now always in logical pixels and `SetLogicalPresentation` is active, experiments using hardcoded pixel values (like the retinotopy texture at 768×768) work correctly on all platforms — SDL scales the output to fill the screen without the experiment needing to know the physical pixel count.
