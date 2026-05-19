// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

# stimuli package

All visual and audio stimulus types, plus high-precision VSYNC-locked presentation loops.

## Core interfaces

```go
type Stimulus interface {
    Present(screen *io.Screen, clear, update bool) error
    Preload() error   // no-op for most visual stimuli
    Unload() error    // destroy GPU texture / audio stream
}

type VisualStimulus interface {
    Stimulus
    Draw(screen *io.Screen) error
    GetPosition() sdl.FPoint
    SetPosition(pos sdl.FPoint)
}
```

Positions are **center-based**: (0,0) = screen center. `screen.CenterToSDL` converts when drawing.

## Lazy GPU allocation

GPU textures are created on the **first `Draw` call**, not at construction. For timing-critical code, force early allocation:

```go
stimuli.PreloadVisualOnScreen(screen, stim)  // single
stimuli.PreloadAllVisual(screen, stims)      // batch
```

`BaseVisual` (embedded by most visual stimuli) provides no-op `Preload()` / `Unload()` and the position accessors.

## Visual stimuli

### Text

**`TextLine`** — Single-line text. `NewTextLine(text, x, y, color)`.
- Optionally set `stim.Font` to override the screen default font before first draw.
- Lazy GPU texture; `Unload()` destroys it. Font change triggers re-preload.

**`TextBox`** — Multi-line wrapped text. `NewTextBox(text, boxWidthPx, position, color)`.
- `Alignment` field: `ttf.HorizontalAlignmentCenter` (default), Left, Right.
- Same lazy texture pattern as TextLine.

### Geometric shapes

**`Circle`** — `NewCircle(x, y, radius, color)`. Drawn with horizontal scanlines, no texture.
- `InsideCircle(areaRadius, areaPos)` — geometric containment check.

**`Rectangle`** — `NewRectangle(x, y, w, h, color)`. Filled rect centered at position.

**`FixCross`** — `NewFixCross(x, y, size, lineWidth, color)`. Two perpendicular lines.

### Images

**`Picture`** — `NewPicture(filePath, x, y)` or `NewPictureFromMemory(data, x, y)`.
- Lazy texture load from file or raw bytes (any SDL-supported format).
- Width/Height available after first `Draw`.

**`Canvas`** — Offscreen render target. `NewCanvas(x, y, w, h, bgColor)`.
- `Blit(stimulus, screen)` — draw a stimulus into the canvas (temporarily shifts coordinate origin).
- `Clear(screen)` — fill with background color.

**`BlankScreen`** — Full-screen colored fill. `NewBlankScreen(color)`.
- `clear` flag in `Present` is ignored; the fill IS the clear.
- Always returns (0, 0) for position.

## Audio stimuli

### Sound (WAV)

```go
snd := stimuli.NewSound("path/to/file.wav")
// or from embedded bytes:
snd := stimuli.NewSoundFromMemory(data)

snd.PreloadDevice(exp.AudioDevice)  // must call before Play
snd.Play()
snd.Wait()  // block until playback done
```

`Wait()` polls `Stream.Queued()` rather than sleeping a fixed duration — correctly handles resampling lookahead.

`PlaySegment(onset, offset, rampSec)` — play only the time window [onset, offset] (seconds). `rampSec` applies a linear fade-in at onset and symmetric fade-out at offset; pass 0 for no ramp. Handles AUDIO_F32*, AUDIO_S16*, AUDIO_U8 natively.

`PlaySoundFromMemory(device, data)` — one-shot synchronous helper, no struct needed.

`PlaySyncedWithFlip(screen) (flipNS uint64, err error)` — VSYNC-synchronised playback. Pauses the audio device, pre-fills the stream, flips the display (blocking on VSYNC), then immediately resumes. Audio onset follows the flip by at most one audio callback period (≤ `frames/sampleRate` s, e.g. ≤ 11.6 ms at 512 frames/44100 Hz). Reduce with `exp.SetAudioSampleFrames(128)` before `Initialize()` (≤ 2.9 ms, higher underrun risk).

