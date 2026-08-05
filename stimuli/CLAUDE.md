// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

# stimuli package

All visual and audio stimulus types, plus high-precision VSYNC-locked presentation loops.

## Core interfaces

```go
type Stimulus interface {
    Present(screen *apparatus.Screen, clear, update bool) error
    Preload() error   // no-op for most visual stimuli
    Unload() error    // destroy GPU texture / audio stream
}

type VisualStimulus interface {
    Stimulus
    Draw(screen *apparatus.Screen) error
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

`PlayTS() (onsetNS uint64, err error)` — plays and returns the **estimated**
onset on the SDL nanosecond clock (same clock as `ShowTS`/`FlipTS`/event
timestamps), so an A/V asynchrony is a subtraction. The estimate is the moment
the data was queued plus `OutputLatency()`; it is good to ±one callback period
and is blind to the DAC, OS mixer DSP, Bluetooth transport and the analog path.
For publishable numbers, measure the rig with external hardware
(`tests/Timing-Tests -test av`) and treat `PlayTS` as the software-side bound.

`OutputLatency() (time.Duration, error)` — the delay that estimate is built
from: hardware buffer period + anything still queued in the software stream.
Report this alongside any onset you derive from `PlayTS`.

Both are available on `Tone` as well as `Sound`.

`PlaySyncedWithFlip(screen) (flipNS uint64, err error)` — VSYNC-synchronised playback. Pauses the audio device, pre-fills the stream, flips the display (blocking on VSYNC), then immediately resumes. Audio onset follows the flip by at most one audio callback period (`frames/sampleRate` s). **The buffer size is the driver's choice, not a goxpyriment default** — measured 1024 frames on PipeWire at 44100 Hz, i.e. ≤ 23.2 ms, not the 512/11.6 ms often assumed. Read the real value with `snd.OutputLatency()` or `exp.AudioDevice.Format()`; reduce it with `exp.SetAudioSampleFrames(128)` before `Initialize()` (≤ 2.9 ms, higher underrun risk). **Browser (GOOS=js):** audio playback works (Web Audio backend), but the device buffer is ~2048 frames (≈ 43 ms at 48 kHz) and these onset guarantees do not transfer — treat browser audio onset as approximate (see `docs/WASM.md`).

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

### PresentMovingGratingDisk

Same idea as `PresentMovingGabor` but with a hard-edged circular aperture instead of a Gaussian: full contrast inside the disk (`diameter` px), invisible outside — no soft falloff.

```go
result, err := stimuli.PresentMovingGratingDisk(
    screen, diameterPx, center,
    orientationDeg, spatialFreqCyclesPerPx, temporalFreqHz,
    contrast, bgLuminance,
    maxDurationMs, interruptKeys, catchMouse,
)
```

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

### Mixed streams

```go
elements := stimuli.MakeRegularStream(stims, durationOn, durationOff) // stims is []Stimulus
// or per-item: stimuli.MakeStream(stims, onsetMs, durationMs)
events, timing, err := stimuli.PresentStreamOfStimuli(exp.Screen, elements, x, y)
```

`PresentStreamOfStimuli` presents a heterogeneous **sequential** stream mixing visual and audio stimulus types, on the same VSYNC-locked GC-disabled loop as `PresentStreamOfImages` (which now delegates to it). Per element it type-switches on `VisualStimulus`:
- **Visual** → centered on `(x,y)`, redrawn every frame for `DurationOn`, blanked for `DurationOff`.
- **Audio / non-visual** (and `nil`) → triggered once via `Present(screen,false,false)` right after the slot's first VSYNC flip; the previous frame is **held** for the whole slot by re-rendering the last stream visual (or the background) every frame — not by relying on GPU backbuffer persistence, which SDL leaves undefined, which flickers on double-buffered drivers, and which under a compositor can freeze the display on a stale frame (a frame with no draw calls is not reliably scanned out). `OnsetNS` = that flip's timestamp.

Audio elements must be device-bound (`PreloadDevice`) beforehand — only visual elements are auto-preloaded. No concurrent AV overlap (strictly one slot at a time). For pure audio, prefer `PlayStreamOfSounds` (finer sub-frame timing).

### Per-frame callback

`PresentStreamOfStimuliFunc(screen, elements, x, y, onFrame)` is `PresentStreamOfStimuli` plus an optional `FrameCallback` invoked once per frame, **just before each flip**, on every on- and off-phase frame:

```go
header := stimuli.NewTextLine("3 / 40", 0, -300, control.White) // trial counter
cb := func(ctx stimuli.FrameContext) error {
    _ = header.Draw(ctx.Screen)              // (a) persistent overlay (drawn over the stimulus)
    if ctx.OnPhase && ctx.FirstFrame {       // (b) real-time logic at each element onset
        // e.g. fire feedback off ctx.NowNS / ctx.Events, return sdl.EndLoop to stop
    }
    return nil
}
events, timing, err := stimuli.PresentStreamOfStimuliFunc(screen, elements, 0, 0, cb)
```

`FrameContext` carries `Screen, Index, Frame, OnPhase, FirstFrame, NowNS` (pre-flip `sdl.TicksNS()`), `Elapsed`, and `Events` (through the previous frame). Return `nil` to continue, `sdl.EndLoop` to stop gracefully, any other error to abort. This unlocks persistent overlays (trial counters, frame borders, fixation) and mid-stream feedback that the plain stream functions can't express. On held (audio) frames the stream re-renders the carried-over visual before the callback, so overlays no longer accumulate. Content drawn *outside* the stream is not carried over at all — the stream cannot reconstruct what it did not draw, so a held slot with no preceding stream visual shows the background. Put such content in the stream as a `VisualStimulus` if it must persist.

### Stream types

```go
type StreamElement struct {       // heterogeneous (PresentStreamOfStimuli)
    Stimulus    Stimulus          // visual or audio; nil = held delay
    DurationOn  time.Duration
    DurationOff time.Duration
}

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

