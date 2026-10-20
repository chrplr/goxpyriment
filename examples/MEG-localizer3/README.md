# MEG Localizer 3: multifocal retinotopy with a bimodal two-button task

Multifocal retinotopic mapping after Kurki, Hyvärinen, Henriksson & Vanni,
*Dynamics of retinotopic spatial attention revealed by multifocal MEG*,
NeuroImage **263** (2022) 119643 (the **mapping localizer only**), with a
two-button task running throughout, in two modalities:

- **Visual.** The fixation cross is dark grey. Every 3–8 s it is replaced for
  800 ms by a **`<`** (press the **left** button) or a **`>`** (press the
  **right** button). The arrows have the cross's height (16 px), line width
  (3 px) and colour, so only the shape changes.
- **Auditory.** Every 10 ± 2 s, independently of the arrows, a spoken word is
  played: "Left!" (`sounds/left.wav`) asks for the **left** button, "Right!"
  (`sounds/right.wav`) for the **right** one.

Within each modality, exactly half the events ask for the left button and half
for the right, in random order. Any two task events are at least 2 s
apart, and a press counts only within 2 s of an event's onset, so every press
belongs to at most one event. The task keeps fixation and gives a behavioural
check that the subject was engaged in both modalities; it also yields
left/right motor responses and auditory responses for localisation.

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
| Visual task | cross replaced by `<` (left) or `>` (right) for 800 ms, every 3–8 s | duration from the paper's 800 ms target; arrows are this example's |
| Auditory task | `left.wav` (left) or `right.wav` (right), every 8–12 s | this example |

A run is 467 trials at a mean 267 ms ≈ **2.1 minutes**, plus the habituation
block.

## Usage

