# Canvas Demo

Demonstrates the `Canvas` stimulus: an off-screen drawing surface (400×400 pixels) on which arbitrary shapes, lines, and text can be drawn before the whole canvas is presented on screen in a single frame.

Use this as a reference when you need to compose complex multi-element stimuli.

---

## Prerequisites

- Go 1.25+

---

## Running

```bash
# Fullscreen
go run main.go

# Windowed (development / testing)
go run main.go -w
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID (integer) |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |

---

## Note

This is a demonstration. No data file is written. Press any key to exit.
