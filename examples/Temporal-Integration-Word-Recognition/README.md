# Temporal Integration in Visual Word Recognition

Replication of Experiments 1 and 2 from:

> Forget, J., Buiatti, M., & Dehaene, S. (2010). Temporal Integration in Visual Word Recognition. *Journal of Cognitive Neuroscience*, 22(5), 1054–1068. https://doi.org/10.1162/jocn.2009.21300

---

## Paradigm

A word is split into two interleaved components: its **odd-positioned letters** (1st, 3rd, 5th, 7th) and its **even-positioned letters** (2nd, 4th, 6th, 8th). The two components flash alternately on screen at a variable Stimulus Onset Asynchrony (SOA). Each component is displayed with spaces at the complementary positions so that the letters sit at the correct visual locations of the full word.

```
Even component:  " A P G E"   (positions 2,4,6,8 of CAMPAGNE)
Odd component:   "C M A N "   (positions 1,3,5,7 of CAMPAGNE)
Merged percept:  "CAMPAGNE"
```

At fast alternation rates the two components fuse into a single readable word. At slow rates they are perceived as two separate (shorter) strings.

---

## Hardware requirements

| Parameter | Value |
|---|---|
| Display refresh rate | 60 Hz |
| Flash duration | 1 frame (≈ 16.7 ms) |
| Viewing distance | 57 cm |
| Maximum stimulus size | 0.9° height × 4° width of visual angle |
| Font | Inconsolata, size 20 pt, white on black |

---

## Stimulus materials

Stimuli are drawn from `words.tsv`, a list of 3 656 French words with the following columns:

| Column | Description |
|---|---|
| `stimulus` | Word form (UTF-8, may include French accented characters) |
| `nblettres` | Number of letters (4 – 8) |
| `freqlemfilms2` | Log10 lemma frequency per million (film subtitles) |
| `ifreq` | Frequency quartile (1 = most frequent, 4 = least frequent) |
| `wtype` | Word type flag |
| `word` | Canonical word form |

Only words of length **4, 6, and 8** letters are used.

---

## Experiment 1 — Subjective Report

### Goal

Measure the boundary between temporal integration (one fused word percept) and temporal segregation (two separate component-word percepts) using subjective reports.

### Stimulus conditions

| Condition | Odd component | Even component | Merged string |
|---|---|---|---|
| `whole_word` | nonword | nonword | valid French word (6 or 8 letters) |
| `component_words` | valid word (4 letters) | valid word (4 letters) | nonword (8 letters) |
| `nonword` | nonword (4 letters) | nonword (4 letters) | nonword (8 letters) |

**Generation details:**
- *whole_word*: a 6- or 8-letter word is split at alternating positions; the two halves are never themselves words.
- *component_words*: two distinct 4-letter words are used as components; their 8-letter interleaved merge is verified to be absent from the lexicon.
- *nonword*: letters of existing 4-letter words are scrambled until neither the scrambled string nor its interleaved merge appears in the lexicon.

### Trial sequence

```
Fixation cross          1510 ms
┌─ repeat 3 times ──────────────────────────────┐
│  Even component        1 frame  (≈ 16.7 ms)   │
│  Blank (ISI)           variable                │
│  Odd component         1 frame  (≈ 16.7 ms)   │
│  Blank (ISI)           variable                │
└────────────────────────────────────────────────┘
Mask  (########)         1 frame
Response screen          until key press
```

### SOA levels

| SOA (ms) | ISI (frames) | ISI (ms) |
|---|---|---|
| 50 | 2 | 33 |
| 67 | 3 | 50 |
| 83 | 4 | 67 |
| 100 | 5 | 83 |
| 117 | 6 | 100 |
| 133 | 7 | 117 |

### Task and response

After the mask the participant presses a number key to report how many words they could read:

| Key | Meaning |
|---|---|
| `0` | No words |
| `1` | One word |
| `2` | Two words |

