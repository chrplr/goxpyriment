# Visual Statistical Learning

Implements a visual statistical learning paradigm in which participants implicitly learn regularities in a stream of shapes. After a familiarization phase, their knowledge is probed with a choice or reaction-time test.

---

## Experiments

### Experiment 1A / 1B — Familiarization + two-alternative forced choice
Participants view a rapid stream of red and green shapes (interleaved). In the test phase they judge which of two sequences is more familiar.

### Experiment 2A / 2B — Familiarization + reaction-time test
Same familiarization; test phase measures reaction times to probes that are consistent or inconsistent with the learned statistics.

### Experiment 3 — Combined
Full design combining familiarization and both test types.

---

## Prerequisites

- Go 1.25+
- SDL3 development libraries (`sudo apt install libsdl3-dev` on Ubuntu/Debian)

---

## Running

```bash
# Experiment 1A, participant 1, fullscreen
go run main.go -exp 1A -s 1

# Windowed (development / testing)
go run main.go -exp 1A -s 1 -d
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-exp` | `1A` | Experiment variant: `1A`, `1B`, `2A`, `2B`, or `3` |
| `-s` | `0` | Participant ID (integer) |
| `-d` | off | Development mode: windowed 1024×768 |

---

## Output

Data are saved to `goxpy_data/` as a `.xpd` file (CSV with a metadata header). One row per trial:

| Column | Description |
|--------|-------------|
| `phase` | Experiment phase (familiarization / test) |
| `trial` | Trial number |
| `shape_idx` | Index of the presented shape |
| `color` | Shape colour (red / green) |
| `is_repetition` | Whether the shape repeated from the previous trial |
| `attended` | Whether attention was directed to this stream |
| `response_key` | Key pressed (test phase only) |
| `rt` | Reaction time in milliseconds |
| `hit` | Whether the response was correct |
