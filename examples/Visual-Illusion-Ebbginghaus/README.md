# Ebbinghaus Illusion — Dynamic Demo

An animated demonstration of the **Ebbinghaus (Titchener circles) illusion**: a central disk appears larger or smaller depending on the size of the surrounding circles, even when both central disks are physically identical.

This demo animates the surrounding circles so they continuously grow and shrink, making the perceptual size distortion easy to observe in real time.

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

This is a demonstration, not a data-collecting experiment. No output file is written.
