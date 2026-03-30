# Memory Span

Measures immediate serial-recall capacity (memory span) for digits, letters, or words using an **adaptive staircase**: the sequence length starts short and increases or decreases based on the participant's accuracy, converging on the longest sequence they can reliably recall.

---

## Task

A sequence of items is presented one at a time on screen. After the last item, the participant uses on-screen buttons (mouse clicks) to reproduce the sequence in order. The sequence length adapts trial by trial.

---

## Prerequisites

- Go 1.25+

---

## Running

```bash
# Fullscreen, participant 1
go run main.go -s 1

# Windowed (development / testing)
go run main.go -s 1 -w
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID (integer) |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file (CSV with a metadata header). One row per trial:

| Column | Description |
|--------|-------------|
| `trial` | Trial number |
| `type` | Item type (digit / letter / word) |
| `length` | Sequence length on this trial |
| `sequence` | The presented sequence |
| `response` | The participant's reproduced sequence |
| `correct` | Whether the sequence was reproduced correctly |
