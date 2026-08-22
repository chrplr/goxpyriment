# Letter Classification (Posner & Mitchell, 1967)

A replication of the classic **chronometric classification** task. Two letters are
shown side by side and the participant decides, as fast as possible, whether they
are **"same"** or **"different."** The twist is that the *definition of "same"*
changes with the **instruction level**, and each level requires a deeper stage of
processing. The central result is that reaction time **increases systematically**
from physical → name → rule identity, tracing the depth of processing needed to
classify each pair.

The letter set is `{A, B, C, E, a, b, c, e}` (vowels `A E a e`, consonants
`B C b c`).

---

## The three levels

One level is selected per session in the setup dialog:

| Level | "Same" means… | Example SAME | Example DIFFERENT | Typical RT (paper) |
|-------|---------------|--------------|-------------------|--------------------|
| **1 — Physical** | identical in shape *and* case | `A A`, `b b` | `A a`, `A B` | ~468 ms |
| **2 — Name** | same letter name, any case | `A A`, `A a`, `b B` | `A B`, `a C` | ~550 ms |
| **3 — Rule** | both vowels *or* both consonants | `A E`, `B C`, `A a` | `A B`, `E c` | ~700–900 ms |

The categories nest: every physical match is also a name match, and every name
match is also a rule match. A full study runs the same participants (or groups)
across all three levels and compares the RTs.

---

## Trial procedure

```
ITI (blank)   Letter pair            Response          Feedback
1000 ms       shown until response   "same"/"different" "Correct/Wrong  RT: … ms"
                                     (up to 5000 ms)    1200 ms
```

- The pair stays on screen until a response (or timeout at 5 s).
- Feedback after every trial shows correctness and the reaction time.
- Reaction time is measured from the pair's VSYNC onset timestamp to the key
  event's hardware timestamp (SDL-clock, sub-millisecond).

> **Note.** The original used a 10 s inter-trial interval for manual card
> handling; this computerised version uses 1 s.

---

## Trial deck

96 trials (close to the paper's 88-card deck), shuffled per session. Physically-
and name-identical pairs are over-sampled (×3), matching the paper's proportions:

| Pair type | Reps | Count |
|-----------|------|-------|
| Physically identical (`AA`, `bb` …) | 3 | 24 |
| Name-same, different case (`Aa`, `Bb` …) | 3 | 24 |
| Rule-same, different name (`AE`, `BC`, `bC` …) | 1 | 16 |
| Rule-different (one vowel + one consonant) | 1 | 32 |
| **Total** | | **96** |

---

## Prerequisites

- Go 1.25+

---

## Running

```bash
go run .
```

A **setup dialog** opens first and collects everything the session needs (there
are no command-line flags):

| Field | Choices |
|-------|---------|
| Subject ID | free text |
| Instruction level | Level 1 — Physical / Level 2 — Name / Level 3 — Rule |
| "Same" response key | `F = Same, J = Different` **or** `J = Same, F = Different` |
| Fullscreen | checkbox (unchecked → 1024×768 window) |

Run from the repository root (or from this directory — `go.work` resolves the
workspace either way).

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file, with the session metadata in a
companion `-info.txt`. One row per trial:

| Column | Description |
|--------|-------------|
| `trial` | Trial number (1-based) |
| `left` | Left letter shown |
| `right` | Right letter shown |
| `category` | Match type of the pair: `physical`, `name`, `rule`, or `different` |
| `expected` | Correct response *at the selected level*: `same` or `different` |
| `response` | Key pressed: `same`, `different`, or `timeout` |
| `rt_ms` | Reaction time in milliseconds (`0` on timeout) |
| `correct` | Whether the response was correct |

The chosen level and response mapping are recorded in the session metadata
(`-info.txt`) via the setup dialog fields.

---

## What to expect in the results

For each level, take the mean RT of correct **"same"** responses. Across sessions
run at different levels, RT should climb:

**physical < name < rule**

This ordering is the paradigm's signature: matching by physical form is fastest,
retrieving the letter's name is slower, and applying the vowel/consonant rule
(a conceptual judgement) is slowest.

---

## References

Posner, M. I., & Mitchell, R. F. (1967). Chronometric analysis of classification.
*Psychological Review*, 74(5), 392–409. https://doi.org/10.1037/h0024808

The original paper is included in this directory as a PDF.
