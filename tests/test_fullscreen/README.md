# Test Fullscreen

Opens an SDL3 window and reports the display resolution, refresh rate, and pixel density. A simple bouncing-ball physics animation runs to let you verify smooth rendering.

Use this to check your display setup before running timing-sensitive experiments.

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
| `-d` | off | Development mode: windowed 1024×768 |

---

## Controls

Press **Escape** or **Q** to quit.

---

## Note

This is a hardware verification utility. No data file is written.
