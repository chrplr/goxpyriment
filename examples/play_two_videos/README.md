# Play Two Videos

Plays pairs of video files side by side and records which key the participant presses after each pair. Videos are loaded from an `assets/` subfolder (`.mpg` files).

This example can serve as a template for video-based preference or recognition tasks.

---

## Prerequisites

- Go 1.25+
- SDL3 development libraries (`sudo apt install libsdl3-dev` on Ubuntu/Debian)
- Video files placed in `assets/*.mpg`

---

## Running

```bash
# Fullscreen, participant 1
go run main.go -s 1

# Windowed (development / testing)
go run main.go -s 1 -d
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID (integer) |
| `-d` | off | Development mode: windowed 1024×768 |

---

## Output

Data are saved to `xpd_results/` as a `.xpd` file (CSV with a metadata header). One row per video pair:

| Column | Description |
|--------|-------------|
| `pair_index` | Sequential pair number |
| `video_left` | Filename of the left video |
| `video_right` | Filename of the right video |
| `key` | Key pressed after the pair |
| `t_rel_ms` | Time from video end to key press (ms) |
