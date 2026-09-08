# Geometry consistency

Replication of the infant change-detection paradigm of Dillon, Izard & Spelke (2020), used to ask which geometric properties of 2D forms — **relative length** and **angle** — 7-month-old infants detect when figures are presented briefly and vary in size, position, orientation and sense.

Two streams of figures alternate simultaneously inside equal bounding panels on the left and right of a large screen. One stream changes in **shape and area**; the other changes in **area alone**. Longer looking at the shape-and-area stream indicates that the infant detected the shape change. An experimenter watches the infant and codes gaze direction in real time by holding the arrow keys.

## Conditions

Select with `-exp`, or from the setup dialog when no `-s` is given.

| Flag | Figures | Change tested | Rotation | Mirror flips |
|------|---------|---------------|----------|--------------|
| `1A` | Closed triangles, 15°-45°-120° vs 45°-60°-75° | Global shape | ±0–30° | no |
| `1B` | Same triangles | Global shape | 0–359° | yes |
| `2A` | Open 2-line figures, 106.77° | Relative length, 1:1.5 vs 1:3 | 0–359° | yes |
| `2B` | Open 2-line figures, 53.39° | Relative length, total line length equated | 0–359° | yes |
| `3A` | Open 2-line figures, sides 1×1.75 | Angle, 75.50° vs 151.00° (2-fold) | 0–359° | yes |
| `3B` | Open 2-line figures, sides 1×2.25 | Angle, 26.57° vs 116.57° (4.39-fold) | 0–359° | yes |
| `3C` | Open 2-line figures, sides 1×2.25 | Angle, 20.00° vs 137.50° (6.88-fold) | 0–359° | yes |
| `3D` | Same figures as 3A | Angle, 75.50° vs 151.00° (2-fold) | ±0–30° | no |

In the paper, infants detected the changes in 1A, 1B, 2A, 2B and 3D, but not in 3A–3C: angle is not detected over unlimited variation in orientation and sense, whereas relative length is.

## Stimulus streams

Both panels are in phase and both start on the **context** figure, then alternate every 0.8 s:

| Stream | Even presentations | Odd presentations |
|--------|--------------------|-------------------|
| Shape-and-area change | context figure | the other figure of the pair — different shape **and** ~2× the implied area |
| Area-only change | context figure | the *same* figure uniformly scaled to that same area change |

The implied area of an open figure is the area of the triangle closing its two endpoints (paper §2.5.1), which is how the paper equated area across streams.

Condition **2B** is the exception: its area-only partner is scaled to match the **total line-length** change of 1.5 units rather than the area (1×1.5 → 1.60×2.40, 1×3 → 0.625×1.875), which controls for total length instead.

### Per-presentation random variation

Applied independently to both panels on every presentation:

| Parameter | Range |
|-----------|-------|
| Position jitter | uniform within a 20 px radius of the panel centre |
| Scale | uniform ±0–15 % |
| Rotation | ±0–30° or 0–359°, per condition |
| Sense | 50 % left-right mirror, per condition |

Rotation, scaling and mirroring are about the **centroid of the figure's vertices**, which is also the point placed at the jittered location. The paper does not specify a reference point; this is our choice.

The stroke width is a **fixed** number of pixels and is deliberately not scaled with the figure — a stroke that grew with the figure would itself signal the area change and confound the area-only stream.

### Bounding rectangles

Each stream is presented "within a bounding rectangle" (paper §2), and it is that rectangle's centre the 20 px position jitter is measured from. The two square panels are drawn as thin grey outlines, as in the paper's Fig. 2b, and stay up for the whole trial — including the 300 ms blank between figures, since they are the spatial frame the figures appear in rather than part of the alternating stimulus. Set `-panel-stroke-px 0` to hide them and present the figures on a bare field.

### Figure geometry

Figures are defined in normalized units with the implied areas printed in the paper's Figures 2a, 4 and 6, then scaled to pixels by a single factor, so the area ratios the design depends on hold at any display size.

Triangle side lengths are **derived from the interior angles** by the law of sines rather than copied from the paper's two-decimal printed values. This keeps the angles exact and the area ratio exactly 2. It reproduces the 15°-45°-120° triangle to the printed precision (0.559 / 1.528 / 1.871 against 0.56 / 1.51 / 1.85), and lands within ~3 % on the 45°-60°-75° one, whose printed sides (1.00 / 1.06 / 0.77) in fact describe a 43.8°-64.0°-72.3° triangle with an area ratio of 2.06.

All open-figure conditions reproduce the paper's printed side lengths and areas to the printed precision. The angle conditions have an implied-area ratio slightly off 2 (1.997 for 3A/3D, 2.000 for 3B, 1.975 for 3C) because the paper fixes the angles, not the areas; the area-only scale is derived from the actual areas so that the **area change is equated across the two streams**, as the design requires.

## Usage

Run from the repo root so `go.work` resolves the workspace. This package has two `.go` files, so name the package — `go run main.go` would compile only half of it.

