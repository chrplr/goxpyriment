# Motion Blur & Phantom Array Demo

Demonstrates two perceptual effects caused by the interaction between eye movement and LCD display rendering, inspired by the [TestUFO](https://testufo.com/) battery:

- **Phantom Array** (Lane 1): Stare at the central fixation cross while a thin vertical bar moves across. The bar appears to split into ghost copies — an artefact of your visual system's temporal integration.
- **Retinal Blur** (Lane 2): Smooth-pursuit the co-moving green square. The bar behind it looks wide and smeared because your retina smears the image across photoreceptors while tracking.

**Sync-strobe mode** (toggle with `S`) draws the bar only on even frames (50 % duty cycle), sharpening the phantom effect.

**Measurement mode** (toggle with `M`) adds a static comparison rectangle whose width you adjust to match the perceived blur width, letting you record a quantitative estimate.

## Running

```bash
go run main.go          # fullscreen
go run main.go -w       # windowed
go run main.go -w -s 1  # windowed, subject ID 1
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |
| `-s` | `0` | Participant ID |

## Controls

| Key | Action |
|-----|--------|
| `S` | Toggle strobe mode |
| `M` | Toggle measurement mode |
| `↑` / `↓` | Velocity +/− 50 px/s (range 100–1500) |
| `←` / `→` | Bar width +/− 1 px (normal mode) or comparison width (measure mode) |
| Enter | Record perceived width (measurement mode) |
| Escape | Quit |

## Output

In measurement mode, each Enter press records a row to `goxpy_data/` with: velocity, actual bar width, strobe status, and perceived (matched) width.

No data file is written if measurement mode is never used.
