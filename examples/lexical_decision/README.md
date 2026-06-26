# Lexical Decision

A standard **lexical decision** task: participants decide as quickly as possible whether a string of letters is a real word or a non-word (pronounceable but meaningless letter string). Stimuli are read from a CSV file, or from a built-in default list when none is given.

The session begins with a short **practice block** (8 items: 4 words + 4 pseudowords). Throughout the experiment, each response is followed by a coloured feedback dot at screen centre — **green** when correct, **red** when incorrect or too slow. Practice trials are not recorded. At the end, an on-screen **summary** reports the median reaction time for correct responses and the hit rate, separately for words and pseudowords.

---

## Trial structure

```
Counter "X/N"  →  Blank  →  Letter string  →  Response  →  Feedback dot
    500 ms        500 ms     until response     key press    800 ms (green/red)
```

---

## Response keys

| Key | Meaning |
|-----|---------|
| `F` | Word |
| `J` | Non-word |

---

## Input file format

A CSV file with a header row and three columns — the letter string, its category (`word` or `pseudo`), and the number of characters:

```
stimulus,category,length
CLOCK,word,5
FLURP,pseudo,5
CHAIR,word,5
...
```

### Default (embedded) stimuli

The file [`lexical_decision_stimuli.csv`](lexical_decision_stimuli.csv) is **embedded into the binary** (via `//go:embed`) and used automatically when **no CSV file is passed on the command line**. To run with a different list, pass your own CSV path as an argument (see below); it overrides the embedded default. Because the default is compiled in, the built binary runs standalone with no external stimulus file.

---

## Prerequisites

- Go 1.25+

---

## Running

Participant information (subject code, fullscreen toggle, etc.) is collected through a **graphical setup dialog** that opens before the experiment starts — there are no `-s`/`-w` flags.

```bash
# Use the embedded default stimulus list
go run main.go

# Use a custom stimulus list
go run main.go my_stimuli.csv
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-headless` | off | Skip the setup dialog and use cached/default field values |

The single optional positional argument is the path to a stimuli CSV file. When omitted, the embedded default list is used.

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file (CSV with a metadata header). One row per trial:

| Column | Description |
|--------|-------------|
| `item` | The letter string shown |
| `category` | `word` or `pseudo` (from input CSV) |
| `nchar` | the number of characters of the item (from input CSV) |
| `key` | Key pressed |
| `rt` | Reaction time in milliseconds |