```bash
# Condition 1A, windowed, subject 1
go run ./examples/Geometry-consistency -exp 1A -w -s 1

# Condition 2B, fullscreen on the second monitor
go run ./examples/Geometry-consistency -exp 2B -d 1 -s 7

# Short trials, for checking the display
go run ./examples/Geometry-consistency -exp 3C -w -s 999 -trial-secs 8
```

Launching with no `-s` opens the setup dialog, which includes a condition selector.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-exp` | `1A` | Condition: `1A`, `1B`, `2A`, `2B`, `3A`, `3B`, `3C`, `3D` |
| `-cb N` | −1 | Counterbalancing group 0–3 (−1 = derive from the subject ID) |
| `-trials N` | `4` | Number of trials |
| `-trial-secs` | `60` | Duration of each trial, in seconds |
| `-panel-px` | `900` | Side of each square bounding panel, in logical pixels |
| `-unit-px` | `0` | Pixels per normalized figure unit (0 = fit the panel automatically) |
| `-stroke-px` | `6` | Figure stroke width, in logical pixels |
| `-panel-stroke-px` | `2` | Outline width of the bounding rectangle around each stream (0 = none) |
| `-jitter-px` | `20` | Radius of the random position jitter, in logical pixels |
| `-attractor-sound` | (embedded ping) | WAV file played with the attractor — e.g. a rattle |
| `-w` | off | Windowed mode (1024×768) instead of fullscreen |
| `-d N` | −1 | Display index (−1 = primary) |
| `-s N` | `0` | Subject ID (drives counterbalancing; skips the dialog) |

Layout is computed on a fixed 1920×1080 logical canvas, so every pixel value above is independent of the real display resolution. `-unit-px` defaults to the largest value that keeps the biggest figure of the chosen condition inside its panel at maximum scale and position jitter.

## Experimenter controls

| Key | Meaning |
|-----|---------|
| `SPACE` | Infant is looking at the centre — start the trial (attractor phase) |
| `LEFT` (hold) | Infant looking at the left panel |
| `RIGHT` (hold) | Infant looking at the right panel |
| (neither) | Infant not attending / looking away |
| `ESC` | Abort the session |

Between trials a large pulsing **pink dot** appears at screen centre, equidistant from the two panels, with a repeating sound, to recentre the infant's gaze. Supply a rattle with `-attractor-sound rattle.wav`; without it the embedded ping is used.

Looking times are measured from the SDL **hardware timestamps** of the key presses, not from wall-clock polling, so the coder's resolution is the keyboard's, not the frame rate's.

## Counterbalancing

Two binary factors, encoded together as group 0–3 and derived from the subject ID unless `-cb` overrides:

- **Context figure** — half the infants see the smaller-area figure of the pair as the context, half the larger one (`subject % 2`).
- **Starting side** — the side of the shape-and-area stream on trial 1 (`subject / 2 % 2`). It alternates across the 4 trials, so that stream appears twice on each side.

The experimenter is blind to which stream is on which side.

## Output data

Two files under the results directory (`goxpy_data/` by default), plus the usual `-info.txt` companion carrying the session metadata, the full condition description and the frame plan.

**`Geometry-consistency_sub-NNN_date-….csv`** — one row per trial.

| Column | Description |
|--------|-------------|
| `trial` | Trial number |
| `condition` | `1A` … `3D` |
| `cb_group` | Counterbalancing group 0–3 |
| `context_figure` | ID of the context figure |
| `change_figure` | ID of the shape-and-area change figure |
| `area_only_scale` | Scale applied to the context figure for the area-only stream |
| `shape_side` | Side showing the shape-and-area stream (`left` / `right`) |
| `look_left_ms`, `look_right_ms` | Cumulative looking time per side |
| `look_shape_and_area_ms`, `look_area_only_ms` | The same, resolved by stream |
| `proportion_shape_and_area` | shape / (shape + area); > 0.50 = shape change detected |
| `left_press_durations_ms`, `right_press_durations_ms` | Individual look durations, `;`-separated |

**`Geometry-consistency_sub-NNN_date-…-frames.csv`** — one row per presentation per panel: `subject_id, trial, slot, side, stream, figure_id, onset_ns, rotation_deg, scale, flip, jitter_dx_px, jitter_dy_px`.

`onset_ns` is the VSYNC-stamped flip timestamp, on the same SDL clock as the key events in the trial file, so gaze and stimulus can be aligned directly offline.

The proportion pooled over all trials — the paper's primary measure — is shown on the summary screen at the end of the session.

## References

Dillon, M. R., Izard, V., & Spelke, E. S. (2020). Infants' sensitivity to shape changes in 2D visual forms. *Infancy*, 25(5), 618–639. https://doi.org/10.1111/infa.12343

Ross-Sheehy, S., Oakes, L. M., & Luck, S. J. (2003). The development of visual short-term memory capacity in infants. *Child Development*, 74(6), 1807–1822. — the change-detection paradigm this method follows.

Libertus, M. E., & Brannon, E. M. (2010). Stable individual differences in number discrimination in infancy. *Developmental Science*, 13(6), 900–906.

---

<!-- BEGIN:links -->
## Try it without building

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/Geometry-consistency/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
