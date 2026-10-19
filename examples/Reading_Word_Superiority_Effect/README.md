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
2. **Staircase**: single letters, each taken from a random item of the list:
   its critical letter pair, at its critical position, either member shown
   (`___D` with D / K). A 1-up/3-down staircase on the number of
   frames (start ≈ 100 ms, step 1 frame, 10 reversals, at most 100 trials)
   converges near 79 % correct. The threshold is the mean of the last 6
   reversals, rounded to whole frames. Skipped with `-frames N`.
3. **Main task**: every item in all three materials (48 × 3 = 144 trials),
   shuffled, at the staircase duration, no feedback, a break every 48 trials.

Within each material × critical position, which member of the pair is the
target (`WORD` with D, or `WORK` with K) and whether the target is the top
alternative are each balanced 50/50. Neither a bias towards the more frequent
word nor a bias towards one key can produce the effect.

## Predictions

The dependent variable is **accuracy** (equivalently, the error rate) in the
main task. Chance is 50%: two alternatives, one of them correct.

### Errors

1. **Word < letter.** Fewer errors on a letter inside a word than on the same
   letter presented alone. This is the word superiority effect proper, the
   contrast Reicher's argument rests on, replicated by Wheeler (1970). A word adds three letters of
   potential interference, and the alternatives are chosen so that word
   knowledge cannot pick between them, yet the word wins. A serial
   letter-by-letter account predicts the opposite.
2. **Word < anagram.** Fewer errors on words than on anagrams of the same
   letters. The anagram holds the letters, the critical position and the two
   alternatives constant, and removes only the word.
3. **Anagram ≥ letter.** No advantage of anagrams over single letters.
   Reicher's anagrams did not beat single letters, and an unpronounceable
   string of letters gives no unit to perceive it by. If anything, the three
   flanking letters should crowd the critical one and make anagrams worse.
   - **Pronounceable anagrams.** Some anagrams in the lists are pronounceable
     (e.g. `TURO`). Pronounceable pseudowords show nearly the same advantage
     as words (Baron & Thurston, 1973; McClelland & Johnston, 1977), so these
     items may do better than the rest.

   Expected order of accuracy: **word > letter ≥ anagram**.

**Secondary predictions:**

- **The effect needs a pattern mask.** With a blank field after the stimulus
  instead of a pattern mask, the word advantage over single letters shrinks or
  disappears (Johnston & McClelland, 1973). This program always uses a
  pattern mask.
- **Errors are choices of the foil, not guesses towards a word.** In the word
  condition both alternatives make a word, and which member of the pair is the
  target is balanced. So errors should not favour the more frequent word of a
  pair beyond what its letter's visibility explains.
- **Position.** In four-letter strings the outer letters (positions 1 and 4)
  are usually reported better than the inner ones, which are crowded on both
  sides. This should show for words and anagrams, but much less for single
  letters, which have no flankers.
- **Duration.** Reicher found the ordering at each of his three durations.
  Accuracy should rise with duration in every condition, but this program runs
  one duration only.

### Response times

Reicher did not measure response times, and the instructions here ask for
accuracy, not speed. There is **no paradigm-specific RT prediction**. RT is
measured from the onset of the alternatives, so it includes reading the two
letters and deciding.

RT is recorded for two checks:

- **Speed–accuracy trade-off.** The word advantage in accuracy should not come
  with *slower* responses to words. If words were both more accurate and
  clearly slower than letters, the accuracy difference could reflect more
  deliberation rather than better perception.
- **Errors slower than correct responses.** In a two-choice task errors are
  usually slower than correct responses, and responses are slower in the
  harder conditions. So RTs are expected to rise with error rate across
  conditions (word fastest). This is a general expectation for two-choice
  tasks, not a claim from Reicher.

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
| `item` | the base word the letter pair comes from |
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

### Analysis

`analyze.py` (Python 3, standard library only) reports each file. With several
files it adds a group summary:

```bash
python3 examples/Reading_Word_Superiority_Effect/analyze.py ~/goxpy_data/Word-Superiority-Effect_*.csv
python3 examples/Reading_Word_Superiority_Effect/analyze.py --errors FILE.csv   # list every error
```

For each file it reports:

- **Timing:** trials whose measured duration is off the request by more than ¼ frame.
- **Staircase:** the track and accuracy at each duration.
- **Accuracy:** per material, with 95% intervals.
- **Comparisons:** item-paired exact McNemar tests between materials.
- **Breakdowns:** by critical position, top/bottom response bias, and first vs second half.
- **Calibration:** staircase accuracy at the main-task duration against main-task letter accuracy.

## Departures from Reicher (1969)

- **One item per display.** There are no two-item conditions (two rows of four
  slots), and no precue condition (alternatives announced before the trial).
- **Duration.** Reicher ran three durations per participant, between their
  60 % and 90 % points for single letters (35–85 ms across participants). Here a single
  duration near 79 % is set by a staircase, and it is a whole number of display
  frames (16.7 ms steps at 60 Hz). On a fast display, or for a participant who
  needs less than one frame, the threshold hits the 1-frame floor. The program
  warns when that happens.
- **Staircase foils.** Reicher's threshold sessions paired each letter with a
  random foil. A random foil is usually easy to tell from the target (Q vs L),
  while the list's pairs are often similar (A/O, I/U). In a pilot run, a
  random-foil staircase settled on 5 frames, at which single letters were
  right 18/19 times in the staircase but 28/48 times in the main task. The
  staircase therefore uses the list's own pairs, so it calibrates the
  discrimination the letter condition tests. The price is that participants
  see the letter pairs, as single letters, before the main task.
- **Mask.** An X and an O overlaid in each slot, after Reicher's field of
  overlapping typewritten Xs and Os, rather than a continuous noise field.

## References

Baron, J., & Thurston, I. (1973). An analysis of the word-superiority effect.
*Cognitive Psychology, 4*(2), 207–228.

Johnston, J. C., & McClelland, J. L. (1973). Visual factors in word
perception. *Perception & Psychophysics, 14*(2), 365–370.

McClelland, J. L., & Johnston, J. C. (1977). The role of familiar units in
perception of words and nonwords. *Perception & Psychophysics, 22*(3), 249–261.

Reicher, G. M. (1969). Perceptual recognition as a function of meaningfulness of
stimulus material. *Journal of Experimental Psychology, 81*(2), 275–280.

Wheeler, D. D. (1970). Processes in word recognition. *Cognitive Psychology,
1*(1), 59–85.

---

<!-- BEGIN:links -->
## Try it without building

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/Reading_Word_Superiority_Effect/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
