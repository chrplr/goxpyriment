# Stimuli Extras

A showcase of advanced stimuli available in the goxpyriment framework, displayed sequentially:

| Stimulus | Description |
|----------|-------------|
| `VisualMask` | Random noise mask (used to terminate iconic memory) |
| `GaborPatch` | Sinusoidal luminance grating in a Gaussian envelope |
| `DotCloud` | Cloud of randomly positioned dots |
| `StimulusCircle` | Circle drawn with the stimulus API |
| `ThermometerDisplay` | Vertical bar gauge for rating scales |

Press any key to advance through each stimulus.

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
