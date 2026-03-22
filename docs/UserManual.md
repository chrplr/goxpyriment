# goxpyriment User's Manual

This manual explains the key concepts of the library. It assumes you have read the [Getting Started](GettingStarted.md) guide and want to understand the framework well enough to write experiments confidently.

---

## Table of Contents

1. [The Experiment Object](#1-the-experiment-object)
2. [The Run Loop and Error Handling](#2-the-run-loop-and-error-handling)
3. [The Coordinate System](#3-the-coordinate-system)
4. [The Rendering Model](#4-the-rendering-model)
5. [Timing Architecture](#5-timing-architecture)
6. [Input Handling](#6-input-handling)
7. [Data Collection](#7-data-collection)
8. [Stimuli: Lifecycle and Preloading](#8-stimuli-lifecycle-and-preloading)
9. [High-Precision Streams (RSVP)](#9-high-precision-streams-rsvp)
10. [Audio](#10-audio)
11. [Experimental Design and Randomization](#11-experimental-design-and-randomization)
12. [Animated Stimuli](#12-animated-stimuli)
13. [Putting It All Together](#13-putting-it-all-together)

---

## 1. The Experiment Object

Every experiment revolves around a single `*control.Experiment` value. It is a façade that owns the SDL window, renderer, default font, audio device, keyboard and mouse handlers, and data file. You never create these separately.

```go
exp := control.NewExperimentFromFlags("My Experiment", control.Black, control.White, 32)
defer exp.End()
```

`NewExperimentFromFlags` parses two optional command-line flags:

| Flag | Effect |
|------|--------|
| `-d` | Developer mode: opens a 1024×768 window instead of going fullscreen |
| `-s N` | Set subject ID to N (integer); written to the data file automatically |

`defer exp.End()` must appear immediately after construction. It saves the data file, releases fonts, destroys the window, and shuts down SDL — even if the experiment panics or returns early.

### Key fields

```go
exp.Screen       *io.Screen           // window + renderer
exp.Keyboard     *io.Keyboard         // keyboard input
exp.Mouse        *io.Mouse            // mouse input
exp.AudioDevice  sdl.AudioDeviceID    // SDL audio device for Sound/Tone
exp.Audio        *control.AudioManager // high-level audio helpers
exp.Data         *io.DataFile         // output data file
exp.Design       *design.Experiment   // block/trial structure
exp.SubjectID    int
exp.DefaultFont  *ttf.Font
```

---

## 2. The Run Loop and Error Handling

### The `exp.Run` wrapper

All experiment logic runs inside a callback passed to `exp.Run`:

```go
err := exp.Run(func() error {
    // your experiment here
    return control.EndLoop
})
if err != nil && !control.IsEndLoop(err) {
    log.Fatalf("experiment error: %v", err)
}
```

`exp.Run` does two important things:

1. **Ensures everything runs on the SDL main thread.** SDL requires that all rendering and event calls happen on the thread that created the window. `exp.Run` guarantees this.
2. **Installs a panic/recover guard.** When the participant presses ESC, the library immediately aborts the trial loop by calling `panic` with an internal sentinel value. `exp.Run` catches this panic, converts it back to an error, and returns it cleanly. Your data is saved.

### Returning from the callback

| Return value | Meaning |
|---|---|
| `control.EndLoop` | Normal exit; experiment is done |
| `nil` | Continue — call the callback again on the next frame |
| any other `error` | Propagated as the return value of `exp.Run` |

In a typical experiment you run the full trial loop inside a single callback invocation and return `control.EndLoop` at the end. The `nil` / "keep looping" pattern is used for frame-by-frame animation; in most cases you will never return `nil`.

### You don't need to check every error

Because pressing ESC triggers the panic/recover mechanism, any call that would have returned an error (e.g., `exp.Show`, `exp.Wait`, `exp.Keyboard.WaitKey`) will instead abort the loop immediately. The error never reaches your code.

This means the following two styles are both correct:

```go
// Style A — check errors explicitly (safer for library/production code)
if err := exp.Show(fixation); err != nil {
    return err
}
if err := exp.Wait(500); err != nil {
    return err
}

// Style B — omit error checks inside exp.Run (fine for experiment scripts)
exp.Show(fixation)
exp.Wait(500)
```

Style B works because if something goes wrong (ESC pressed, window closed), the panic/recover mechanism unwinds the call stack before your code can observe an error. Use Style A if you have cleanup that must run on abort; otherwise Style B keeps experiment scripts readable.

---

## 3. The Coordinate System

All stimulus positions use a **center-origin** system:

```
             +Y (up)
              |
   -X --------+-------- +X
              |
             -Y (down)
```

- `(0, 0)` is the screen center.
- Positive Y is **up** (opposite to SDL's default, which is top-down).
- Units are pixels at the logical resolution.

```go
// Center of screen
stimuli.NewTextLine("Hello", 0, 0, control.White)

// 200 px to the right
stimuli.NewCircle(30, control.Red).SetPosition(control.Point(200, 0))

// Bottom-left area
stimuli.NewTextLine("Score", -400, -250, control.White)
```

The conversion to SDL coordinates (top-left origin) is handled internally by `screen.CenterToSDL(x, y)`. You never call this yourself unless you are writing a custom stimulus type.

### Logical size

If you call `exp.SetLogicalSize(width, height)`, the coordinate system scales to that virtual resolution regardless of the actual window or screen size. This is useful for making experiments resolution-independent:

```go
if err := exp.SetLogicalSize(1920, 1080); err != nil {
    log.Printf("warning: %v", err)
}
// Now (960, 540) is the center-right area, regardless of actual screen size.
```

---

## 4. The Rendering Model

SDL uses a **double-buffered** rendering model. There is an off-screen backbuffer where you draw, and the visible display. You draw to the backbuffer; calling `screen.Update()` (also called a "flip") presents it to the screen, typically synchronized to the vertical retrace (VSYNC).

The three-step cycle for showing one stimulus is:

```go
exp.Screen.Clear()       // fill backbuffer with background color
myStim.Draw(exp.Screen)  // draw stimulus onto backbuffer
exp.Screen.Update()      // present to display (VSYNC-blocks)
```

`exp.Show(stim)` does all three in one call and is the right choice for single-stimulus presentations. For cases where you need to draw multiple stimuli simultaneously — so they appear on screen at the same time — call each `Draw` separately before the single `Update`:

```go
// Show fixation cross and stimulus simultaneously
exp.Screen.Clear()
fixation.Draw(exp.Screen)
targetCircle.Draw(exp.Screen)
exp.Screen.Update()
```

### The blank screen

`exp.Blank(ms)` clears the screen, flips it (showing a blank), and then waits:

```go
exp.Blank(1000) // 1-second blank inter-trial interval
```

This is equivalent to `exp.Screen.Clear() + exp.Screen.Update() + exp.Wait(ms)`.

### Never draw outside exp.Run

All rendering must happen inside the `exp.Run` callback — equivalently, on the SDL main thread. Drawing from a goroutine will silently do nothing or crash.

---

## 5. Timing Architecture

### Frame cadence

`screen.Update()` blocks until the display's vertical retrace (VSYNC). On a 60 Hz monitor this is ~16.67 ms; on a 120 Hz monitor ~8.33 ms. This is the fundamental clock of the visual display: every stimulus change is aligned to a frame boundary automatically.

To know your frame duration at runtime:

```go
frameDur := exp.Screen.FrameDuration()  // e.g., 16.666ms on a 60 Hz display
fmt.Printf("Frame: %.2f ms\n", frameDur.Seconds()*1000)
```

### `exp.Wait` vs `clock.Wait`

| Function | Pumps SDL events? | Detects ESC? | Use when |
|---|---|---|---|
| `exp.Wait(ms)` | Yes | Yes | Inside `exp.Run`, between stimuli |
| `clock.Wait(ms)` | No | No | Short busy-waits, inside animation loops |

Always use `exp.Wait` for inter-trial and inter-stimulus intervals — it keeps the OS responsive and responds to ESC. Use `clock.Wait` only inside VSYNC-locked loops that handle events themselves (streams, animation loops).

### Sub-millisecond timing is not guaranteed for `exp.Wait`

`exp.Wait` sleeps in 1 ms increments. For coarse delays (ISIs, fixation durations) this is perfectly fine. For frame-accurate stimulus timing — e.g., showing a stimulus for exactly 2 frames — use the stream functions described in [Section 9](#9-high-precision-streams-rsvp).

### Disabling garbage collection

Go's garbage collector can pause execution for several milliseconds at unpredictable times. The stream and animation functions disable it during the critical loop:

```go
old := debug.SetGCPercent(-1)
defer debug.SetGCPercent(old)
```

You do not need to do this yourself for ordinary trial loops; only for high-speed RSVP or animation. The stream and animation functions handle it automatically.

---

## 6. Input Handling

### Keyboard

The `exp.Keyboard` object provides blocking and non-blocking access:

```go
// Block until any key — returns the keycode
key, err := exp.Keyboard.Wait()

// Block until a specific key
err := exp.Keyboard.WaitKey(control.K_SPACE)

// Block until one of several keys, with optional timeout (−1 = no timeout)
// Returns 0 on timeout, sdl.EndLoop on ESC/quit
key, err := exp.Keyboard.WaitKeys([]control.Keycode{control.K_F, control.K_J}, 3000)

// Same, but also returns reaction time in milliseconds
key, rt, err := exp.Keyboard.WaitKeysRT([]control.Keycode{control.K_F, control.K_J}, 3000)

// Non-blocking poll — returns 0 if nothing pressed
key, err := exp.Keyboard.Check()

// Drain all pending key events (use before a new trial to discard stale presses)
exp.Keyboard.Clear()
```

Reaction time from `WaitKeysRT` is measured from the moment the call is made, not from when the stimulus was shown. To measure RT from stimulus onset:

```go
exp.Show(target)         // stimulus appears on this VSYNC
key, rt, _ := exp.Keyboard.WaitKeysRT(responseKeys, 3000)
// rt is ms from Show() return to keypress — includes ~1 frame of rendering latency
```

For maximum RT precision, record `clock.GetTime()` just after `exp.Screen.Update()` returns (the exact VSYNC moment) and compute the delta yourself.

### Mouse

```go
// Block until any button
btn, err := exp.Mouse.WaitPress()

// Non-blocking poll
btn, err := exp.Mouse.Check()

// Current cursor position in center-based coordinates
x, y := exp.Mouse.Position()

// Show / hide cursor
exp.Mouse.ShowCursor(false)
```

Button values: `control.BUTTON_LEFT`, `control.BUTTON_RIGHT`.

### Key codes

Key codes are re-exported in `control` for the most common experiment keys. For anything not listed there, use `go-sdl3/sdl` directly:

```go
import "github.com/Zyko0/go-sdl3/sdl"

key, _ := exp.Keyboard.Wait()
if key == sdl.K_A { ... }
```

---

## 7. Data Collection

### The `.xpd` file

`exp.Data` writes to a file named `<expname>_<subjectID>_<timestamp>.xpd` in `~/goxpy_data/`. The format is plain CSV with `#`-prefixed comment lines as a metadata header. It opens automatically on `Initialize()` and is flushed to disk when `exp.End()` is called (or when `exp.Data.Save()` is called explicitly).

### Declaring column names

Call this once, before the trial loop:

```go
exp.AddDataVariableNames([]string{"condition", "response", "rt_ms", "correct"})
```

This writes a `# VARIABLES` header line. **Subject ID is always prepended automatically** — do not include it in the name list.

### Adding rows

```go
exp.Data.Add(condition, response, rt, correct)
```

Fields are written in the same order as the variable names. Any type that has a meaningful `fmt.Sprint` representation works: `int`, `float64`, `bool`, `string`. The subject ID and a timestamp are prepended automatically. Fields containing commas or quotes are escaped.

### Example output

```
# EXPERIMENT: My Experiment
# SUBJECT: 3
# DATE: 2026-03-22T14:05:11
# VARIABLES: subject_id,condition,response,rt_ms,correct
3,congruent,F,412,true
3,incongruent,J,538,false
```

### Saving mid-experiment

For long experiments it is good practice to call `exp.Data.Save()` after each block. This flushes the buffer to disk so that data up to that point is not lost if the experiment crashes.

---

## 8. Stimuli: Lifecycle and Preloading

### GPU textures are lazily allocated

Creating a stimulus (e.g., `stimuli.NewTextLine(...)`) is cheap — it allocates a Go struct and stores the parameters, but does no GPU work. The SDL texture is created on the **first call to `Draw`**. This means:

- You can safely create all your stimuli before the run loop.
- The first presentation of each stimulus will be slightly slower due to texture upload.

For timing-sensitive presentations, preload explicitly:

```go
stimuli.PreloadVisualOnScreen(exp.Screen, myStim)
// or, for a slice:
stimuli.PreloadAllVisual(exp.Screen, []stimuli.VisualStimulus{stim1, stim2, stim3})
```

Call this after `exp.Run` starts (the renderer is ready), but before the critical section:

```go
err := exp.Run(func() error {
    // Preload everything while showing instructions
    exp.ShowInstructions("Loading, please wait...")
    stimuli.PreloadAllVisual(exp.Screen, stimSlice)

    // Now timing-sensitive trials
    for _, stim := range stimSlice { ... }
    return control.EndLoop
})
```

### Releasing textures

If you generate many stimuli dynamically (e.g., hundreds of unique text strings), call `stim.Unload()` after each trial to free the GPU memory:

```go
for _, word := range wordList {
    stim := stimuli.NewTextLine(word, 0, 0, control.White)
    exp.Show(stim)
    exp.Keyboard.Wait()
    stim.Unload()  // release GPU texture
}
```

For a fixed, small set of stimuli created once and reused throughout the experiment, you never need to call `Unload` — `exp.End()` handles cleanup.

### Writing a custom stimulus

Any type that implements `VisualStimulus` can be passed to `exp.Show`:

```go
type VisualStimulus interface {
    Present(screen *io.Screen, clear, update bool) error
    Preload() error
    Unload() error
    Draw(screen *io.Screen) error
    GetPosition() sdl.FPoint
    SetPosition(pos sdl.FPoint)
}
```

Embed `stimuli.BaseVisual` to get the position methods and no-op `Preload`/`Unload` for free, then implement only `Draw` and `Present`:

```go
type MyStimulus struct {
    stimuli.BaseVisual
    // your fields
}

func (m *MyStimulus) Draw(screen *io.Screen) error {
    // draw using screen.Renderer SDL calls
    return nil
}

func (m *MyStimulus) Present(screen *io.Screen, clear, update bool) error {
    return stimuli.PresentDrawable(m, screen, clear, update)
}
```

---

## 9. High-Precision Streams (RSVP)

For paradigms that present stimuli at high speed — RSVP, attentional blink, priming — the standard `exp.Show` / `exp.Wait` cycle is not precise enough. The stream functions provide frame-accurate presentation:

- GC is **disabled** for the duration of the stream.
- Every onset and offset is aligned to a **VSYNC boundary**.
- A `TimingLog` records the predicted and actual onset for each stimulus.
- All keyboard and mouse events that occur during the stream are captured.

### Text stream

The simplest entry point:

```go
words := []string{"CHAIR", "RIVER", "TIGER", "CLOCK", "STONE"}
on  := 150 * time.Millisecond
off :=  50 * time.Millisecond

events, logs, err := stimuli.PresentStreamOfText(
    exp.Screen, words, on, off,
    0, 0,          // center of screen
    control.White,
)
```

### Image / mixed stimulus stream

For images or any `VisualStimulus`:

```go
stims := []stimuli.VisualStimulus{pic1, fixation, pic2, fixation}

// Regular cadence (all stimuli share the same on/off duration)
elements := stimuli.MakeRegularVisualStream(stims, 100*time.Millisecond, 0)

// Irregular cadence (individual onset times and durations, in ms)
elements, err := stimuli.MakeVisualStream(stims, onsetMs, durationMs)

events, logs, err := stimuli.PresentStreamOfImages(exp.Screen, elements, 0, 0)
```

### Reading events from the stream

```go
for _, ev := range events {
    if ev.Event.Type == sdl.EVENT_KEY_DOWN {
        key := ev.Event.KeyboardEvent().Key
        t   := ev.Timestamp.Milliseconds()  // ms from stream start
        fmt.Printf("key %d pressed at %d ms\n", key, t)
    }
}
```

### Reading the timing log

```go
for i, l := range logs {
    jitter := (l.ActualOnset - l.TargetOn).Milliseconds()
    fmt.Printf("stimulus %d: target %d ms, actual %d ms, jitter %d ms\n",
        i, l.TargetOn.Milliseconds(), l.ActualOnset.Milliseconds(), jitter)
}
```

Jitter below ±1 frame (±8–17 ms depending on monitor) is normal and expected. Larger jitter indicates system load or GPU driver issues.

### Audio stream

The same model applies to sounds:

```go
// All tones must be pre-loaded before the stream
for _, t := range tones {
    t.PreloadDevice(exp.AudioDevice)
}

elements := stimuli.MakeRegularSoundStream(tones, 200*time.Millisecond, 100*time.Millisecond)
events, logs, err := stimuli.PlayStreamOfSounds(elements)
```

---

## 10. Audio

### Sounds from files or embedded bytes

```go
// From a file
snd := stimuli.NewSound("ping.wav")
snd.PreloadDevice(exp.AudioDevice)
snd.Play()
snd.Wait()   // block until playback finishes

// From embedded bytes (go:embed)
//go:embed assets/beep.wav
var beepWav []byte

snd := stimuli.NewSoundFromMemory(beepWav)
snd.PreloadDevice(exp.AudioDevice)
snd.Play()
```

### Procedural tones

```go
tone := stimuli.NewTone(1000.0, 200*time.Millisecond, 200) // 1 kHz, 200 ms, medium volume
tone.PreloadDevice(exp.AudioDevice)
tone.Play()
```

Volume is 0–255. Use `PreloadDevice` once; then call `Play` repeatedly.

### Segment playback with fade

Play only part of a longer sound, with smooth fade-in and fade-out:

```go
snd.PlaySegment(1.5, 3.0, 0.05) // play 1.5–3.0 s, 50 ms ramps
```

This is useful for stimuli like vowels extracted from longer recordings.

### Built-in feedback sounds

```go
exp.Audio.PlayBuzzer()   // play the embedded error/incorrect sound asynchronously
exp.Audio.PlayCorrect()  // play the embedded correct/reward sound asynchronously
```

Both return immediately; the sound plays in the background.

### Synchronous vs asynchronous

`snd.Play()` starts playback and returns immediately. `snd.Wait()` blocks until it finishes. For trial timing where you need to know when a sound ended, call both:

```go
snd.Play()
doSomethingElse()
snd.Wait()  // wait here if needed
```

For fire-and-forget feedback sounds, call `snd.Play()` without `snd.Wait()`.

---

## 11. Experimental Design and Randomization

### When to use `design.Block` / `design.Trial`

The `design` package provides a structured `Experiment → Block → Trial` hierarchy with string-keyed factors. Use it when:

- You have a multi-block experiment and want `exp.Design.Blocks` to drive the loop.
- You want between-subjects Latin-square counterbalancing (`AddBWSFactor`).

For simple single-block experiments, a plain Go slice of a custom struct is often more readable (and type-safe). Both approaches are valid.

### Building a design

```go
block := design.NewBlock("main")
for _, cond := range []string{"congruent", "incongruent"} {
    t := design.NewTrial()
    t.SetFactor("condition", cond)
    t.SetFactor("soa", 200)
    block.AddTrial(t, 20, false)  // 20 copies, appended in order
}
block.ShuffleTrials()
exp.AddBlock(block, 1)
```

Iterate at runtime:

```go
for _, blk := range exp.Design.Blocks {
    for _, trial := range blk.Trials {
        cond := trial.GetFactor("condition").(string)
        soa  := trial.GetFactor("soa").(int)
        // ...
    }
}
```

### Randomization helpers

The `design` package provides randomization functions that work on any slice type:

```go
// In-place shuffle (generic — works on any []T)
design.ShuffleList(mySlice)

// Random integer in [a, b] inclusive
design.RandInt(500, 1500)

// Random element from a slice
word := design.RandElement(wordList)

// Weighted coin flip
if design.CoinFlip(0.75) { /* 75% chance */ }

// Truncated normal sample in [a, b]
design.RandNorm(800.0, 1200.0)

// Shuffled integer range [first, last]
order := design.RandIntSequence(0, len(stimuli)-1)
```

### Between-subjects counterbalancing

```go
// Register once during setup
exp.AddBWSFactor("mapping", []interface{}{"F=left/J=right", "F=right/J=left"})

// At runtime — returns the condition assigned to this subject's ID
mapping := exp.GetPermutedBWSFactorCondition("mapping").(string)
```

The assignment follows a balanced Latin square so that conditions rotate across subjects (subject 1 → condition A, subject 2 → condition B, subject 3 → condition A, …).

### Constrained shuffling

For designs where you need to prevent the same condition from appearing more than N times in a row, use block-level `AddTrial` with `randomPosition: true`, which inserts each trial at a random position rather than appending:

```go
for _, cond := range conditions {
    t := design.NewTrial()
    t.SetFactor("condition", cond)
    block.AddTrial(t, 5, true)  // 5 copies each, randomly interleaved during construction
}
```

---

## 12. Animated Stimuli

Three functions run self-contained VSYNC-locked animation loops. All three:

- Disable GC for the duration of the loop.
- Drain the SDL event queue before the first frame.
- Return a `MotionResult` with the response key, mouse button, and reaction time.

```go
type MotionResult struct {
    Key    sdl.Keycode  // interrupt key pressed (0 if none)
    Button uint8        // mouse button (0 if none)
    RTms   int64        // ms from first frame to response; total elapsed on timeout
}
```

### Moving dot cloud

```go
result, err := stimuli.PresentMovingDotCloud(
    exp.Screen,
    100,                           // number of dots
    3.0,                           // dot radius (px)
    150.0,                         // cloud radius (px)
    control.Origin(),              // center at (0, 0)
    150.0,                         // speed in px/sec
    5000,                          // max duration ms (0 = infinite)
    []control.Keycode{control.K_SPACE}, // interrupt keys
    false,                         // catch mouse?
    control.White,                 // dot color
    control.Color{A: 0},           // background (transparent)
)
fmt.Printf("RT: %d ms\n", result.RTms)
```

### Drifting grating

```go
result, err := stimuli.PresentMovingGrating(
    exp.Screen,
    400, 400,              // width, height (px)
    control.Origin(),      // center
    0.0,                   // orientation (degrees; 0 = vertical bars drifting right)
    0.05,                  // spatial frequency (cycles per pixel)
    2.0,                   // temporal frequency (Hz)
    0.8,                   // contrast [0, 1]
    0.5,                   // mean luminance [0, 1]
    3000,                  // max duration ms
    []control.Keycode{control.K_SPACE},
    false,
)
```

### Drifting Gabor patch

```go
result, err := stimuli.PresentMovingGabor(
    exp.Screen,
    200,                   // bounding box size (px); at least 6×sigma
    30.0,                  // sigma (Gaussian SD in px)
    control.Origin(),      // center
    45.0,                  // orientation
    0.05,                  // spatial frequency (cycles/px)
    3.0,                   // temporal frequency (Hz)
    0.9,                   // contrast
    0.5,                   // mean luminance
    0,                     // max duration (0 = until response)
    []control.Keycode{control.K_SPACE},
    false,
)
```

---

## 13. Putting It All Together

Here is a skeleton that illustrates how the concepts compose in a realistic experiment:

```go
package main

import (
    "fmt"
    "log"

    "github.com/chrplr/goxpyriment/control"
    "github.com/chrplr/goxpyriment/design"
    "github.com/chrplr/goxpyriment/stimuli"
)

const (
    NReps           = 10
    FixationDuration = 500
    ResponseTimeout  = 3000
)

type trialDef struct {
    word  string
    color control.Color
    name  string
}

func main() {
    exp := control.NewExperimentFromFlags("Color Word Task", control.Black, control.White, 36)
    defer exp.End()

    exp.AddDataVariableNames([]string{"trial", "word", "ink", "key", "rt_ms", "correct"})

    // Build trial list
    words  := []string{"RED", "GREEN", "BLUE"}
    colors := []control.Color{control.Red, control.Green, control.Blue}
    names  := []string{"red", "green", "blue"}
    keys   := []control.Keycode{control.K_R, control.K_G, control.K_B}

    var trials []trialDef
    for _, word := range words {
        for j := range colors {
            trials = append(trials, trialDef{word, colors[j], names[j]})
        }
    }
    for i := 0; i < NReps-1; i++ {
        trials = append(trials, trials[:9]...)
    }
    design.ShuffleList(trials)

    // Preload stimuli
    fixation := stimuli.NewFixCross(20, 2, control.White)

    err := exp.Run(func() error {
        // Preload fixation cross (it will be used every trial)
        stimuli.PreloadVisualOnScreen(exp.Screen, fixation)

        exp.ShowInstructions(
            "Name the INK COLOR of each word.\n\n" +
            "R = Red   G = Green   B = Blue\n\n" +
            "Press SPACE to start.",
        )

        for i, t := range trials {
            // Fixation
            exp.Show(fixation)
            exp.Wait(FixationDuration)

            // Stimulus
            stim := stimuli.NewTextLine(t.word, 0, 0, t.color)
            exp.Show(stim)

            // Response
            key, rt, _ := exp.Keyboard.WaitKeysRT(keys, ResponseTimeout)

            // Score
            correct := false
            for j, k := range keys {
                if key == k && names[j] == t.name {
                    correct = true
                    break
                }
            }

            exp.Data.Add(i, t.word, t.name, key, rt, correct)
            fmt.Printf("trial %3d  %s/%s  rt=%dms  %v\n", i, t.word, t.name, rt, correct)

            if !correct {
                exp.Audio.PlayBuzzer()
            }

            // Release texture (new TextLine each trial)
            stim.Unload()

            exp.Blank(500)
        }

        return control.EndLoop
    })

    if err != nil && !control.IsEndLoop(err) {
        log.Fatalf("experiment error: %v", err)
    }
}
```

Key patterns illustrated:

- `NewExperimentFromFlags` + `defer exp.End()` at the top.
- `exp.AddDataVariableNames` before the run loop.
- `design.ShuffleList` for trial randomization.
- `stimuli.PreloadVisualOnScreen` inside `exp.Run` for timing-sensitive stimuli.
- `exp.Show` for single stimuli; `exp.Blank` for ITIs.
- `exp.Keyboard.WaitKeysRT` for response + RT.
- `stim.Unload()` for dynamically created stimuli.
- `exp.Audio.PlayBuzzer()` for feedback.
- `return control.EndLoop` at the end.

---

*For the complete function signatures and type definitions, see the [API Reference](API.md). For more worked examples, browse the `examples/` directory.*
