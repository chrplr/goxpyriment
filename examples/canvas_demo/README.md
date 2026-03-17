# Canvas Demo

Demonstrates the `Canvas` stimulus: an off-screen drawing surface (400×400 pixels) on which arbitrary shapes, lines, and text can be drawn before the whole canvas is presented on screen in a single frame.

Use this as a reference when you need to compose complex multi-element stimuli.

---

## Prerequisites

- Go 1.25+
- SDL3 development libraries (`sudo apt install libsdl3-dev` on Ubuntu/Debian)

---

## Running

```bash
# Fullscreen
go run main.go

# Windowed (development / testing)
go run main.go -d
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID (integer) |
| `-d` | off | Development mode: windowed 1024×768 |

---

## Note

This is a demonstration. No data file is written. Press any key to exit.
