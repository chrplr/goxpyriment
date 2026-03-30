# Test Fullscreen

Opens an SDL3 window and reports the display resolution, refresh rate, and pixel density. A simple bouncing-ball physics animation runs to let you verify smooth rendering.

Use this to check your display setup before running timing-sensitive experiments.

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
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |

---

## Controls

Press **Escape** or **Q** to quit.

---

## Note

This is a hardware verification utility. No data file is written.
