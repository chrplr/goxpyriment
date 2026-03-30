# Mouse Audio Feedback

An interactive demo that plays audio feedback in response to mouse clicks: a **ping** on left click and a **buzzer** on right click. The cursor position is tracked and displayed in real time.

Use this to verify that audio output is working correctly on your system, or as a template for mouse-driven experiments.

---

## Prerequisites

- Go 1.25+
- Working audio output (speakers or headphones)

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

## Controls

| Action | Sound |
|--------|-------|
| Left mouse button | Ping |
| Right mouse button | Buzzer |
| **Escape** / **Q** | Quit |

---

## Note

This is a demonstration. No data file is written.
