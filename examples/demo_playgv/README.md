# Test Play GV Video

Plays a `.gv` (GPU-friendly video) file using the `PlayGv` function. Use this to verify `.gv` playback on your hardware before embedding video in an experiment.

The `.gv` format stores frames as LZ4-compressed RGBA texture blocks with a seekable index, enabling ultra-low-overhead VSYNC-locked frame delivery.

## Prerequisites

- Go 1.25+
- A `.gv` file (see https://github.com/chrplr/images2gv to convert image sequences)

## Running

```bash
go run main.go -f assets/bonatti1.gv
go run main.go -w -f assets/bonatti1.gv   # windowed
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-f` | — | Path to the `.gv` file to play (required) |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |

## Note

This is a hardware verification utility. No data file is written. Press Escape to quit.
