# Wisconsin Card Sorting Task

A computerised version of the **Wisconsin Card Sorting Test** (Berg, 1948; Grant & Berg, 1948; Heaton et al., 1993), the classic measure of *set shifting* — the ability to abandon a rule that has stopped working.

Four **key cards** are displayed in a row at the top of the screen:

| 1 | 2 | 3 | 4 |
|---|---|---|---|
| one red triangle | two green stars | three yellow crosses | four blue circles |

On each trial a **test card** appears below them. The participant decides which key card it goes with and says so by pressing `1`–`4` or by clicking the card. The only feedback is **Correct** or **Wrong**.

The sorting rule is never announced. It is one of **number**, **colour** or **shape**, and it changes — without warning, to one of the other two — as soon as the participant has made a run of 6 to 10 consecutive correct sorts (the run length is drawn at random for each category). Once the rule has changed, continuing to sort by the old one produces a **perseverative error**, which is what the test is built to count.

---

## Trial structure

```
ITI (blank)  →  key cards + test card  →  response (1–4 or click)  →  feedback
   400 ms              (self-paced)                                     800 ms
```

The chosen key card is framed in black during the feedback, so the participant can see which card the feedback refers to.

---

## The deck

The stimuli are the 64 cards in `stimuli/` (4 numbers × 4 colours × 4 shapes), embedded in the binary with `//go:embed`.

By default the test cards are drawn from the **24 unambiguous cards** — those whose number, colour and shape each point at a *different* key card. Every response then identifies exactly one rule, which is what makes perseveration readable in the data. `-all-cards` switches to the 60-card deck of the clinical version, where a card can match one key card on two dimensions at once.

Cards are drawn without replacement and the pool is reshuffled when exhausted (never repeating a card across the seam), so no card recurs until all the others have been seen.

---

## Running