## MPEG-1 video (with MP2 audio)

`Video` plays an MPEG-1 program stream (`.mpg` / `.mpeg`) decoded in pure Go by
`gen2brain/mpeg`. Unlike `GvVideo` it is wall-clock driven, not VSYNC-locked — it
drops frames under load rather than falling behind, so onset timing is only
approximate. For timing-critical stimuli prefer `.gv`.

```go
v, err := stimuli.NewVideo(exp.Screen, "clip.mpg")
defer v.Close()

v.PreloadDevice(exp.AudioDevice) // enable the MP2 soundtrack (see below)
v.Play()

exp.Run(func() error {
    if err := v.Update(); err == io.EOF {
        return control.EndLoop
    }
    exp.Screen.Clear()
    v.Draw(exp.Screen, 0, 0)
    exp.Screen.Update()
    return nil
})
```

**Audio is off until `PreloadDevice` is called.** Without it the video plays
silently and audio packets are discarded at the demuxer (leaving decoding enabled
with nothing draining the buffer would grow it for the length of the clip).
`PreloadDevice` is a no-op returning `nil` when the file has no audio stream, so
it is safe to call unconditionally; check `v.HasAudio` if you need to know.

| Method | Description |
|---|---|
| `PreloadDevice(device) error` | Enable MP2 audio and bind it to an audio device. Call before `Play` |
| `HasAudio` (field) | True if the file carries an audio stream |
| `SetVolume(gain) error` | Scale playback gain (1.0 = unchanged); no-op without audio |

`Update()` tops the SDL audio queue back up to 100 ms ahead each call. Audio is
free-running once queued, so it stays aligned with the video to within that depth.
Two consequences worth knowing:

- `Pause()` clears the queue so sound stops with the image; because that audio was
  already decoded, resuming leaves audio up to 100 ms ahead of the video.
- `Rewind()` clears the queue too, so a restarted clip does not play over its own
  tail.

## GV video

### High-level one-shot

```go
events, logs, err := stimuli.PlayGv(screen, "path/to/video.gv", x, y)
```

Plays an LZ4-compressed RGBA `.gv` file once, frame-by-frame, VSYNC-locked. GC disabled. Exits on ESC/window-close. Returns per-frame timing logs.

### Per-frame callback

```go
events, logs, err := stimuli.PlayGvFunc(screen, "stim.gv", 0, 0, func(ctx stimuli.GvFrameContext) error {
    if ctx.Frame == onsetFrame {
        trig.SetHigh(0)          // rising edge tied to this frame's flip
    }
    return nil                   // sdl.EndLoop to stop early
})
```

`GvFrameContext` carries `Screen`, `Frame` (0-based video frame index), `OnsetNS` (SDL timestamp of that frame's **first** flip — same clock as event timestamps), and `Hold` (refreshes this frame is shown for). The callback runs once per video frame, immediately after the onset flip and before the remaining hold flips, so a trigger raised there lands as close to the onset as possible.

It runs inside the GC-disabled VSYNC loop: do not allocate heavily, never sleep, and never call a blocking `Pulse` — raise the line here and lower it from a later frame. `PlayGv` is `PlayGvFunc` with a nil callback.

**Frame rate:** plays at the rate in the `.gv` header, holding each frame for `refresh / fps` refreshes (30 fps on 60 Hz → 2 refreshes per frame). Both rates are rounded first (59.94 Hz + 29.97 fps behave as 60 + 30). Only exact integer ratios work; 24 fps on 60 Hz needs 2.5 and returns an error naming a workable rate, because the pulldown would make onsets uneven. Before this was enforced, `PlayGv` presented one video frame per refresh and a 30 fps clip played at double speed.

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
| `CurrentFrame() int` | 0-based index of the frame in the GPU texture — the one the next flip shows; `-1` before the first `Update`. Use it to act on a specific frame (e.g. raise a TTL line) |
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
