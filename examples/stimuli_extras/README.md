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
| `-s` | `0` | Participant ID (integer) |
| `-d` | off | Development mode: windowed 1024×768 |

---

## Note

This is a demonstration. No data file is written.
