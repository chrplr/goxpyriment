# Parity Decision

A simple **even/odd decision** task on single digits (0–9). Each digit appears at the centre of the screen; the participant presses a key to indicate whether it is even or odd. Reaction times and accuracy are recorded.

This example is useful as a minimal working experiment and as a starting point for building more complex tasks.

---

## Trial structure

```
Fixation cross  →  Digit  →  Response  →  ITI
    500 ms        until key   key press    500 ms
```

---

## Response keys

| Key | Meaning |
|-----|---------|
| `F` | Even |
| `J` | Odd |

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
| `number` | The digit shown (0–9) |
| `key` | Key pressed |
| `rt` | Reaction time in milliseconds |
| `correct` | Whether the response was correct |
