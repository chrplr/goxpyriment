# Text Input Demo

Demonstrates the `TextInput` stimulus: a text box that collects keyboard input from the participant and returns it as a string when **Enter** is pressed.

This example prompts the participant for their name and displays it back on screen.

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

This is a demonstration. No data file is written.
