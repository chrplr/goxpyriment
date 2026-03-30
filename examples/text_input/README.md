# Text Input Demo

Demonstrates the `TextInput` stimulus: a text box that collects keyboard input from the participant and returns it as a string when **Enter** is pressed.

This example prompts the participant for their name and displays it back on screen.

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

This is a demonstration. No data file is written.
