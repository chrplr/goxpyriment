# Statistical Learning of Tone Sequences

Implementation of all three experiments from Saffran, Johnson, Aslin & Newport (1999), the study demonstrating that both adults and 8-month-old infants can segment continuous tone sequences using statistical regularities (transitional probabilities).

Participants are exposed to a continuous stream of pure tones.  The tones belong to a small vocabulary of 3-tone "words" — sequences whose within-word transitional probabilities are much higher than the transitional probabilities across word boundaries.  After exposure, tests reveal implicit knowledge of these statistical boundaries.

---

## Experiments

### Experiment 1 — Adults: Word vs Non-word (2AFC)

Adults listen to a continuous 21-minute tone stream (three 7-minute sessions with breaks), then complete a 36-trial two-alternative forced-choice (2AFC) test. Each test trial presents a **word** (trained sequence) against a **non-word** (sequence with TP = 0 in the trained language). Participants press **1** or **2** to indicate which sounded more familiar.

### Experiment 2 — Adults: Word vs Part-word (2AFC)

Identical to Experiment 1 except that the foil items are **part-words** — 3-tone sequences that spanned word boundaries during training (TP ≈ 0.14–0.15 at the junction) rather than completely novel sequences. This tests sensitivity to graded statistical structure.

### Experiment 3 — Infants: Head-Turn Preferential Listening (HPP)

8-month-old infants are exposed to a 3-minute tone stream. The test phase uses the **head-turn preferential listening** procedure: on each of 12 trials a side light blinks to attract the infant's attention, then either a word or a part-word is played repeatedly until the infant looks away for 2 consecutive seconds or 15 seconds of total looking has elapsed. Looking times are measured by the experimenter using the keyboard.

---

## Tone alphabet

All 12 tones are pure sine waves from the chromatic scale starting at middle C (C4).  Each tone lasts exactly **0.33 s** with a 10 ms linear ramp to suppress clicks; there is **no gap** between consecutive tones.

| Note | Frequency (Hz) |
|------|---------------|
| C    | 261.63 |
| C#   | 277.18 |
| D    | 293.66 |
| D#   | 311.13 |
| E    | 329.63 |
| F    | 349.23 |
| F#   | 369.99 |
| G    | 392.00 |
| G#   | 415.30 |
| A    | 440.00 |
| A#   | 466.16 |
| B    | 493.88 |

---

## Languages

### Experiments 1 & 2 (6-word languages)

| Language | Words (3-tone sequences) |
|----------|--------------------------|
| **1** | A–D–B, D–F–E, G–G#–A, F–C–F#, D#–E–D, C–C#–D |
| **2** | A–C#–E, F#–G#–E, G–C–D#, C#–B–A, C#–F–D, G#–B–A |

Language 1 and Language 2 share the same 11-tone alphabet but arrange tones into entirely distinct word structures; Language 2's words are non-words in Language 1 (TP = 0), enabling participant-level counterbalancing.

Within-word TP ≈ 0.64; across-boundary TP ≈ 0.14 (Experiment 1).

### Experiment 3 (4-word languages, infants)

| Language | Words |
|----------|-------|
| **1** | A–F–B, F#–A#–D, E–G–D#, C–G#–C# |
| **2** | D#–C–G#, C#–E–G, F–B–F#, A#–D–A |

Within-word TP = 1.0; part-word first-bigram TP ≈ 0.33.

---

## Design

### Exposure

| Experiment | Duration | Sessions |
|------------|----------|---------|
| 1 & 2 | 21 min total | 3 × 7 min, with break screens |
| 3 | 3 min | 1 session (180 word tokens) |

Words are concatenated in random order with no consecutive repetition.

### Test (Experiments 1 & 2)

- 36 trials: each of 6 words × each of 6 foils (1 rep)
- Word and foil order within each trial counterbalanced randomly
- 750 ms silence between the two items; 5 s ITI

### Test (Experiment 3)

- 12 trials: first 2 words × 3 reps + first 2 part-words × 3 reps
- Trials pseudo-randomised
- Test items repeated continuously (500 ms gaps) until trial ends

---

## Response keys

### 2AFC (Experiments 1 & 2)

| Key | Meaning |
|-----|---------|
| `1` | First sequence sounded more familiar |
| `2` | Second sequence sounded more familiar |

### HPP observer controls (Experiment 3)

| Key | Meaning |
|-----|---------|
| `SPACE` | Stage 1: infant fixates centre; Stage 2: infant turns to side |
| `SPACE` (during sound) | Toggle looking / not-looking state |
| `ESC` | Abort |

---

## Prerequisites

- Go 1.25+
- Audio output device

---

## Running

```bash
go run .
```

A graphical setup dialog opens first. Select the experiment (1 / 2 / 3), the language assignment (counterbalancing), and the subject ID.

### Command-line flags

| Flag | Default | Description |
|------|---------|-------------|
| `-w` | off | Windowed mode (1024×768) — useful for testing |
| `-d N` | -1 | Display ID: monitor index (-1 = primary) |

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file. Column layout depends on the experiment:

**Experiments 1 & 2:**

| Column | Description |
|--------|-------------|
| `trial` | Trial number (1–36) |
| `word_position` | Whether the word was item 1 or item 2 |
| `response` | Key pressed (1 or 2) |
| `correct` | `true` if response matched word position |

**Experiment 3:**

| Column | Description |
|--------|-------------|
| `trial` | Trial number (1–12) |
| `item_type` | `word` or `part-word` |
| `looking_time_ms` | Total looking time in milliseconds |

### Key analysis

- **Experiments 1 & 2:** proportion correct overall and per foil type; binomial test against 50 % chance.
- **Experiment 3:** paired t-test of looking time to words vs part-words.

---

## References

Saffran, J. R., Johnson, E. K., Aslin, R. N., & Newport, E. L. (1999). Statistical learning of tone sequences by human infants and adults. *Cognition*, 70(1), 27–52. https://doi.org/10.1016/S0010-0277(98)00075-4

Saffran, J. R., Aslin, R. N., & Newport, E. L. (1996). Statistical learning by 8-month-old infants. *Science*, 274(5294), 1926–1928. https://doi.org/10.1126/science.274.5294.1926
