# Examples

This directory contains self-contained programs built with the goxpyriment framework. Each subdirectory has its own `main.go` and a `README.md` (or `description.md`) with full details.

## Building and Running

You can build all examples at once or individually using the provided `Makefile`.

To build all examples:
```bash
make all
```

To build a specific example (e.g., `hello_world`):
```bash
make hello_world
```

To remove all compiled binaries:
```bash
make clean
```

Alternatively, you can run an example directly without building:
```bash
go run hello_world/main.go
```


Programs that open a **GetParticipantInfo** dialog collect all setup interactively (subject ID, monitor dimensions, fullscreen toggle, and any experiment-specific options). Pass `-headless` on the command line to skip the dialog and use field defaults — useful for scripted runs and automated testing. Programs that do not use the dialog still accept `-d` for windowed development mode and `-s <id>` for a subject ID.

---

## Psychological Experiments

Full experiments that record and save behavioural data to an `.xpd` file in `goxpy_data/`.

| Directory | Task | Reference |
|-----------|------|-----------|
| [Attention-Posner-Task](Attention-Posner-Task/) | Arrow cue directs covert attention; measure cost/benefit on reaction time to a peripheral target | Posner (1980) |
| [Attentional-Blink](Attentional-Blink/) | RSVP stream; participant detects two targets embedded in a stream of distractors — the second target is often missed within ~500 ms of the first | Raymond et al. (1992) |
| [Change-Blindness](Change-Blindness/) | Flicker paradigm: alternating original and modified scenes separated by blanks; participant detects what changed | Rensink et al. (1997) |
| [Contrast-Detection-QUEST](Contrast-Detection-QUEST/) | 2-IFC adaptive staircase estimating the contrast detection threshold for a Gabor patch; converges on the 82 % correct point | Watson & Pelli (1983) |
| [Finger-Tapping](Finger-Tapping/) | Patterned finger-tapping: memorise a key sequence then reproduce it 6 times consecutively as fast as possible; only error-free runs recorded | Povel & Collard (1982) |
| [Hemispheric-differences-word-processing](Hemispheric-differences-word-processing/) | Lateralised recognition memory: words studied in LVF or RVF, tested centrally with old/new judgements | Federmeier & Benjamin (2005) |
| [Letter-size-illusion](Letter-size-illusion/) | Compare heights of letters vs. mirror/pseudo-letters; replicates the letter height superiority illusion (two experiments) | New et al. (2015) |
| [lexical_decision](lexical_decision/) | Decide whether a letter string is a word or a non-word (F / J keys); stimuli loaded from a CSV file | |
| [LoT-geometry](LoT-geometry/) | Comprehension of geometric primitives and rules; reproduces Amalric et al. (2017) | Amalric et al. (2017) |
| [Magnitude-Estimation-Luminosity](Magnitude-Estimation-Luminosity/) | Stevens' magnitude estimation of luminance: assign a number to perceived brightness of grey disks | Stevens (1957) |
| [Memory-for-binary-sequences](Memory-for-binary-sequences/) | Memory and reproduction of auditory binary sequences of varying complexity | Planton et al. (2021) |
| [Memory-Iconic-Sperling](Memory-Iconic-Sperling/) | Partial-report procedure measuring capacity and duration of iconic (visual sensory) memory | Sperling (1960) |
| [Memory-Scanning](Memory-Scanning/) | Hold a set of digits in memory; decide whether a probe was in the set — RT scales with set size | Sternberg (1966) |
| [Memory_span](Memory_span/) | Adaptive staircase measuring immediate serial recall span for digits, letters, or words | |
| [Mental-Logic-Card-Game](Mental-Logic-Card-Game/) | Mental logic and inference task using a card-game paradigm | |
| [Mental-Rotation](Mental-Rotation/) | Decide whether two 3-D figures are identical or mirror images; RT increases linearly with angular disparity | Shepard & Metzler (1971) |
| [Multiple-Object-Tracking](Multiple-Object-Tracking/) | Track a subset of identical moving targets among distractors; evidence for a parallel tracking mechanism | Pylyshyn & Storm (1988) |
| [Number-Comparison](Number-Comparison/) | Compare numerical magnitudes of digits and dot patterns; stimulus group (digits / regular / irregular / random) selected via GetParticipantInfo UI | Buckley & Gillman (1974) |
| [Number-Double-Digits-Comparison](Number-Double-Digits-Comparison/) | Compare two-digit numbers against a fixed standard (55 or 65); two experiments with different response-key mappings | Dehaene et al. (1990) |
| [parity_decision](parity_decision/) | Classify single digits (0–9) as even or odd (F / J keys) | |
| [Perception-of-Temporal-Patterns](Perception-of-Temporal-Patterns/) | Reproduction of isochronous and non-isochronous rhythmic patterns; tests internal clock induction and coding complexity | Povel & Essens (1985) |
| [Psychological-Refractory-Period](Psychological-Refractory-Period/) | Two tasks presented in rapid succession; the second response is delayed when the SOA is short | Welford (1952) |
| [retinotopy](retinotopy/) | HCP retinotopic mapping paradigm (ported from Python); flickering wedge/ring/bar stimuli for visual cortex mapping; run type selected via GetParticipantInfo UI | |
| [Sensory-Threshold-Estimation-Auditory](Sensory-Threshold-Estimation-Auditory/) | 1-up/2-down adaptive staircase with 2-IFC to estimate pure-tone hearing thresholds across multiple frequencies | Levitt (1971) |
| [Simon_task](Simon_task/) | Identify colour of a square regardless of its screen position; congruent trials are faster | Simon (1969) |
| [simple_reaction_times](simple_reaction_times/) | 20-trial simple RT task: press any key as quickly as possible when a target appears | |
| [Stroop_task](Stroop_task/) | Name the ink colour of colour words; incongruent trials (e.g. RED in blue ink) are slower | Stroop (1935) |
| [Subliminal-Priming](Subliminal-Priming/) | Masked word priming: words rendered invisible by surrounding masks still influence processing | Dehaene et al. (2004) |
| [Trubutschek_Unconscious_Working_Memory](Trubutschek_Unconscious_Working_Memory/) | Probe access to briefly presented stimuli below and above the threshold of consciousness | Trübutschek et al. (2017) |
| [Visual_Statistical_Learning](Visual_Statistical_Learning/) | Implicit learning of statistical regularities in a shape stream, probed with forced-choice and RT tests | Turk-Browne et al. (2005) |
| [Visual-Illusion-Lilac-Chaser](Visual-Illusion-Lilac-Chaser/) | Lilac chaser illusion: a ring of disappearing disks produces a rotating green afterimage | |

