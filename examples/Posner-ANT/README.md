# Posner Attention Network Task (vertical variant)

Implementation of Fan et al.'s **Attention Network Task** (ANT), adapted for a vertical layout: target arrows appear **above or below** the fixation cross rather than left/right, removing the Simon response-compatibility component and making the task easier for MRI and other constrained settings.

The task dissociates three attentional networks:

- **Alerting** — achieved by comparing *no-cue* vs *double-cue* trials
- **Orienting** — achieved by comparing *spatial-cue* vs *double-cue* trials
- **Executive control** — achieved by comparing *incongruent* vs *congruent* flanker trials

---

## Trial structure

```
Fixation + frames       →  Cue (optional)  →  Interval  →  Target  →  Response  →  Feedback
1500 – 3500 ms (jittered)     100 ms           400 ms       200 ms     up to 1900 ms   400 ms
```

On each trial a row of five arrows appears either above or below the fixation cross. The participant identifies the **direction of the central arrow**, ignoring the flankers.

### Cue types

| Alerting condition | What appears |
|--------------------|--------------|
| `no_cue` | Neither frame changes |
| `dbl_cue` | Both frames briefly brighten (alerting, no location information) |
| `spatial_cue` | One frame brightens — either the target frame (valid) or the other (invalid) |

---

## Response keys

| Key | Meaning |
|-----|---------|
| `F` | Central arrow points **left** ( `<` ) |
| `J` | Central arrow points **right** ( `>` ) |

Feedback is immediate: the fixation cross turns **green** for a correct response, **red** for an error.

---

## Design

72 trials in the main session (from `assets/trials.csv`), 24 in the training session (`assets/training.csv`). Trials are shuffled at the start of each run.

| Factor | Levels |
|--------|--------|
| Arrow direction | left, right |
| Flanker congruency | congruent (`< < < < <`), incongruent (`> > < > >`) |
| Alerting | no\_cue, dbl\_cue, spatial\_cue |
| Cue validity | NA, valid, invalid |
| Target position | up, down |

---

## Prerequisites

- Go 1.25+

---

## Running

```bash
go run .
```

A graphical setup dialog opens first. Fill in the participant information and choose the session type (**Training** or **Main experiment**), then click OK.

### Command-line flags

| Flag | Default | Description |
|------|---------|-------------|
| `-w` | off | Windowed mode (1024×768) — useful for testing |
| `-d N` | -1 | Display ID: monitor index (-1 = primary) |

The fullscreen / windowed choice can also be toggled in the setup dialog.

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file with a `#`-prefixed metadata header. One row per trial:

| Column | Description |
|--------|-------------|
| `arrow_direction` | Direction of the central arrow (`left` / `right`) |
| `flanker_congruency` | `cong` or `incong` |
| `alerting` | Cue condition (`no_cue`, `dbl_cue`, `spatial_cue`) |
| `cue_validity` | `valid`, `invalid`, or `NA` |
| `cue_up` | Whether the top frame was cued (`true` / `false`) |
| `cue_down` | Whether the bottom frame was cued (`true` / `false`) |
| `target_position` | Target location (`up` / `down`) |
| `response_key` | SDL keycode of the key pressed (0 = no response) |
| `reaction_time_ms` | RT in ms from target onset (0 = no response) |
| `correct` | Whether the response was correct (`true` / `false`) |

---

## References

Fan, J., McCandliss, B. D., Fossella, J., Flombaum, J. I., & Posner, M. I. (2005). The activation of attentional networks. *NeuroImage*, 26(2), 471–479. https://doi.org/10.1016/j.neuroimage.2005.02.004

Fan, J., McCandliss, B. D., Sommer, T., Raz, A., & Posner, M. I. (2002). Testing the efficiency and independence of attentional networks. *Journal of Cognitive Neuroscience*, 14(3), 340–347. https://doi.org/10.1162/089892902317361886
