# Random-Dot Stereogram

Displays a **random-dot stereogram** (RDS) — a pair of dot patterns that, when fused binocularly (by crossing or diverging the eyes), reveals a 3-D shape that is invisible in either image alone.

This example demonstrates the `stimuli.RandomDotStereogram` stimulus type.

---

## Prerequisites

- Go 1.25+
- Binocular vision (stereopsis) required to perceive the depth effect

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

## Viewing instructions

Place the screen at a comfortable distance. Relax your eyes as if looking through the screen (diverge) or cross them slightly until the two dot patterns merge into one. A shape will appear to float in front of or behind the background.

---

## Note

This is a demonstration. No data file is written.
