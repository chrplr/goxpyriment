# Tearing Test

Displays a full-height vertical white bar sweeping horizontally across a black screen. Use it to diagnose display synchronisation problems.

## Usage

```bash
go run main.go              # fullscreen, default parameters
go run main.go -w           # windowed mode (1024×768)
go run main.go -w 8 -v 1200 # 8 px wide bar at 1200 px/s
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-w` | 4 | Bar width in pixels |
| `-v` | 800 | Speed in pixels per second |
| `-w` | — | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |
| `-s` | — | Subject ID |

Runtime keys: **↑/↓** adjust speed · **←/→** adjust width · **ESC/Q** quit.

## How to interpret what you see

### Clean display (expected with VSYNC on)

The bar moves smoothly with sharp, unbroken edges. No horizontal splits or jumps are visible.

### Screen tearing

**What it looks like:** The bar appears cut into two (or more) horizontal segments that are offset from each other — a step or zigzag in what should be a straight vertical edge.

**What causes it:** The GPU finishes a new frame and starts writing it to the framebuffer while the monitor is still scanning out the previous frame. When the scan line crosses the region being overwritten, part of the screen shows the old frame and part shows the new one.

**How to fix it:**
- Enable VSYNC in your application or GPU driver settings.
- On Linux/X11, check your compositor settings; tearing is common with no compositor or with direct rendering.
- On Linux/Wayland, tearing is normally absent because the compositor always presents complete frames.

### Stuttering / jerky motion

**What it looks like:** The bar moves in irregular jumps rather than smoothly; it may appear to stall briefly then leap forward.

**What causes it:**
- Frame-rate drops below the monitor refresh rate (the FPS counter in the HUD will show this).
- Garbage collection pauses (should not happen here — GC is disabled in the animation loop).
- Thermal throttling or competing CPU/GPU load.
- VSYNC enabled but frame pacing uneven (common on some drivers when the render time is close to the frame budget).

**How to investigate:** Watch the FPS counter. If FPS is consistently near your monitor refresh rate (e.g. 60, 144) but motion still looks jerky, the issue is frame-pacing rather than throughput.

### Ghosting / blur

**What it looks like:** The trailing edge of the bar leaves a faint smear or shadow.

**What causes it:** Pixel response time of the panel (inherent to LCD technology, especially at low refresh rates or with overdrive disabled). This is a display hardware property, not a software issue.

### Flickering

**What it looks like:** The bar dims or disappears on some frames.

**What causes it:** The application is not delivering frames every refresh cycle, or the display connection (HDMI/DP cable or adapter) is marginal. Check the FPS counter and cable/adapter quality.