### Tone (procedural)

```go
tone := stimuli.NewTone(440.0, 200, 0.5)  // freq Hz, duration ms, amplitude
// complex (additive):
tone := stimuli.NewComplexTone([]float64{440, 880}, 200, 10, 0.5) // freqs, dur, rampMs, amplitude

tone.PreloadDevice(exp.AudioDevice)
tone.Play()
```

### Embedded feedback sounds

```go
stimuli.PlayBuzzer(exp.AudioDevice)  // incorrect response
stimuli.PlayPing(exp.AudioDevice)    // correct response
```

### Audio recording (microphone)

Desktop only (`audio_recorder.go`, `!js`). Uses `sdl.OpenAudioDeviceStream` on a recording device.

```go
mic, _ := exp.SelectAudioRecordingDevice("Select microphone") // control package
rec, _ := stimuli.NewAudioRecorderOnDevice(mic.ID, &stimuli.DefaultRecorderSpec())
rec.Start()
pcm, _ := rec.Drain(nil)
rec.Stop()
stimuli.WriteFloat32WAV(path, pcm, int(rec.OutputFormat().Freq), int(rec.OutputFormat().Channels))
rec.Close()
```

| API | Notes |
|---|---|
| `DefaultRecorderSpec()` | Mono `AUDIO_F32LE` @ 44100 Hz |
| `ListRecordingDevices()` | System default + `GetAudioRecordingDevices` |
| `AudioRecorder` | `Start` / `Stop` / `Read` / `Drain` / `Close` |
| `WriteFloat32WAV` | IEEE float32 LE WAV writer |

`Menu.Caption` supports a title above device lists. Examples: `examples/audio_record`, `examples/audio_record_trials`.

## VSYNC-locked animation loops

All three functions disable GC, drain stale events before the first frame, and return `MotionResult{Key, Button, RTms}`.

### PresentMovingDotCloud

```go
result, err := stimuli.PresentMovingDotCloud(
    screen, nDots, dotRadius, cloudRadius, center,
    speedPxPerSec, maxDurationMs,
    interruptKeys, catchMouse,
    dotColor, bgColor,
)
```

Dots move at constant speed in random directions; respawned at a random position on the cloud boundary when they exit.

### PresentMovingGrating

```go
result, err := stimuli.PresentMovingGrating(
    screen, widthPx, heightPx, center,
    orientationDeg, spatialFreqCyclesPerPx, temporalFreqHz,
    contrast, bgLuminance,
    maxDurationMs, interruptKeys, catchMouse,
)
```

Drifting sinusoidal grating in a rectangular aperture. `orientationDeg` = 0° → vertical bars drifting right. Spatial args precomputed per pixel; only phase advances per frame.

### PresentMovingGabor

Same signature as `PresentMovingGrating` but uses a circular Gaussian envelope so edges fade to background luminance. Per-pixel alpha modulated by the envelope.

### MotionResult

```go
type MotionResult struct {
    Key    sdl.Keycode  // non-zero if response was a keypress
    Button uint8        // non-zero if response was a mouse button
    RTms   int64        // ms from first frame to response (0 on timeout)
}
```

## RSVP / stream presentation

### Visual streams

```go
elements := stimuli.MakeRegularVisualStream(stims, durationOn, durationOff)
// or with per-item timing:
elements := stimuli.MakeVisualStream(stims, onsetMs, durationMs)  // slices

events, timing, err := stimuli.PresentStreamOfImages(elements, x, y)
```

`PresentStreamOfImages` pre-loads all textures, disables GC, aligns to VSYNC, and returns:
- `[]UserEvent` — all SDL events recorded during presentation (with stream-relative timestamps)
- `[]TimingLog` — actual onset/offset vs target for each element

