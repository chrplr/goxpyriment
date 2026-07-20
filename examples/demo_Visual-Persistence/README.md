# Visual Persistence Illusion

Reproduces the **"Persistence of Vision"** illusion demonstrated at
[TestUFO.com/persistence](http://TestUFO.com/persistence).

## The illusion

A source image scrolls horizontally behind a black mask that only lets thin vertical
slits pass through.

* **Stare at the fixation cross** — you see only disconnected vertical strips of colour.
* **Track the yellow dot with your eyes** — your brain integrates the successive slit
  samples over time, and the full hidden image materialises.

## Usage

```bash
go run main.go [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-image <path>` | *(built-in)* | Image file to show (JPEG, PNG, BMP, …) |
| `-speed <px/s>` | `300` | Scroll speed in pixels per second |
| `-slit-width <px>` | `4` | Width of each visible slit in pixels |
| `-slit-gap <px>` | `36` | Black gap between slits in pixels |
| `-w` | | Windowed mode (1024 × 768) instead of fullscreen |
| `-d N` | | Display index (−1 = primary monitor) |

### Example with a custom image

```bash
go run main.go -image face.jpg -speed 250 -slit-width 3 -slit-gap 27
```

## Runtime controls

| Key | Action |
|-----|--------|
| `↑` / `↓` | Speed ± 50 px/s |
| `+` / `-` | Slit width ± 1 px |
| `[` / `]` | Gap − / + 4 px |
| `F` | Toggle: show full image (bypass slits) |
| `ESC` | Quit |

## Tips for the best illusion

* **Ratio** — start with slit width 2–4 px and gap 18–36 px (ratio ≈ 1:9).
  Wider slits reveal too much; narrower gaps make the image too visible without tracking.
* **Speed** — match a comfortable eye-tracking speed (200–400 px/s works well).
* **Image choice** — high-contrast images (bold text, faces, cartoons) survive the
  narrow slit much better than fine-detail photographs.
* **Instruction to viewers** — "keep your eyes on the gold dot and follow it smoothly
  from left to right".

## How it works

The rendering algorithm uses **vertical strip slicing**:

```
For every slit position X across the screen (stepping by slitWidth + slitGap):
    srcX  = (X − imageOffset) mod imageWidth   // tiled source lookup
    Draw image columns [srcX, srcX + slitWidth) → screen columns [X, X + slitWidth)
```

`imageOffset` increases by `speed × Δt` every VSYNC-locked frame, driving the
horizontal scroll. The modulo arithmetic tiles the source image seamlessly.
