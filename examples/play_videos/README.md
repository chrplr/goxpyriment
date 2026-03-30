# Play Videos

Plays all `.mpg` video files found in an `assets/` subfolder sequentially. A keypress advances to the next video.

This example demonstrates video playback using the goxpyriment framework and can serve as a template for video-based tasks.

---

## Prerequisites

- Video files placed in `assets/*.mpg`

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
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |

---

## Controls

Press any key to advance to the next video. Press **Escape** or **Q** to quit.
