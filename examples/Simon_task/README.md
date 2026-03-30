# Simon Task

Demonstrates the **Simon effect**: even when stimulus location is irrelevant to the task, responses are faster and more accurate when the stimulus appears on the same side as the required response (congruent) than on the opposite side (incongruent).

Participants identify the **colour** of a square (red or green) regardless of where it appears on screen (left or right).

---

## Trial structure

```
Fixation cross  →  Coloured square  →  Response  →  ITI
    500 ms           until response       key press    500 ms
```

---

## Response keys

| Key | Meaning |
|-----|---------|
| `F` | Red |
| `J` | Green |

---

## Design

- Two colours × two positions = 4 conditions
- Congruent: red square on left (F key = left hand) / green square on right (J key = right hand)
- Incongruent: colour and position do not match

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
| `color` | Stimulus colour (`red` / `green`) |
| `position` | Stimulus position (`left` / `right`) |
| `key` | Key pressed |
| `rt` | Reaction time in milliseconds |
| `correct` | Whether the response was correct |
| `congruency` | `congruent` or `incongruent` |

---

## References

Simon, J. R. (1969). Reactions toward the source of stimulation. *Journal of Experimental Psychology*, 81(1), 174–176. https://doi.org/10.1037/h0027448
