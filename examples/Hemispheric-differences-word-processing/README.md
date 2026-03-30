# Hemispheric Differences in Word Processing

A lateralized recognition memory experiment investigating differences between the left and right cerebral hemispheres in processing written words.

**Study phase:** words are presented in the left visual field (LVF) or right visual field (RVF) — projecting initially to the right and left hemispheres respectively.

**Test phase:** words appear at the centre of the screen. Participants judge whether each word is **old** (seen during study) or **new**.

---

## Trial structure

### Study phase
```
Central fixation  →  Word (LVF or RVF)  →  ITI
```

### Test phase
```
Central fixation  →  Word (centre)  →  Old/New response  →  ITI
```

---

## Response keys

| Key | Meaning |
|-----|---------|
| `F` | Old (seen before) |
| `J` | New (not seen before) |

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
| `trial_index` | Trial number |
| `phase` | `study` or `test` |
| `word` | The presented word |
| `vf` | Visual field during study: `LVF` or `RVF` |
| `lag_tag` | Lag category between study and test |
| `lag_ms` | Time between study and test presentation (ms) |
| `key` | Key pressed |
| `rt` | Reaction time in milliseconds |
| `correct` | Whether the response was correct |
