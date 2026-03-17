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
- SDL3 development libraries (`sudo apt install libsdl3-dev` on Ubuntu/Debian)

---

## Running

```bash
# Fullscreen, participant 1
go run main.go -s 1

# Windowed (development / testing)
go run main.go -s 1 -d
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID (integer) |
| `-d` | off | Development mode: windowed 1024×768 |

---

## Output

Data are saved to `goxpy_data/` as a `.xpd` file (CSV with a metadata header). One row per trial:

| Column | Description |
|--------|-------------|
| `number` | The digit shown (0–9) |
| `key` | Key pressed |
| `rt` | Reaction time in milliseconds |
| `correct` | Whether the response was correct |
