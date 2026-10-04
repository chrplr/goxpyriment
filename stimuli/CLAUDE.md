// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

# stimuli package

All visual and audio stimulus types, plus high-precision VSYNC-locked presentation loops.

## Lazy GPU allocation

GPU textures are created on the **first `Draw` call**, not at construction. For timing-critical code, force early allocation:

```go
stimuli.PreloadVisualOnScreen(screen, stim)  // single
stimuli.PreloadAllVisual(screen, stims)      // batch
```

`BaseVisual` (embedded by most visual stimuli) provides no-op `Preload()` / `Unload()` and the position accessors.

## Visual stimuli

### Geometric shapes

**`PolyLine`** — Stroked poly-line, open or closed. `NewPolyLine(points, closed, lineWidth, color)`.
- The thick-stroke counterpart of `Line` and the *unfilled* counterpart of `Shape`. Use it for outlined polygons, open angles, arbitrary contours.
- `Points` are relative to the stimulus centre, **+Y up** (same convention as `Shape.Points`); `SetPosition` moves the whole figure without rewriting them. `Closed` connects the last point back to the first.
- One `RenderGeometry` call: a quad per segment plus a disc per vertex, giving **round joins and round caps**. Round joins matter for sharp angles — a mitre would shoot a long spike out of a 20° vertex.
- Vertex/index buffers are retained between `Draw` calls and only grow, so redrawing every frame allocates nothing after the first call. Safe inside a GC-disabled VSYNC loop.

> **`Line.LineWidth` is not honoured.** `Line.Draw` calls `RenderLine`, which is always 1 px, so `NewLine(a, b, c, 5)` silently draws a hairline. Use `PolyLine` for any stroke wider than one pixel.

## Audio stimuli

### Sound (WAV)

`Wait()` polls `Stream.Queued()` rather than sleeping a fixed duration — correctly handles resampling lookahead.

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

## VSYNC-locked animation loops

The `PresentMoving*` loops (dot cloud, grating, Gabor, grating disk) disable GC, drain stale events before the first frame, and return a `MotionResult`.

## RSVP / stream presentation

### Audio streams

Uses `time.Sleep(1ms)` polling (not VSYNC) for audio timing. `Sound` field in `SoundStreamElement` may be nil for silence.

### Mixed streams

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

## MPEG-1 video (with MP2 audio)

`Video` plays an MPEG-1 program stream (`.mpg` / `.mpeg`) decoded in pure Go by
`gen2brain/mpeg`. Unlike `GvVideo` it is wall-clock driven, not VSYNC-locked — it
drops frames under load rather than falling behind, so onset timing is only
approximate. For timing-critical stimuli prefer `.gv`.

**Audio is off until `PreloadDevice` is called.** Without it the video plays
silently and audio packets are discarded at the demuxer (leaving decoding enabled
with nothing draining the buffer would grow it for the length of the clip).
`PreloadDevice` is a no-op returning `nil` when the file has no audio stream, so
it is safe to call unconditionally; check `v.HasAudio` if you need to know.

`Update()` tops the SDL audio queue back up to 100 ms ahead each call. Audio is
free-running once queued, so it stays aligned with the video to within that depth.
Two consequences worth knowing:

- `Pause()` clears the queue so sound stops with the image; because that audio was
  already decoded, resuming leaves audio up to 100 ms ahead of the video.
- `Rewind()` clears the queue too, so a restarted clip does not play over its own
  tail.

## GV video

### High-level one-shot

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

## Key conventions

- Always call `sound.PreloadDevice(exp.AudioDevice)` before playing any `Sound` or `Tone`.
- After `sdl.AudioStream.PutData()`, call `Flush()` to emit resampling lookahead frames — omitting this causes truncated playback when WAV sample rate ≠ device rate.
- GC-disabling loops (`PresentStreamOfImages`, motion loops, `PlayGv`) restore GC via `defer`; do not call these functions from within another GC-disabled scope unless you manage restoration yourself.
- `spatialFreq` is **cycles per pixel** (e.g. 0.05 = one cycle per 20 px), NOT cycles per degree.
- `temporalFreq` is **Hz**.
- `orientation` is **degrees from horizontal** (0° = vertical bars drifting rightward).
