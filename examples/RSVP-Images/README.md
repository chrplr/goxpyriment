# RSVP Images — Hebart et al. (2023) oddball task

A replication of the rapid-serial-visual-presentation (RSVP) oddball-detection
task from **Hebart et al. (2023, THINGS-data)**, used to acquire MEG and fMRI
responses to thousands of object images.

Object images from the [THINGS database](https://things-initiative.org/) flash by
one after another on a mid-gray background with a central black fixation dot. The
participant keeps their eyes on the dot and presses **SPACE** as fast as possible
whenever an **oddball** appears. Oddballs are generated *at runtime* by pixelating
a few of the natural images, so no separate target images are needed.

---

## Trial structure

```
Image        →   Fixation (dot only)   →   next image …
500 ms           variable SOA
```

Two acquisition variants differ only in their stimulus-onset asynchrony (SOA):

| Variant | Image on | Fixation | SOA |
|---------|----------|----------|-----|
| **MEG**  | 500 ms | 1000 ± 200 ms | 1500 ± 200 ms (jittered) |
| **fMRI** | 500 ms | 4000 ms        | 4500 ms (fixed) |

Timings are **not** hardcoded — they are read from a design CSV (see below), so
the same program runs both variants.

---

## Two-step workflow

### 1. Generate a design CSV

`cmd/gen-design` scans `images/`, shuffles the order, marks a few images as
`catch` (oddball) trials, and writes one CSV per variant.

```bash
# from the repo root
go run ./cmd/gen-design \
    -images images \
    -out    .
```

This writes `design_meg.csv` and `design_fmri.csv` (both committed in this folder
as ready-to-use samples). Each has the columns:

| Column | Meaning |
|--------|---------|
| `onset` | Image onset time, seconds (cumulative) |
| `duration` | Image-on time, seconds (0.5) |
| `trial_type` | `exp` (natural image) or `catch` (pixelated oddball) |
| `file_path` | Path to the image, as resolved by the presentation program |

Generator flags: `-ncatch N` (number of oddballs, default 3), `-seed N`
(reproducible randomization), `-images`, `-out`.

### 2. Run the presentation

The design CSV is passed as the first positional argument. **Run it as a package**
(this example has more than one `.go` file, so `go run main.go` would miss
`pixelate.go`):

```bash
# from the repo root — MEG timing, windowed, subject 1
go run . -w -s 1 design_meg.csv

# fMRI timing
go run . -w -s 1 design_fmri.csv
```

Press **SPACE** on each pixelated oddball; **ESC** (or close the window) to abort.

---

## Flags (presentation program)

| Flag | Default | Description |
|------|---------|-------------|
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display index (monitor); -1 = primary |
| `-s N` | 0 | Subject ID |
| `-pixel N` | 16 | Block size for pixelating oddballs (larger = coarser) |
| `-maxpx N` | 900 | Downscale every image so its largest side ≤ N px (caps GPU memory) |
| `-dotsize R` | 5 | Radius of the central black fixation dot, in pixels |
| `-box N` | 600 | Bounding-box side in px; images scale to fit, preserving aspect ratio |

> **Why `-maxpx`?** The presentation preloads *all* images as GPU textures up
> front. The THINGS images are 1600×1600, so at native resolution 148 of them need
> ~1.24 GB of VRAM. Downscaling to the on-screen size (the native detail is never
> visible in the bounding box anyway) caps this at a few hundred MB — important on
> integrated graphics or a Raspberry Pi. Raise it for higher fidelity if you have
> the VRAM.

---

## Output

Data are saved to `goxpy_data/` as a `.csv` (plus a `-info.txt` metadata file).
The results CSV mirrors the input design plus a reaction-time column:

| Column | Description |
|--------|-------------|
| `subject_id` | Participant ID (prepended automatically) |
| `onset` | Image onset, seconds (from the design) |
| `duration` | Image-on time, seconds |
| `trial_type` | `exp` or `catch` |
| `file_path` | Image path |
| `reaction_time` | ms from the image's onset to the SPACE press attributed to it, or `n/a` |

Each SPACE press is attributed to the image whose onset most recently preceded it
(the first press per image wins); all other rows are `n/a`.

---

## Prerequisites

- Go 1.25+

---

## References

Hebart, M. N., Contier, O., Teichmann, L., et al. (2023). THINGS-data, a
multimodal collection of large-scale datasets for investigating object
representations in human brain and behavior. *eLife*, 12, e82580.
https://doi.org/10.7554/eLife.82580

Hebart, M. N., Dickter, A. H., Kidder, A., et al. (2019). THINGS: A database of
1,854 object concepts and more than 26,000 naturalistic object images. *PLOS ONE*,
14(10), e0223792. https://doi.org/10.1371/journal.pone.0223792
