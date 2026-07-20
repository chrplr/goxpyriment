# Simple Reaction Time

A minimal **simple reaction time** task. A fixation cross appears for 1000 ms, then a "GO" stimulus appears for 1000 ms, then the screen goes blank. The participant presses any key as fast as possible after the GO signal. RT is measured from fixation onset.

This example illustrates:

- Hardware-precision RT measurement using `exp.ShowTS` + `exp.Keyboard.GetKeyEventTS`: the key event is timestamped at hardware-interrupt time and preserved in the SDL event queue. This means a response made *during* the GO display is not lost — it sits in the queue with its original hardware timestamp, and `GetKeyEventTS` retrieves it immediately after the blank, giving the correct RT without any polling bias.
- Using `exp.Keyboard.Clear()` at the start of each trial to discard stale events from the previous trial.
- Converting nanosecond timestamps to milliseconds.
- Saving key code, key name, and RT to the data file.

---

## Trial structure

```
Clear queue  →  Fixation cross  →  GO stimulus  →  Blank  →  Response
                   1000 ms           1000 ms                   any key
```

RT is measured from fixation onset (`ShowTS`). A response pressed during the GO display (1000–2000 ms from fixation) is captured immediately; a response after the blank (> 2000 ms) causes `GetKeyEventTS` to block until the key is pressed.

---

## Running

```bash
# Fullscreen, participant 1
go run main.go -s 1

# Windowed (development / testing)
go run main.go -s 1 -w
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID (integer) |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display index (-1 = primary) |

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file. One row per trial (10 trials total):

| Column | Description |
|--------|-------------|
| `key` | SDL keycode of the key pressed |
| `keyname` | Human-readable key name (locale-aware, e.g. `"a"`, `"Return"`) |
| `rt` | Reaction time in milliseconds from fixation onset |