```bash
# From the repo root (go.work resolves the workspace).

# Check the design before anyone is scanned — no window, works over ssh.
go run ./examples/MEG-localizer3 -verify

# Look at the stimulus: the static, full-contrast dartboard. Any key exits.
go run ./examples/MEG-localizer3 -w -s 1 -fullfield

# A full run, fullscreen.
go run ./examples/MEG-localizer3 -s 1

# MEG: pulse TTL line 0 of a DLP-IO8 at every trial onset, lines 1-4 with
# the task events (the defaults), response box sending keys b and y.
go run ./examples/MEG-localizer3 -s 1 -ttl dlpio8 -left-key b -right-key y

# No photodiode square (it is on by default).
go run ./examples/MEG-localizer3 -s 1 -photodiode=false

# fMRI: wait for the scanner pulse (key 't') before starting the run.
go run ./examples/MEG-localizer3 -s 1 -wait-trigger

# A short unattended run, for testing.
go run ./examples/MEG-localizer3 -w -s 999 -autostart -trials 40 -habituation 2

# Hear the sounds often, to check them and the response scoring.
go run ./examples/MEG-localizer3 -w -s 999 -trials 100 -habituation 0 -sound-gap-min 3 -sound-gap-max 4
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
| `-seed N` | 1 | Seed for trial durations and task events (combined with the subject ID) |
| `-left-key K` / `-right-key K` | `left` / `right` | Keys sent by the two response buttons: `left`, `right`, `up`, `down`, `space`, `a`–`z`, `0`–`9` |
| `-sound-gap-min S` / `-sound-gap-max S` | 8 / 12 | Range of the gap between sound onsets, in s |
| `-screen-width-cm` / `-viewing-distance-cm` | 30 / 50 | Used when the participant dialog is skipped (`-s` given, or in the browser) |
| `-wait-trigger` | off | Wait for the scanner pulse (key `t`) before the run |
| `-ttl KIND[:PORT]` | none | TTL device for the per-trial markers: `dlpio8`, `dlpio20`, `megttlbox:/dev/ttyACM0`, `mmbts:/dev/ttyACM0`, `parallel`, `gpio`, `ft232h`, `labjackt4:HOST`, `null` |
| `-ttl-line N` | 0 | TTL line pulsed at every trial onset (0–7) |
| `-ttl-arrow-left N` / `-ttl-arrow-right N` | 1 / 2 | TTL line raised with the trial onset at which `<` / `>` appears |
| `-ttl-sound-left N` / `-ttl-sound-right N` | 3 / 4 | TTL line raised with the trial onset at which `left.wav` / `right.wav` starts |
| `-ttl-ms N` | 5 | Pulse width in ms; must be shorter than the shortest trial |
| `-photodiode` | on | Flash a white square in the top-left corner for one frame at every trial onset (habituation flashes included), drawn in the same flip as the onset; the square is black on every other frame, instructions included, because grey→white is too small a step for reliable photodiode detection. The TTL marker is raised when that flip returns, which is one to three frames *before* the square reaches the screen; the photodiode measures that lag. `-photodiode=false` disables it |
| `-photodiode-size N` | 100 | Side of the photodiode square, in pixels |
| `-marker-frames N` | 1 | Frames the square stays white from each onset |

The photodiode square and the TTL pulse are the library's onset marker
(`control.MarkerFlags`, `exp.Marker`). The pulse is raised with one `SetHigh`
right after the onset flip and lowered by a timer `-ttl-ms` later. Until
October 2026 the device was chosen with `-trigger NAME -trigger-device S
-trigger-line N -trigger-ms N`; `-trigger megttl -trigger-device /dev/ttyACM0`
is now `-ttl megttl:/dev/ttyACM0` (`megttl` and `parport` are still accepted as
names). A device that cannot be opened stops the program before any window
opens. Hardware triggers are desktop-only: the browser build refuses `-ttl`;
`-photodiode` works there.

**Task events on the trigger channel.** An arrow or a sound starts only
on a trial-onset flip, and its line is raised together with the trial line in
the same code (e.g. trial line 0 + `<` line 1 = code 3). That keeps the
photodiode flashing at trial onsets only, so it still times every trial, and
costs at most one trial (217–317 ms) of delay on a schedule of seconds; the
logged onset is exact. All five lines must be distinct. Responses are logged
in the CSV only, not sent as TTL.

**Sound onset.** A sound is queued with the audio device paused and released
when the onset flip returns, so it starts within one audio buffer of `onset_ms`;
the `-info.txt` records the software estimate of the output latency
(`audio_output_latency_ms`; 21.3 ms on PipeWire at 44.1 kHz on the development
machine). That estimate cannot see the DAC, the OS mixer or the air tube of an
MEG sound system: measure the real lag at the rig with a microphone on the
trigger channel and subtract it. Between sounds a silent tone is replayed every
2 s, because an audio sink left idle for ~5 s is suspended and the resume
swallows the start of the next sound.

Checked in software (windowed, 60 Hz, `-autostart -habituation 1 -trials 3
-ttl-line 2`, a logging stand-in device): at each of the 4 onsets the corner
was white for exactly one frame, line 2 was raised 2–21 µs after the onset
flip timestamp and lowered 5.2–5.5 ms later. Not measured on hardware.

## Data

One row per event, in `goxpy_data/MEG-localizer3_sub-NNN_date-….csv`.

| Column | Description |
|---|---|
| `event_type` | `trial`, `habituation`, `arrow_left`, `arrow_right`, `sound_left`, `sound_right`, `response`, or `miss` |
| `trial` | Trial index (blank for task events and responses) |
| `onset_ms` | Flip time of the onset, in ms from the start of the run |
| `toa_frames` | Trial duration in whole video frames |
| `toa_ms` | The same duration in ms, at the measured refresh rate |
| `n_on` | How many of the 24 regions were stimulated |
| `regions` | 24 characters, `1` = on; region 0 first |
| `modality` | `visual` or `audio`, for a task event and the responses and miss attributed to it |
| `side` | The button the event asks for: `left` or `right` |
| `button` | For a response, the button pressed |
| `outcome` | For a response: `correct`, `wrong` (other button), `extra` (a second press to the same event) or `false_alarm` (no event in the last 2 s); `miss` on a miss row |
| `rt_ms` | For a response attributed to an event, ms since that event's onset |

A task event shares its `onset_ms` with the `trial` row of the trial it starts
with. A `miss` row is written when the event's 2 s window closes with no press;
its `onset_ms` is the event's onset. The end of the `-info.txt` summarises
events, correct, wrong and misses per modality and side.

At the end of the run a result screen shows, for each modality and side
(`<`, `>`, the words "Left!" and "Right!"), the number of correct responses over the
number of events. An event counts as correct only if the first press in its
2 s window was the right button; a wrong button and a miss are both errors.
The screen stays up until **SPACE** is pressed — not any key, so a late press
on a response button cannot dismiss it. `-autostart` skips it.

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
./examples/MEG-localizer3/` checks the same properties, plus the
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
4. **Arrow targets** last 800 ms (the duration of the paper's colour-change
   target) and are separated by 3–8 s (not stated).
5. **Task-event spacing**: at least 2 s between any two events, and a 2 s
   response window. An arrow that falls due within 2 s of a sound waits
   for the next eligible trial onset, so the visual gaps are slightly longer
   than 3–8 s on average.
6. **Exact side balance.** The whole schedule is planned before the run (all
   trial durations are drawn up front; `schedule.go`), and each modality is
   then given equal numbers of left and right events, shuffled. If a modality
   has an odd number of events, its last one is dropped. The planned counts are
   in the `-info.txt` (`planned_events`). A run stopped early with ESC is
   balanced only up to where it stopped.

## Implementation notes

The 24 regions are rasterised once at startup into 24 bounding-box-sized RGBA
textures, antialiased by 3×3 supersampling, with transparent pixels outside each
sector. The two sounds are embedded (`//go:embed`), so the binary is
self-contained. To use other sounds, replace `sounds/left.wav` and
`sounds/right.wav` and rebuild.

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

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/MEG-localizer3/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