```bash
# From the repository root (go.work resolves the workspace)
go run ./examples/Wisconsin-Card-Sorting-Task -s 1        # fullscreen
go run ./examples/Wisconsin-Card-Sorting-Task -s 1 -w     # windowed
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID |
| `-w` | off | Windowed mode (1024×768) |
| `-d N` | -1 | Display index (-1 = primary) |
| `-n` | 128 | Number of trials |
| `-minrun` | 6 | Minimum consecutive correct sorts before the rule changes |
| `-maxrun` | 10 | Maximum consecutive correct sorts before the rule changes |
| `-fb` | 800 | Feedback duration (ms) |
| `-iti` | 400 | Inter-trial interval (ms) |
| `-soa` | 0 | Fixed stimulus-onset asynchrony (ms). 0 = self-paced trials; any positive value puts every trial on a fixed grid (scanner mode, below) |
| `-trigger` | off | Wait for the scanner trigger (the first `t` keypress after the instructions) before starting |
| `-all-cards` | off | Use the full 60-card deck instead of the 24 unambiguous cards |
| `-max_duration` | 0 | Time limit for the session, as a Go duration (`10m`, `90s`, `1m30s`); 0 = no limit. See below |
| `-photodiode` | off | White square in the top-left corner for 3 frames from every card onset, black otherwise. See below |
| `-photodiode-size` | 100 | Side of the photodiode square, in pixels |
| `-marker-frames` | 3 | Frames the square stays white from each onset; also the default TTL pulse length |
| `-ttl KIND[:PORT]` | none | Send a TTL code at every card onset: `dlpio8[:PORT\|auto]`, `dlpio20`, `megttlbox:/dev/ttyACM0`, `mmbts:/dev/ttyACM0`, `parallel[:/dev/parport0]`, `gpio`, `ft232h`, `labjackt4:HOST`, `null` |
| `-ttl-ms` | 0 | How long the code stays on the lines, in ms; 0 = `-marker-frames` frame periods (50 ms at 60 Hz) |
| `-ttl-code` | 1 | Code sent at each onset (1–255; 1 raises line 0) |

ESC quits at any point; the data collected so far are saved.

### Time limit

`-max_duration` ends the task after a fixed time, whichever comes first of the limit and the `-n` trials (so set `-n` high enough for the limit to be the one that binds):

```bash
go run ./examples/Wisconsin-Card-Sorting-Task -s 1 -n 1000 -max_duration 10m
```

- The time is counted from the same zero as the trials: after the instructions screen, or from the scanner pulse with `-trigger`.
- No trial starts once the limit has passed.
- A card still waiting for an answer when the limit expires is withdrawn; that trial is **not recorded** (it is not logged as a miss).
- A trial answered before the limit is completed with its feedback, so the session can end up to one feedback duration (`-fb`) after the limit — or, self-paced, up to one inter-trial interval (`-iti`) if the limit falls during the blank.
- The limit is written into the `-info.txt`, and so, when it is reached, are the number of trials completed and the elapsed time (also printed on the console). The CSV holds exactly the completed trials; the end screen counts only those.


### Onset markers: photodiode and TTL

For a recording (MEG, EEG, or checking the display timing), every card onset can be marked on the screen and on a trigger line:

```bash
go run ./examples/Wisconsin-Card-Sorting-Task -s 1 -photodiode -ttl dlpio8
go run ./examples/Wisconsin-Card-Sorting-Task -s 1 -photodiode -ttl megttlbox:/dev/ttyACM0 -ttl-code 4
```

These flags are the library's onset marker (`control.MarkerFlags`; see UserManual Section 17), so they behave the same in every paradigm that uses it.

- `-photodiode` shows a square in the top-left corner (100 px, `-photodiode-size`) that is **white for 3 frames from each card onset** (50 ms at 60 Hz) and black on every other frame of the session, instructions included. It is black rather than background grey, because black→white is a much larger step than grey→white and a photodiode detects it more reliably. The 100 px square leaves the first key card clear; a size above 160 px overlaps it, and the program warns.
- `-ttl` writes `-ttl-code` to the TTL lines right after the onset flip — the same flip that turns the square white — and clears them by timer 3 frame periods later (50 ms at 60 Hz; `-ttl-ms` to change). A device that cannot be opened stops the program before any window opens; a write that fails during the run is reported on the console and counted in the `-info.txt`.
- With `-photodiode` the card is held, redrawn, until the square has gone black again (4 flips) before the response wait starts. A response given meanwhile is kept, with its own timestamp, so RTs are unaffected. `-ttl` alone adds no frames.
- The TTL edge lands a few microseconds after the flip returns, while the square reaches the screen one or more frames later, depending on the display stack. That lag is what a photodiode on the corner measures: subtract it in analysis.
- Hardware triggers are desktop-only. In the browser build `-ttl` is refused; `-photodiode` works.
- The settings are written into the `-info.txt`, and at the end of the session a `marker stats:` line (onsets, codes written and cleared, write errors).

Checked on this machine (windowed, Wayland, 60 Hz, `-n 2 -soa 2000 -photodiode`, with a logging TTL device swapped in that records each write's SDL timestamp): the corner pixel read back just before each present was black on the blank and feedback screens and white on card frames 0–2, black from frame 3. The code was written 0–2 µs after the onset flip timestamp and cleared 50.5 and 51.0 ms later (the clearing timer's slack is about a millisecond). Not yet measured: the real devices' latency, and the photons, which need a photodiode on the screen.

---

## Scanner mode: fixed SOA

With `-soa` the trial is a fixed grid instead of a self-paced sequence, which is what an fMRI (or EEG) design needs:

```
t = k*soa                                            t = (k+1)*soa
   |<--- card: soa - fb - iti --->|<-- fb -->|<- iti ->|
   card onset                     feedback    blank     next card onset
