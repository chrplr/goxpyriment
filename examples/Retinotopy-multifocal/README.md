# Multifocal Retinotopy

Multifocal retinotopic mapping after Kurki, Hyvärinen, Henriksson & Vanni,
*Dynamics of retinotopic spatial attention revealed by multifocal MEG*,
NeuroImage **263** (2022) 119643. 

This is the **mapping localizer only**. The task here is the central
fixation colour-change task, which keeps fixation and gives a
behavioural check that the subject was engaged.

Twenty-four regions of the visual field — three annuli crossed with eight
45° sectors — are stimulated **simultaneously**, each following its own
quasi-orthogonal binary on/off sequence. Because the sequences are
near-orthogonal, 24 region-specific responses can be deconvolved from a single
continuous recording of about two minutes, instead of the much longer runs a
travelling-wave design needs (compare [`../Retinotopy/`](../Retinotopy/)).

Stimulation is **pattern onset**, not pattern reversal: when a region is "on"
for a trial, its checkerboard appears abruptly at full contrast and then fades
linearly back to the mid-grey background over that trial.

## What is and is not implemented

The authors released no code, so the stimulus was rebuilt from the Methods
section and Figure 1B. Everything the paper states is fixed; everything it does
not state is a flag and is listed under [Assumptions](#assumptions) below.

## The stimulus

| Parameter | Value | Source |
|---|---|---|
| Regions | 3 annuli × 8 sectors = 24 | paper |
| Radii | 0.5°, 2.3°, 4.7°, 8.4° | paper |
| Sector boundaries | on the vertical and horizontal meridians | Fig. 1B |
| Checks per region | 4 radial × 4 angular | paper |
| Stimulation | pattern onset, contrast fades linearly to 0 | paper |
| Trial onset asynchrony | 217–317 ms, randomised | paper |
| Trials per run | 467 (one full sequence period) | see below |
| Sequences | quadratic-residue (Legendre), 24 cyclic shifts | paper |
| Lead-in | 20 full-field flashes | paper |
| Fixation task | cross turns green for 800 ms; press SPACE | paper (target), see below (timing) |

A run is 467 trials at a mean 267 ms ≈ **2.1 minutes**, plus the habituation
block.

## Usage

```bash
# From the repo root (go.work resolves the workspace).

# Check the design before anyone is scanned — no window, works over ssh.
go run ./examples/Retinotopy-multifocal -verify

# Look at the stimulus: the static, full-contrast dartboard. Any key exits.
go run ./examples/Retinotopy-multifocal -w -s 1 -fullfield

# A full run, fullscreen.
go run ./examples/Retinotopy-multifocal -s 1

# MEG: pulse TTL line 0 of a DLP-IO8 at every trial onset.
go run ./examples/Retinotopy-multifocal -s 1 -trigger dlpio8 -trigger-line 0

# fMRI: wait for the scanner pulse (key 't') before starting the run.
go run ./examples/Retinotopy-multifocal -s 1 -wait-trigger

# A short unattended run, for testing.
go run ./examples/Retinotopy-multifocal -w -s 999 -autostart -trials 40 -habituation 2
```

Besides the standard `-w`, `-d N` and `-s <id>`:

| Flag | Default | Meaning |
|---|---|---|
| `-verify` | — | Print the orthogonality report and exit, without opening a window |
| `-fullfield` | off | Show the static full-contrast dartboard and wait for a key |
| `-autostart` | off | Skip the instruction and end screens (unattended runs) |
| `-trials N` | 0 | Trials in the run; 0 = one full period (`p`) |
| `-habituation N` | 20 | Full-field flashes before the run |
| `-p N` | 467 | Sequence period; must be a prime ≡ 3 (mod 4) |
| `-shift N` | 0 | Cyclic shift between regions; 0 = `p / 24` |
| `-max-ecc D` | 8.4 | Outer radius in degrees of visual angle |
| `-seed N` | 1 | Seed for trial durations and target times |
| `-screen-width-cm` / `-viewing-distance-cm` | 30 / 50 | Used when the participant dialog is skipped (`-s` given, or in the browser) |
| `-wait-trigger` | off | Wait for the scanner pulse (key `t`) before the run |
| `-trigger NAME` | none | `none`, `dlpio8`, `dlpio20`, `ft232h`, `labjackt4`, `parport`, `megttl` |
| `-trigger-device S` | "" | Serial port or host, for the devices that need one |
| `-trigger-line N` | 0 | TTL line to pulse (0–7) |
| `-trigger-ms N` | 5 | Pulse width; must be shorter than the shortest trial |

Hardware triggers are desktop-only: `triggers/` does not build for the browser,
so it is behind a `!js` build tag and the browser build accepts only
`-trigger=none`.

## Data

One row per event, in `goxpy_data/Retinotopy-multifocal_sub-NNN_date-….csv`.

| Column | Description |
|---|---|
| `event_type` | `trial`, `habituation`, `fix_target`, or `response` |
| `trial` | Trial index (blank for targets and responses) |
| `onset_ms` | Flip time of the onset, in ms from the start of the run |
| `toa_frames` | Trial duration in whole video frames |
| `toa_ms` | The same duration in ms, at the measured refresh rate |
| `n_on` | How many of the 24 regions were stimulated |
| `regions` | 24 characters, `1` = on; region 0 first |
| `rt_ms` | For a response, ms since the last fixation target |

`onset_ms` comes from `Screen.FlipTS`, and response times from the SDL event
clock, so onsets and responses share one clock.

The companion `-info.txt` records the whole design: the sequence parameters and
their **measured** orthogonality, the achieved eccentricities, the refresh rate,
and a region table mapping each of the 24 indices to its radii and polar angles.
Analysis code should read the region table from there rather than hard-coding it,
because the design is rescaled if it does not fit the display.

## Verifying the design

The deconvolution is only well-conditioned if the 24 sequences are close to
orthogonal, so that is measured rather than assumed. `-verify` builds the
design and reports it:

```
sequence: quadratic-residue (Legendre), p=467, shift=19, regions=24, trials=467
sequence: on-fraction per region min=0.4989 max=0.4989 (expected ~0.5)
sequence: max |pairwise cross-correlation| at lag 0 = 0.002141 (expected 0.002141 = 1/p)
sequence: max |autocorrelation| over non-zero lags = 0.002141 (expected 0.002141 = 1/p)
OK: all correlations at the 1/p floor
```

For a prime *p* ≡ 3 (mod 4), the ±1-coded Legendre sequence has a periodic
autocorrelation of exactly −1 at every non-zero lag, so two distinct cyclic
shifts correlate at −1/*p*. The same four lines are written into every session's
`-info.txt`, and the run aborts if they are not met. `go test
./examples/Retinotopy-multifocal/` checks the same properties, plus the
geometry: the pattern's mean luminance, that regions do not overlap, that +Y is
up, and that checks alternate across the meridians.

## Assumptions

The paper does not state these; each is a flag, and each is recorded in the
data file.

1. **`p = 467`, shift 19.** The paper reports 469 trials per run but gives
   neither the prime nor the shift, and 469 = 7×67 is not prime. 467 is the
   nearest prime ≡ 3 (mod 4), and ⌊467/24⌋ = 19 spaces the 24 shifts evenly
   over one period.
2. **The fade lasts exactly one trial**, so a region reaches the background
   just as the next trial begins. The paper gives no fade duration.
3. **The fade is linear in RGB value, not in luminance.** It is implemented by
   modulating each region's texture alpha over the mid-grey background, where
   `a·checker + (1−a)·grey` makes alpha equal to Michelson contrast — but the
   blend happens in the GPU after gamma, so on a γ≈2.2 display the *luminance*
   contrast falls faster than linearly. Making it luminance-linear would
   require per-pixel CPU compositing on every frame; it is not done here.
4. **Fixation targets** last 800 ms (the paper's value) and are separated by
   3–8 s (not stated). Respond with SPACE.

## Implementation notes

The 24 regions are rasterised once at startup into 24 bounding-box-sized RGBA
textures, antialiased by 3×3 supersampling, with transparent pixels outside each
sector. Nothing is embedded and nothing is read from disk, so the example is
self-contained and runs in the browser.

Per frame the loop is 24 `SetAlphaMod` plus 24 `RenderTexture` calls and **no
CPU pixel work at all** — possible only because pattern-onset stimulation never
changes the pattern, only its contrast. Trial durations are whole frame counts
drawn from the measured refresh rate (13–19 frames at 60 Hz = 216.5–316.5 ms),
and each frame is one VSYNC-locked `Screen.FlipTS`.

The check phase is computed from *global* radial and angular indices, so the 24
regions form one continuous dartboard as in Figure 1B rather than 24
independently-phased patches.

## References

Kurki, I., Hyvärinen, A., Henriksson, L., & Vanni, S. (2022). Dynamics of
retinotopic spatial attention revealed by multifocal MEG. *NeuroImage*, 263,
119643. https://doi.org/10.1016/j.neuroimage.2022.119643

James, A. C. (2003). The pattern-pulse multifocal visual evoked potential.
*Investigative Ophthalmology & Visual Science*, 44(2), 879–890.

Vanni, S., Henriksson, L., & James, A. C. (2005). Multifocal fMRI mapping of
visual cortical areas. *NeuroImage*, 27(1), 95–105.

---

<!-- BEGIN:links -->
## Try it without building

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/Retinotopy-multifocal/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
