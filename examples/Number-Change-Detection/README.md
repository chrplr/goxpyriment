# Number Change Detection

Replication of the infant looking-time paradigm used by Decarli, Piazza & Izard (2023) to investigate whether infants' preferences in the number change detection task are driven by numerosity changes or by the regularity (pattern) of the changing sequence.

Infants are seated in front of a large projection screen. Two streams of dot arrays are presented simultaneously on the left and right. Each stream shows either 5 or 20 dots per image. The experimenter watches the infant and codes the gaze direction in real time by holding arrow keys; the program accumulates looking times and computes the log ratio of preference.

## Experiments

| Flag | Left vs. right stream | Purpose |
|------|-----------------------|---------|
| `preliminary` | Alternating vs. constant | Establish baseline preference for the changing stream |
| `exp1` | Random vs. constant | Test whether unpatterned changes alone attract attention |
| `exp2` | Alternating vs. random | Test whether the regular alternating pattern adds extra appeal |

## Stimulus streams

| Type | Description |
|------|-------------|
| Constant | All 48 images show the same numerosity (5 or 20) |
| Alternating | Images alternate systematically: 5–20–5–20… |
| Random | 12 images of 5 dots and 12 of 20 dots in an unpredictable order; repeated twice |

Each stream is a block of 24 unique images repeated twice, giving 48 images per trial (~40 s at 500 ms on / 300 ms blank).

## Non-numerical parameter controls

To prevent infants from responding to dot size or total area rather than numerosity, two conditions control the non-numerical parameters:

| Condition | What is equated |
|-----------|-----------------|
| Extensive | Total dot area and convex hull of the array |
| Intensive | Individual dot diameter and dot density |

Trials 1–2 use one condition; trials 3–4 use the other (see Counterbalancing).

## Usage

```bash
# Preliminary experiment — alternating vs. constant, windowed, subject 1
go run main.go -exp preliminary -w -s 1

# Experiment 1 — random vs. constant
go run main.go -exp exp1 -w -s 2

# Experiment 2 — alternating vs. random
go run main.go -exp exp2 -w -s 3
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-exp` | `preliminary` | Experiment type: `preliminary`, `exp1`, or `exp2` |
| `-w` | off | Windowed mode (1024×768) instead of fullscreen |
| `-d N` | −1 | Display index for the experiment window (−1 = primary) |
| `-s N` | `0` | Subject ID (controls counterbalancing) |

## Experimenter controls during each trial

| Key | Meaning |
|-----|---------|
| `SPACE` | Infant is looking at centre — start the stream (attractor phase) |
| `LEFT` (hold) | Infant looking at the left stream |
| `RIGHT` (hold) | Infant looking at the right stream |
| (neither) | Infant not attending / looking away |
| `ESC` | Abort session |

## Counterbalancing (automatic, by subject ID)

- **Side of stream A**: alternates across the 4 trials and across even/odd subject IDs.
- **Parameter condition**: trials 1–2 use one condition (intensive or extensive), trials 3–4 use the other; which comes first depends on `subjectID / 2 % 2`.
- **Constant-stream numerosity**: 5 dots for even subject IDs, 20 dots for odd.

The assignment is automatic; the experimenter is blind to which stream is on which side.

## Physical calibration

The paper used 68 × 51 cm images with a 43 cm gap on a large projection screen. The constant `pxPerCm` in `main.go` (default 9) converts physical dimensions to pixels. Adjust it to match your setup:

```go
const pxPerCm = 9.0 // pixels per centimetre — calibrate to your projector
```

At `pxPerCm = 9` the streams are approximately 612 × 459 px each, centred at ±499 px from screen centre, on a 1920 × 1080 logical canvas.

## Output data

Written to `goxpy_data/<subject_id>.csv`. One row per trial.

| Column | Description |
|--------|-------------|
| `trial` | Trial number (1–4) |
| `exp_type` | `preliminary`, `exp1`, or `exp2` |
| `param_cond` | `intensive` or `extensive` |
| `left_stream` | Stream type on the left |
| `right_stream` | Stream type on the right |
| `constant_num` | Numerosity of the constant stream (5 or 20) |
| `look_left_ms` | Cumulative looking time toward the left stream (ms) |
| `look_right_ms` | Cumulative looking time toward the right stream (ms) |
| `look_stream_a_ms` | Looking time toward stream A (primary stream) |
| `look_stream_b_ms` | Looking time toward stream B |
| `log_ratio_a_vs_b` | log(stream A / stream B); positive = preference for stream A |

The cumulative log ratio across all 4 trials (the primary analysis measure) is displayed at the end of the session.

## References

Decarli, G., Piazza, M., & Izard, V. (2023). Are infants' preferences in the number change detection paradigm driven by sequence patterns? *Infancy*, 1–12. https://doi.org/10.1111/infa.12505

Libertus, M. E., & Brannon, E. M. (2010). Stable individual differences in number discrimination in infancy. *Developmental Science*, 13(6), 900–906.

---

<!-- BEGIN:links -->
## Try it without building

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/Number-Change-Detection/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