```

- The card stays on screen for the **whole** response window, whether or not the participant has already answered; later presses in the same window are ignored. Feedback onset is therefore on the grid and does not inherit the jitter of the reaction time.
- Every phase is timed against a single session zero, not against the phase before it, so a late frame costs its own phase and nothing downstream: onsets cannot drift over a run.
- **Total duration = `n` × `soa`**, exactly, excluding the instruction screen. `-n 128 -soa 4000` is 512 s = 8 min 32 s.
- The response window is `soa - fb - iti`; it must leave at least 500 ms or the program refuses to start. With the defaults (`-fb 800 -iti 400`), `-soa 4000` gives a 2800 ms window.
- A trial with no response shows **Too slow**, is logged with `response = 0`, `modality = none`, `rt_ms = 0`, `correct = false`, and (like any error) resets the run of consecutive correct sorts.

Measured on this machine with `-n 8 -soa 2000` (windowed, no responses given): onsets fell at 16, 2001, 4001, 6001, 8000, 10000, 12000, 14000 ms against targets 0, 2000 … 14000 — trials 2–8 within 1 ms of the grid, trial 1 one frame late because its flip waits for the first retrace after the session zero. The clock used for scheduling is the Go monotonic clock; reaction times are still taken from the SDL event timestamps against the VSYNC-stamped flip.

### Starting on the scanner trigger

`-trigger` inserts a wait between the instructions and the first card:

```
instructions (SPACE)  →  "Waiting for scanner TTL…"  →  first 't'  →  trial 1
```

The screen shows **Waiting for scanner TTL…** until the first `t` keypress arrives — the pulse most scanners send at the start of each volume, delivered as a keystroke by the usual interfaces. Subsequent `t` pulses during the run are ignored: they are not response keys.

Trial onsets are counted from the **pulse**, not from the moment the program noticed it. The trigger is detected by a 1 ms polling loop, so the session clock is reset a millisecond or two late; the SDL event timestamp of the keypress says by how much, and the grid is shifted back by that lag. The lag is printed at startup and written into the `-info.txt`.

Verified with a simulated pulse placed 3 ms in the past (`-n 4 -soa 2000 -trigger`): the run reported a 3 ms detection lag and trial onsets landed at 16, 1998, 3997, 5997 ms against targets −3, 1997, 3997, 5997 — i.e. on the trigger-anchored grid, the first trial again one frame late because of the flip. The waiting screen itself was checked by frame capture. The real `t`-keypress path uses the standard `Keyboard.GetKeyEventTS` call and has not been exercised against a scanner.

ESC works on the waiting screen, so a run can be abandoned before the first pulse.

---

## Responses

- **Keyboard:** `1`, `2`, `3`, `4` (digit row or keypad) for the key card at that position, left to right.
- **Mouse:** left-click anywhere on a key card. Clicks outside the four cards are ignored.

The cursor is made visible at startup for the mouse route. On display back-ends that cannot create a cursor shape (KMSDRM on a bare console), a warning is printed and the keyboard route still works.

---

## Output

One `.csv` per session in the data directory (plus the companion `-info.txt` with the session metadata):

| Column | Meaning |
|---|---|
| `trial` | Trial number, 1-based |
| `category` | Index of the current rule block (1-based) |
| `rule` | Rule in force during this trial: `number`, `color` or `shape` |
| `trial_in_category` | Position of the trial within the current rule block |
| `run_correct` | Consecutive correct sorts after this trial |
| `test_card` | e.g. `3-blue-triangle` |
| `number`, `color`, `shape` | The test card's attributes |
| `match_number`, `match_color`, `match_shape` | The key card (1–4) this test card belongs under by each rule |
| `response` | Key card chosen, 1–4 (0 = no response before the deadline, fixed-SOA mode only) |
| `modality` | `key`, `mouse`, or `none` (no response) |
| `rt_ms` | Time from the VSYNC-stamped card onset to the response event |
| `correct` | Whether the response matched the rule in force |
| `matched_dims` | Dimensions the chosen card actually shared with the test card (`color+number`, empty if none) |
| `perseverative` | True when an error sorts by the rule abandoned at the last change (Heaton's perseveration to the previous principle) |
| `rule_changed_after` | True when this trial completed a category and the rule changed |

`rule`, `category` and `trial_in_category` describe the state *during* the trial; a change triggered by the trial is reported by `rule_changed_after` and takes effect on the next row.

---

## References

Berg, E. A. (1948). A simple objective technique for measuring flexibility in thinking. *The Journal of General Psychology*, 39(1), 15–22. https://doi.org/10.1080/00221309.1948.9918159

Grant, D. A., & Berg, E. A. (1948). A behavioral analysis of degree of reinforcement and ease of shifting to new responses in a Weigl-type card-sorting problem. *Journal of Experimental Psychology*, 38(4), 404–411. https://doi.org/10.1037/h0059831

Heaton, R. K., Chelune, G. J., Talley, J. L., Kay, G. G., & Curtiss, G. (1993). *Wisconsin Card Sorting Test Manual: Revised and Expanded*. Psychological Assessment Resources.

Wikipedia: https://en.wikipedia.org/wiki/Wisconsin_Card_Sorting_Test

---

<!-- BEGIN:links -->
## Try it without building

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/Wisconsin-Card-Sorting-Task/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
