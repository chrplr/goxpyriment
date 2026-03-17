# Random-Dot Stereogram

Displays a **random-dot stereogram** (RDS) — a pair of dot patterns that, when fused binocularly (by crossing or diverging the eyes), reveals a 3-D shape that is invisible in either image alone.

This example demonstrates the `stimuli.RandomDotStereogram` stimulus type.

---

## Prerequisites

- Go 1.25+
- SDL3 development libraries (`sudo apt install libsdl3-dev` on Ubuntu/Debian)
- Binocular vision (stereopsis) required to perceive the depth effect

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

## Viewing instructions

Place the screen at a comfortable distance. Relax your eyes as if looking through the screen (diverge) or cross them slightly until the two dot patterns merge into one. A shape will appear to float in front of or behind the background.

---

## Note

This is a demonstration. No data file is written.