`PresentStreamOfText(words, durationOn, durationOff, x, y, color)` — convenience wrapper that builds TextLine stimuli from strings.

### Audio streams

```go
elements := stimuli.MakeRegularSoundStream(sounds, durationOn, durationOff)
events, timing, err := stimuli.PlayStreamOfSounds(elements)
```

Uses `time.Sleep(1ms)` polling (not VSYNC) for audio timing. `Sound` field in `SoundStreamElement` may be nil for silence.

### Stream types

```go
type VisualStreamElement struct {
    Stimulus   VisualStimulus
    DurationOn  time.Duration
    DurationOff time.Duration
}

type SoundStreamElement struct {
    Sound       AudioPlayable  // nil = silence
    DurationOn  time.Duration
    DurationOff time.Duration
}

type TimingLog struct {
    Index       int
    TargetOn    time.Duration
    ActualOnset time.Duration
    ActualOffset time.Duration
}
```

## GV video

### High-level one-shot

```go
events, logs, err := stimuli.PlayGv(screen, "path/to/video.gv", x, y)
```

Plays an LZ4-compressed RGBA `.gv` file once, frame-by-frame, VSYNC-locked. GC disabled. Exits on ESC/window-close. Returns per-frame timing logs.

### Interactive (manual control)

```go
v, err := stimuli.NewGvVideo("path/to/video.gv")
defer v.Close()

v.Play()
err = exp.Run(func() error {
    if err := v.Update(exp.Screen); err == io.EOF {
        return control.EndLoop
    }
    dest := sdl.FRect{X: 100, Y: 50, W: 640, H: 480}
    v.DrawAt(exp.Screen, &dest)
    exp.Screen.Update()
    return nil
})
```

| Method | Description |
|---|---|
| `Play()` | Start or resume; resets to frame 0 if not yet started |
| `Pause()` | Freeze at current frame; `Update()` becomes a no-op |
| `Rewind()` | Jump back to frame 0 and resume |
| `IsPlaying() bool` | True after `Play()`, before all frames shown |
| `IsPaused() bool` | True while paused |
| `Update(screen) error` | Advance one frame; returns `io.EOF` when done (and on every subsequent call); no-op if paused/stopped |
| `Draw(screen) error` | Render current frame centered at `v.Position` |
| `DrawAt(screen, *sdl.FRect) error` | Render current frame into a custom rectangle |
| `Close()` | Release GPU texture and file handle (alias for `Unload`) |

`Width`, `Height` (float32), `FrameCount` (int), and `FPS` (float64) are populated at construction from the file header. GPU resources are lazy-initialised on the first `Update` or `Draw`/`DrawAt` call.

## Splash screens

```go
// Image + wrapped text message, with optional timeout
stimuli.SplashScreen(screen, imageData, "Press any key to continue", 5.0)

// Image + two text lines with layout control
stimuli.TwoLineSplash(screen, imageData, titleFont, "Title", subtitleFont, "Subtitle", 0, false)
```

`splitLayout=true` — title at vertical center, image+subtitle in lower third.
`splitLayout=false` — all three stacked and centered.

Both return `sdl.EndLoop` on ESC/quit, `nil` on timeout or keypress.

## Key conventions

- Always call `sound.PreloadDevice(exp.AudioDevice)` before playing any `Sound` or `Tone`.
- After `sdl.AudioStream.PutData()`, call `Flush()` to emit resampling lookahead frames — omitting this causes truncated playback when WAV sample rate ≠ device rate.
- GC-disabling loops (`PresentStreamOfImages`, motion loops, `PlayGv`) restore GC via `defer`; do not call these functions from within another GC-disabled scope unless you manage restoration yourself.
- `spatialFreq` is **cycles per pixel** (e.g. 0.05 = one cycle per 20 px), NOT cycles per degree.
- `temporalFreq` is **Hz**.
- `orientation` is **degrees from horizontal** (0° = vertical bars drifting rightward).
