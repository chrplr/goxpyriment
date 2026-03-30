# Stroop Task

A classic cognitive interference paradigm (Stroop, 1935). Participants name the **ink colour** of colour words, ignoring the word's meaning. Responses are slower and more error-prone when the word and ink colour conflict (incongruent) than when they match (congruent).

---

## Trial structure

```
Fixation cross  →  Colour word  →  Response  →  ITI
    500 ms          until key       key press     500 ms
```

---

## Response keys

| Key | Ink colour |
|-----|------------|
| `R` | Red |
| `G` | Green |
| `B` | Blue |
| `Y` | Yellow |

---

## Design

- 4 ink colours × 4 word meanings = 16 combinations
- Congruent: word and ink match (e.g. RED written in red)
- Incongruent: word and ink conflict (e.g. RED written in blue)

---

## Prerequisites

- Go 1.25+

---

## Running

```bash
# Fullscreen
go run main.go

# Windowed (development / testing)
go run main.go -w
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file (CSV with a metadata header). One row per trial:

| Column | Description |
|--------|-------------|
| `trial` | Trial number |
| `word` | The displayed word |
| `ink_color` | The ink colour |
| `response` | Key pressed |
| `rt` | Reaction time in milliseconds |
| `correct` | Whether the response was correct |
| `congruent` | `true` if word meaning matches ink colour |

---

## References

Stroop, J. R. (1935). Studies of interference in serial verbal reactions. *Journal of Experimental Psychology*, 18(6), 643–662. https://doi.org/10.1037/h0054651