Both the main keyboard row and the numeric keypad are accepted.

### Trial count

20 stimuli per condition × 6 SOAs × 3 conditions = **360 trials** (randomised).

---

## Experiment 2 — Objective Lexical Decision

### Goal

Objectively measure the temporal integration threshold by having participants perform a speeded lexical decision. Unlike Experiment 1, the alternating sequence continues indefinitely until the participant responds.

### Stimulus factors

| Factor | Levels |
|---|---|
| SOA | 50, 67, 83, 100, 117, 133 ms |
| Word length | 4, 6, 8 letters |
| Lexicality | word, pseudoword |

**Pseudoword generation:** the first half of one word is concatenated with the second half of another word of the same length (cross-splicing). The resulting string is verified to be absent from the lexicon.

### Trial sequence

```
Fixation cross          1510 ms
┌─ repeat until response ───────────────────────┐
│  Even component        1 frame  (≈ 16.7 ms)   │
│  Blank (ISI)           variable                │
│  Odd component         1 frame  (≈ 16.7 ms)   │
│  Blank (ISI)           variable                │
└────────────────────────────────────────────────┘
```

### Task and response

Bi-manual lexical decision:

| Key | Hand | Meaning |
|---|---|---|
| `F` | Right index | WORD |
| `J` | Left index | NOT A WORD (pseudoword) |

**RT measurement:** response time is measured from the onset of the **first odd component** — the moment when all letters of the string have appeared on screen at least once.

### Trial count

10 stimuli per (length × lexicality) × 6 SOAs × 3 lengths × 2 lexicalities = **360 trials** (randomised).

---

## Usage

```bash
# Experiment 1 — fullscreen
go run main.go -exp 1 -s <subject_id>

# Experiment 2 — fullscreen
go run main.go -exp 2 -s <subject_id>

# Windowed mode (1024×768)
go run main.go -exp 1 -w
go run main.go -exp 2 -w
```

| Flag | Default | Description |
|---|---|---|
| `-exp` | 1 | Experiment number (1 or 2) |
| `-s` | 0 | Subject / participant ID |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |

---

## Output data

Results are written to an `.csv` file in the current directory (one file per subject).

### Experiment 1 columns

| Column | Description |
|---|---|
| `trial` | Trial number (1-based) |
| `condition` | `whole_word` / `component_words` / `nonword` |
| `soa_ms` | Stimulus Onset Asynchrony in ms |
| `length` | Merged string length in letters |
| `merged` | Full merged string |
| `odd` | Odd-component letters (no spaces) |
| `even` | Even-component letters (no spaces) |
| `response` | Participant response: `0`, `1`, or `2` |
| `rt_ms` | Response time from mask offset (ms) |

### Experiment 2 columns

| Column | Description |
|---|---|
| `trial` | Trial number (1-based) |
| `condition` | `word` / `pseudoword` |
| `length` | Merged string length in letters |
| `soa_ms` | Stimulus Onset Asynchrony in ms |
| `merged` | Full merged string |
| `odd` | Odd-component letters (no spaces) |
| `even` | Even-component letters (no spaces) |
| `response` | `word` or `pseudoword` |
| `correct` | 1 if response matches condition, 0 otherwise |
| `rt_ms` | Response time from onset of first odd component (ms) |

---

## Expected results

### Experiment 1

At short SOAs (50–67 ms) the two components fuse: participants predominantly report reading **one word** in the whole-word condition and **zero words** in the nonword condition. As SOA increases past ~80 ms the components segregate: reports of **two words** increase in the component-words condition and **one-word** reports drop in the whole-word condition. The transition occurs around **80 ms SOA**.

### Experiment 2

Both RT and error rate increase with SOA beyond the integration threshold (~80 ms), with a stronger word-length effect at slow SOAs (suggesting serial letter-by-letter processing once parallel integration fails). Pseudowords show a larger and earlier length effect than words, consistent with a higher integration threshold for less familiar letter strings.
