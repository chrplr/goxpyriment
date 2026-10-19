# Word superiority effect (Reicher, 1969)

A four-letter string is flashed for a few display frames and immediately
covered by a pattern mask. Two letters then appear, one above and one below the
position of one of the letters, and the participant reports which of the two
was shown (**↑ = top, ↓ = bottom**).

The string is either a **word** (`WORD`), an **anagram** of it that keeps the
critical letter in place (`ORWD`), or the critical **letter** alone (`___D`).
The alternatives are always the same two letters (`D` / `K`), and both of them
complete a word in the word condition (`WORD` / `WORK`) and a non-word in the
anagram condition. Knowing the word therefore cannot, by itself, tell the
participant which letter was there. The **word superiority effect** is the
higher accuracy for words than for single letters and anagrams.

```
  ITI (blank, 700 ms)
  fixation cross (500 ms)
  stimulus   N frames       W O R D        or  O R W D   or      D
  response   until key            K
                            ⊗ ⊗ ⊗ ⊗      ← X and O overlaid in every slot
                                  D
```

## Phases

1. **Practice**: 6 trials (2 items × 3 materials) at about 150 ms, with a
   green/red feedback dot.
2. **Staircase**: single random letters at a random slot with a random foil,
   as in Reicher's threshold sessions. A 1-up/3-down staircase on the number of
   frames (start ≈ 100 ms, step 1 frame, 10 reversals, at most 100 trials)
   converges near 79 % correct. The threshold is the mean of the last 6
   reversals, rounded to whole frames. Skipped with `-frames N`.
3. **Main task**: every item in all three materials (48 × 3 = 144 trials),
   shuffled, at the staircase duration, no feedback, a break every 48 trials.

Within each material × critical position, which member of the pair is the
target (`WORD` with D, or `WORK` with K) and whether the target is the top
alternative are each balanced 50/50. Neither a bias towards the more frequent
word nor a bias towards one key can produce the effect.

## Running

From the repo root:

```bash
go run ./examples/Reading_Word_Superiority_Effect -w -s 999             # English
go run ./examples/Reading_Word_Superiority_Effect -w -s 999 -lang fr    # French
go run ./examples/Reading_Word_Superiority_Effect -w -s 999 -frames 3   # no staircase
```

### Flags

Standard: `-w` (windowed), `-d N` (display), `-s ID` (subject).

| Flag | Default | Meaning |
|---|---|---|
| `-lang` | `en` | Stimuli and instructions: `en` or `fr` |
| `-frames` | 0 | Stimulus duration in frames; 0 = set it with the staircase |
| `-stim` | *(embedded)* | Stimulus CSV replacing the built-in list |
| `-fontsize` | 64 | Point size of the letters and the mask |
| `-fix` | 500 | Fixation cross duration, ms |
| `-iti` | 700 | Blank inter-trial interval, ms |
| `-break` | 48 | Main-task trials between breaks (0 = none) |

## Items

`stimuli_en.csv` and `stimuli_fr.csv` each hold 48 items, 12 per critical
position, in the format `word,pos,alt,anagram`:

```
word,pos,alt,anagram
WORD,4,K,ORWD
```

- **word** and the **alternative word** (the word with `alt` at `pos`) are both
  words. No word appears twice in a list, either as an item or as an
  alternative.
- **anagram** is a permutation of the word that keeps the critical letter at
  `pos` and moves every other letter where possible. Neither the anagram nor
  the anagram with `alt` is a word.

The lists were checked against the English dictionaries in `/usr/share/dict`
and hunspell, and against [Lexique 3.83](http://www.lexique.org) for French. The
French pairs are content words with a film-subtitle frequency of at least
8 per million in both members. The English items were chosen by hand for
frequency, since no frequency list was available offline. At startup the
program checks every row mechanically (letters, position, anagram, critical
letter kept) and stops on the first bad row.

## Output

One row per trial, from all three phases (`subject_id` is prepended):

| Column | Meaning |
|---|---|
| `phase` | `practice`, `staircase` or `main` |
| `trial` | running trial number over the session |
| `material` | `word`, `anagram` or `letter` |
| `item` | the base word of the item (empty in the staircase) |
| `displayed` | the 4 slots as shown, `_` = empty slot |
| `crit_pos` | 1-based position of the critical letter |
| `target`, `foil` | the letter that was shown, and the other alternative |
| `top_letter` | the alternative drawn above the mask |
| `frames` | requested stimulus duration in frames |
| `stim_dur_ms` | measured stimulus duration: mask onset − stimulus onset |
| `response` | `up` or `down` |
| `chosen` | the letter chosen |
| `correct` | `true` / `false` |
| `rt_ms` | response time from the onset of the alternatives |

The `-info.txt` file records the refresh rate, the frame duration, the
staircase reversals and threshold, the main-task duration, and the accuracy per
material.

## Departures from Reicher (1969)

- **One item per display.** There are no two-item conditions (two rows of four
  slots), and no precue condition (alternatives announced before the trial).
- **Duration.** Reicher ran three durations per participant, between their
  60 % and 90 % points for single letters (35–85 ms across participants). Here a single
  duration near 79 % is set by a staircase, and it is a whole number of display
  frames (16.7 ms steps at 60 Hz). On a fast display, or for a participant who
  needs less than one frame, the threshold hits the 1-frame floor. The program
  warns when that happens.
- **Mask.** An X and an O overlaid in each slot, after Reicher's field of
  overlapping typewritten Xs and Os, rather than a continuous noise field.

## Reference

Reicher, G. M. (1969). Perceptual recognition as a function of meaningfulness of
stimulus material. *Journal of Experimental Psychology, 81*(2), 275–280.

---

<!-- BEGIN:links -->
## Try it without building

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/Reading_Word_Superiority_Effect/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
