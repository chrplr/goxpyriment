# One-Back Picture RSVP

A **one-back** working-memory task using **Rapid Serial Visual Presentation
(RSVP)**. A stream of pictures flashes by, and the participant presses **SPACE**
whenever an image is **identical to the one shown immediately before it**. It is a
continuous-performance test of visual short-term memory and sustained attention:
each item must be held just long enough to compare against the next.

The presentation loop is **VSYNC-locked with the garbage collector disabled**, so
image durations are frame-accurate and per-image onset timestamps are recorded on
the SDL hardware clock.

---

## Stimuli

- Every image in the **`images/`** subfolder is used (currently **148 pictures**,
  a random subsample of the [THINGS](https://things-initiative.org/) object-image
  database — see `images/README.md`).
- Formats: `.jpg`, `.jpeg`, `.png`, `.bmp`.
- Each image is scaled to fit inside a `size × size` box (default 400 px),
  preserving aspect ratio, and preloaded to the GPU before the stream starts.

To use your own images, just drop them into `images/`.

---

## Task & timing

```
image      blank      image      blank      image …
200 ms     100 ms     200 ms     100 ms     200 ms
└─────── 300 ms SOA ──────┘
```

- Each image is shown for **200 ms**, followed by a **100 ms** blank (a **300 ms**
  stimulus-onset asynchrony).
- Each unique image appears once, in a fresh random order. About **10 %** of the
  presentations are then made **immediate repeats** (a one-back target) by showing
  that image a second time in a row.
- The participant presses **SPACE** on a repeat. A response counts as a **hit** if
  it lands within the response window (default **1000 ms** from the repeat's
  onset).
- On a **miss** (no SPACE before the window closes), a **buzzer** sounds the
  instant the window lapses — the loop finalises each target as soon as its
  deadline passes.
- Targets are never placed at position 1, so the stream never opens on a repeat.

Press **ESC** (or close the window) to abort.

---

## Prerequisites

- Go 1.25+
- Audio output (for the miss buzzer)
- Images present in `images/`

---

## Running

```bash
# Fullscreen, participant 1
go run main.go -s 1

# Windowed (development / testing)
go run main.go -s 1 -w

# More repeats (20 %) and a tighter response window
go run main.go -s 1 -repeat 0.20 -window 800
```

Run from the repository root (or from this directory — `go.work` resolves the
workspace either way). The `images/` folder is resolved relative to the working
directory, so run from inside this example directory (or ensure `images/` is on
the working path).

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID (integer) |
| `-size` | `400` | Bounding-box side in px; images are scaled to fit (aspect-preserving) |
| `-repeat` | `0.10` | Proportion of presentations that are one-back repeats (0–1) |
| `-window` | `1000` | Response window in ms after a repeat's onset |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | `-1` | Display ID: monitor index where the window/fullscreen opens (`-1` = primary) |

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file (with a `#`-prefixed metadata
header). One row per presentation:

| Column | Description |
|--------|-------------|
| `position` | Position in the stream (1-based) |
| `filename` | Image file shown |
| `is_repeat` | `true` if this presentation is a one-back target (an immediate repeat) |
| `responded` | `true` if a SPACE press was attributed to this image |
| `rt_ms` | Reaction time from this image's onset to the press, in ms (`-1` if no press) |

Each SPACE press is credited as a **hit** to the open target window that contains
it; a press that matches no open target window is a **false alarm**, attributed to
the image on screen at press time. So a `responded = true` row is a hit when
`is_repeat = true` and a false alarm when `is_repeat = false`.

At the end, a hit / miss / false-alarm summary is printed to the terminal.

**Scoring:**

- **Hit** — `is_repeat = true` and `responded = true`.
- **Miss** — `is_repeat = true` and `responded = false` (buzzer sounded).
- **False alarm** — `is_repeat = false` and `responded = true`.

---

## References

Hebart, M. N., Dickter, A. H., Kidder, A., et al. (2019). THINGS: A database of
1,854 object concepts and more than 26,000 naturalistic object images. *PLOS ONE*,
14(10), e0223792. https://doi.org/10.1371/journal.pone.0223792

Images are a CC0 subsample from the THINGS initiative
(https://things-initiative.org/).
