# Dual `.gv` Video Player (demo)

A small demonstration of playing **two `.gv` videos side by side, synchronised**,
using the framework's `stimuli.GvVideo` type. It shows how to drive several video
streams from one `exp.Run` loop, letterbox each into a region of the screen,
overlay a fixation cross, and log keypresses with their time relative to video
onset.

`.gv` is goxpyriment's simple LZ4-compressed RGBA video format (see
`stimuli/CLAUDE.md` and `media/`).

---

## What it does

- Reads the **first two `.gv` files** found in the `assets/` folder.
- Plays them **synchronised**, each letterboxed into one half of the screen
  (left video in the left quarter, right video in the right quarter), with a
  white fixation cross drawn on top.
- Both videos start together; when both reach end-of-file the program exits.
- Every keypress is logged with a timestamp relative to when the videos started
  (reset by a Rewind).

### Controls

| Key | Action |
|-----|--------|
| `SPACE` | Pause / resume both videos |
| `R` | Synchronised rewind (both restart, onset clock resets) |
| `S` | Skip (end playback) |
| `ESC` | Quit |

---

## Assets

The demo needs **at least two `.gv` files** in `assets/`. This directory ships
with two symlinks to the framework's test videos:

```
assets/physical_val.gv -> ../../../tests/test_playgv/physical_val.gv
assets/wedges.gv       -> ../../../tests/test_playgv/wedges.gv
```

`.gv` files are excluded from git (`*.gv` in `.gitignore`), so if these are not
present on your machine, generate or copy two `.gv` files into `assets/`. Any two
`.gv` files will do — drop them in and they become the left/right pair.

---

## Prerequisites

- Go 1.25+
- Two `.gv` files in `assets/`

---

## Running

```bash
# Fullscreen
go run main.go

# Windowed (development / testing)
go run main.go -w
```

Run from **inside this directory** (the `assets/` folder is resolved relative to
the working directory).

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Subject ID (integer; only affects the data file name) |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | `-1` | Display ID: monitor index where the window/fullscreen opens (`-1` = primary) |

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file (with a `#`-prefixed metadata
header). One row per keypress:

| Column | Description |
|--------|-------------|
| `pair_index` | Always `1` (a single video pair is shown) |
| `video_left` | Filename of the left video |
| `video_right` | Filename of the right video |
| `key` | SDL keycode of the key pressed |
| `t_rel_ms` | Time of the press, in ms, relative to video onset (reset on Rewind) |

Timing here uses the millisecond Go clock — this is a playback demo, not an
RT-critical experiment.