---

## Demonstrations

Visual illusions, interactive showcases, and minimal templates. Most do not write a data file.

| Directory | Description |
|-----------|-------------|
| [canvas_demo](canvas_demo/) | Drawing on an off-screen `Canvas` surface before presenting it in one frame |
| [getinfo_demo](getinfo_demo/) | Demonstrates the `GetParticipantInfo` dialog: collects participant demographics and monitor characteristics before the experiment window opens |
| [hello_world](hello_world/) | Simplest possible goxpyriment program — good starting point for new users |
| [Motion-Blur](Motion-Blur/) | Motion blur vs. phantom array demo: animated bar demonstrates retinal blur and the strobe effect at 60 Hz |
| [mouse_audio_feedback](mouse_audio_feedback/) | Left/right mouse clicks trigger ping/buzzer audio; useful for testing sound output |
| [play_two_videos](play_two_videos/) | Plays `.mpg` video pairs side by side and records a keypress response after each pair |
| [play_videos](play_videos/) | Plays all `.mpg` files from an `assets/` folder sequentially |
| [random-dot-stereogram](random-dot-stereogram/) | Random-dot stereogram that reveals a 3-D shape when fused binocularly |
| [simple_example](simple_example/) | Minimal five-trial loop (fixation → stimulus → keypress); use as a starting template |
| [stimuli_extras](stimuli_extras/) | Showcase of advanced stimuli: visual mask, Gabor patch, dot cloud, stimulus circle, thermometer |
| [text_input](text_input/) | Demonstration of the `TextInput` stimulus collecting free-text keyboard input |
| [Visual-Angle-Calibration](Visual-Angle-Calibration/) | Draws concentric rings at 2°, 5°, and 10° of visual angle for a quick sanity-check of the `units.Monitor` calibration |
| [Visual-Illusion-Ebbginghaus](Visual-Illusion-Ebbginghaus/) | Animated Ebbinghaus (Titchener circles) size-contrast illusion |
| [Visual-Illusion-Kanizsa](Visual-Illusion-Kanizsa/) | Kanizsa illusory-contour square: a square is perceived where none is drawn |

---

## Technical Tests

Hardware and timing tests live in the `tests/` directory at the repository root (separate Go module). See `tests/README.md`.
