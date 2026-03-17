# Examples

This directory contains self-contained programs built with the goxpyriment framework. Each subdirectory has its own `main.go` and a `README.md` with full details.

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


All programs accept `-d` for windowed development mode and `-s <id>` for a subject/participant ID unless stated otherwise.

---

## Psychological Experiments

Full experiments that record and save behavioural data to an `.xpd` file in `goxpy_data/`.

| Directory | Task | Reference |
|-----------|------|-----------|
| [attentional-blink](attentional-blink/) | RSVP stream; participant detects two targets embedded in a stream of distractors — the second target is often missed within ~500 ms of the first | Raymond et al. (1992) |
| [card_game](card_game/) | Mental logic and inference task using a card-game paradigm | |
| [hemispheric-differences-word-processing](hemispheric-differences-word-processing/) | Lateralised recognition memory: words studied in LVF or RVF, tested centrally with old/new judgements | Federmeier, K. D., & Benjamin, A. S. (2005).|
| [lexical_decision](lexical_decision/) | Decide whether a letter string is a word or a non-word (F / J keys); stimuli loaded from a CSV file | |
| [LoT-geometry](LoT-geometry/) | Comprehension of geometric primitives and rules; reproduces Amalric et al. (2017) | Amalric et al. (2017) |
| [Magnitude-Estimation-Luminosity](Magnitude-Estimation-Luminosity/) | Stevens' magnitude estimation of luminance: assign a number to perceived brightness of grey disks | Stevens (1957) |
| [Memory_span](Memory_span/) | Adaptive staircase measuring immediate serial recall span for digits, letters, or words | |
| [New-letter-size-illusion](New-letter-size-illusion/) | Compare heights of letters vs. mirror/pseudo-letters; replicates the letter height superiority illusion (two experiments) | New et al. (2015) |
| [parity_decision](parity_decision/) | Classify single digits (0–9) as even or odd (F / J keys) | |
| [Posner_task_simple](Posner_task_simple/) | Arrow cue directs covert attention; measure cost/benefit on reaction time to a peripheral target | Posner (1980) |
| [retinotopy](retinotopy/) | HCP retinotopic mapping paradigm (ported from Python); flickering wedge/ring stimuli for visual cortex mapping | |
| [Sensory-Threshold-Estimation-Auditory](Sensory-Threshold-Estimation-Auditory/) | 1-up/2-down adaptive staircase with 2-IFC to estimate pure-tone hearing thresholds across multiple frequencies | Levitt (1971) |
| [Shepard-mental-rotation](Shepard-mental-rotation/) | Decide whether two 3-D figures are identical or mirror images; RT increases linearly with angular disparity | Shepard & Metzler (1971) |
| [Simon_task](Simon_task/) | Identify colour of a square regardless of its screen position; congruent trials are faster | Simon (1969) |
| [simple_reaction_times](simple_reaction_times/) | 20-trial simple RT task: press any key as quickly as possible when a target appears | |
| [Sperling-iconic-memory](Sperling-iconic-memory/) | Partial-report procedure measuring capacity and duration of iconic (visual sensory) memory | Sperling (1960) |
| [Sternberg_memory_search](Sternberg_memory_search/) | Hold a set of digits in memory; decide whether a probe was in the set — RT scales with set size | Sternberg (1966) |
| [Stroop_task](Stroop_task/) | Name the ink colour of colour words; incongruent trials (e.g. RED in blue ink) are slower | Stroop (1935) |
| [Trubutschek_Unconscious_Working_Memory](Trubutschek_Unconscious_Working_Memory/) | Probe access to briefly presented stimuli below and above the threshold of consciousness | Trübutschek et al. (2017) |
| [Visual_Statistical_Learning](Visual_Statistical_Learning/) | Implicit learning of statistical regularities in a shape stream, probed with forced-choice and RT tests | Turk-Browne et al. (2005) |

---

## Demonstrations

Visual illusions, interactive showcases, and minimal templates. Most do not write a data file.

| Directory | Description |
|-----------|-------------|
| [canvas_demo](canvas_demo/) | Drawing on an off-screen `Canvas` surface before presenting it in one frame |
| [Ebbinghaus-illusion-dynamic](Ebbinghaus-illusion-dynamic/) | Animated Ebbinghaus (Titchener circles) size-contrast illusion |
| [hello_world](hello_world/) | Simplest possible goxpyriment program — good starting point for new users |
| [kanizsa-square](kanizsa-square/) | Kanizsa illusory-contour square: a square is perceived where none is drawn |
| [lilac_chaser](lilac_chaser/) | Lilac chaser illusion: a ring of disappearing disks produces a rotating green afterimage |
| [mouse_audio_feedback](mouse_audio_feedback/) | Left/right mouse clicks trigger ping/buzzer audio; useful for testing sound output |
| [play_two_videos](play_two_videos/) | Plays `.mpg` video pairs side by side and records a keypress response after each pair |
| [play_videos](play_videos/) | Plays all `.mpg` files from an `assets/` folder sequentially |
| [random-dot-stereogram](random-dot-stereogram/) | Random-dot stereogram that reveals a 3-D shape when fused binocularly |
| [simple_example](simple_example/) | Minimal five-trial loop (fixation → stimulus → keypress); use as a starting template |
| [stimuli_extras](stimuli_extras/) | Showcase of advanced stimuli: visual mask, Gabor patch, dot cloud, stimulus circle, thermometer |
| [text_input](text_input/) | Demonstration of the `TextInput` stimulus collecting free-text keyboard input |

---

## Technical Tests

Utilities for verifying hardware and framework internals. No experimental data collected.

| Directory | Description |
|-----------|-------------|
| [test_audio](test_audio/) | Plays a buzzer then a ping to confirm that SDL3 audio output is working |
| [test_fullscreen](test_fullscreen/) | Reports display resolution, refresh rate, and pixel density; animates a physics ball to check rendering |
| [test_playgv](test_playgv/) | Plays a `.gv` (LZ4-compressed RGBA) video file to verify the custom video format |
| [test_stream_images](test_stream_images/) | Runs `PresentStreamOfImages` and logs actual vs. requested onset/offset times |
| [test_stream_text](test_stream_text/) | RSVP word-stream timing test with per-frame onset/offset measurement |
