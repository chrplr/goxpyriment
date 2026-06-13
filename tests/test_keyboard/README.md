# test_keyboard

An interactive demonstration of every keyboard input method provided by the
goxpyriment framework. Run it once to get a hands-on feel for how each
function behaves before choosing the right one for your experiment.

## Running

```bash
go run examples/test_keyboard/main.go -w        # windowed mode (recommended)
go run examples/test_keyboard/main.go           # fullscreen
```

Press **ESC** at any time to quit.

## What it covers

| Section | Method | What it demonstrates |
|---------|--------|----------------------|
| 1 | `Wait()` | Block until any key; prints which key was pressed |
| 2 | `WaitKey()` | Block until F specifically; all other keys ignored |
| 3 | `WaitKeys()` | First of F/J within 3 s; shows the timeout case |
| 4 | `WaitKeysRT()` | Same plus RT in ms measured from the call site |
| 5 | `GetKeyEventTS()` | Fixation → GO signal; hardware-precision RT from stimulus onset |
| 6 | `GetKeyEventsTS()` | Simultaneous F+J press; shows inter-key lag in ms |
| 7 | `Check()` | Non-blocking poll inside a 5 s animation loop |
| 8 | `IsPressed()` | Hold-SPACE state polled at 50 ms intervals |
| 9 | `WaitKeyReleaseTS()` | Hold then release F; measures press duration in ms |

## Choosing the right method

| Situation | Recommended method |
|-----------|-------------------|
| "Wait for the participant to press any key to continue" | `WaitKey(K_SPACE)` or `Wait()` |
| Single-stimulus trial, approximate RT | `Show(stim)` + `WaitKeysRT(keys, timeout)` |
| Precise RT relative to stimulus onset | `ShowTS(stim)` + `GetKeyEventTS(keys, timeout)` |
| Multiple stimuli, RT relative to a specific one | `ShowTS(stim1)` + later `GetKeyEventTS(...)` — events are preserved in the queue |
| Detect simultaneous bilateral presses | `GetKeyEventsTS(keys, timeout)` |
| Check for a keypress without blocking (e.g. inside a draw loop) | `Check()` |
| Is a key held down right now? | `IsPressed(key)` |
| Measure how long a key was held | `GetKeyEventTS` (down) + `WaitKeyReleaseTS` (up) |

Always call `exp.Keyboard.Clear()` **before** presenting a stimulus to discard
stale events from the previous trial — but never after `ShowTS`, as the
participant may have already responded.
