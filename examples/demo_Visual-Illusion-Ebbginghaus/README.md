# Ebbinghaus Illusion — Dynamic Demo

An animated demonstration of the **Ebbinghaus (Titchener circles) illusion**: a central disk appears larger or smaller depending on the size of the surrounding circles, even when both central disks are physically identical.

This demo animates the surrounding circles so they continuously grow and shrink, making the perceptual size distortion easy to observe in real time.

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

This is a demonstration, not a data-collecting experiment. No output file is written.
