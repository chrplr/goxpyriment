# Lexical Decision

A standard **lexical decision** task: participants decide as quickly as possible whether a string of letters is a real word or a non-word (pronounceable but meaningless letter string). Stimuli are read from a CSV file.

---

## Trial structure

```
Fixation cross  →  Letter string  →  Response  →  ITI
    500 ms         until response     key press     500 ms
```

---

## Response keys

| Key | Meaning |
|-----|---------|
| `F` | Word |
| `J` | Non-word |

---

## Input file format

Create a CSV file with two columns (no header):

```
item,category
table,word
flurp,nonword
chair,word
...
```

Place the file in the same directory as `main.go` or pass its path on the command line.

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
| `item` | The letter string shown |
| `category` | `word` or `nonword` (from input CSV) |
| `key` | Key pressed |
| `rt` | Reaction time in milliseconds |
