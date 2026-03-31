# Goxpyriment Example Experiments (Demos)


Pre-built binaries for all [examples](https://github.com/chrplr/goxpyriment/tree/main/examples) are published with each [GitHub release](https://github.com/chrplr/goxpyriment/releases/latest):

⚠️  **WARNING** These binaries are unsigned. macOS Gatekeeper and Windows Defender will show security warnings or worse, _misleasding messages_ such as 'this program is damaged'. Don't be intimidated: 
*  macOS: Right-click the app → **Open**, or run `xattr -dr com.apple.quarantine <AppName>.app` in Terminal. See [macOS installation and security](https://chrplr.github.io/note-about-macos-unsigned-apps) for step-by-step instructions.
*  Windows: Just click on "More info" then "Run anyway". 

## Download pre-built binaries


* **Linux (x86-64):** Download [`goxpyriment-examples-linux-x86_64-appimages.tar.gz`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-linux-x86_64-appimages.tar.gz), extract with `tar xzf`, and run the `.AppImage` files directly (no install needed).

* **Windows (x86-64):** Download [`goxpyriment-examples-windows-x86_64.zip`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-windows-x86_64.zip), extract it, and run any `.exe` directly.

* **macOS (Apple Silicon):** Download [`goxpyriment-examples-macos-arm64.zip`](https://github.com/chrplr/goxpyriment/releases/latest/download/goxpyriment-examples-macos-arm64.zip), extract it, and move the `.app` bundles to a folder of your choice (e.g. `~/Applications/goxpyriment`).

A good place to start: `Memory_span`, `Change-Blindness`, `retinotopy`.

When launched from the command line, most programs accept `-w` (windowed 1024×768 mode), `-d N` (open on monitor N, 0 = primary), and `-s <id>` (subject ID written to the `.csv` data file).

Results are saved in a folder `goxpy_data` in your home folder.


If you want to compile these experiments from source on your computer, e.g. because you have Windows/ARM or macOS/Intel,  read on. 

## Building from source

If [Go](https://go.dev) is installed, you can run any example directly from a clone of the repository:

```bash
go run ./examples/parity_decision/ -w -s 1
```

Or build and run from inside the example directory:

```bash
cd examples/hello_world
go run .            # fullscreen by default
go run . -w         # windowed 1024×768
go run . -w -s 1    # windowed, subject ID = 1
go run . -d 1       # fullscreen on monitor 1
go run . -w -d 1    # windowed on monitor 1
go build .          # build a standalone binary
```

To build all examples at once (binaries go to `examples/_build/`):

```bash
make examples       # from the repo root
# or, from inside the examples/ directory:
bash build.sh
```

Programs that open a **GetParticipantInfo** dialog collect all setup interactively (subject ID, monitor dimensions, fullscreen toggle, and any experiment-specific options). Pass `-headless` on the command line to skip the dialog and use field defaults — useful for scripted runs and automated testing. Programs that do not use the dialog still accept `-w` for windowed mode, `-d N` to select a monitor, and `-s <id>` for a subject ID.

---

## Psychological Experiments

Full experiments that record and save behavioural data to an `.csv` file in `goxpy_data/`.

<!-- BEGIN:experiments -->

| Directory | Task | Reference |
|-----------|------|-----------|
| [Attention-Posner-Task](Attention-Posner-Task/) | Arrow cue directs covert attention; measure cost/benefit on reaction time to a peripheral target | Posner (1980) |
| [Attentional-Blink](Attentional-Blink/) | RSVP stream; participant detects two targets embedded in a stream of distractors — the second target is often missed within ~500 ms of the first | Raymond et al. (1992) |
| [Change-Blindness](Change-Blindness/) | Flicker paradigm: alternating original and modified scenes separated by blanks; participant detects what changed | Rensink et al. (1997) |
| [Classification-Posner-Mitchell](Classification-Posner-Mitchell/) | Classify letter pairs at three levels (physical, name, rule identity); RT increases with depth of processing required | Posner & Mitchell (1967) |
| [Contrast-Detection-QUEST](Contrast-Detection-QUEST/) | 2-IFC adaptive staircase estimating the contrast detection threshold for a Gabor patch; converges on the 82 % correct point | Watson & Pelli (1983) |
| [Finger-Tapping](Finger-Tapping/) | Patterned finger-tapping: memorise a key sequence then reproduce it 6 times consecutively as fast as possible; only error-free runs recorded | Povel & Collard (1982) |
| [Go-NoGo](Go-NoGo/) | Stop-signal task: respond to letters on go-trials; withhold response when a stop-signal tone is played at variable delays | Logan et al. (1984) |
| [Hemispheric-differences-word-processing](Hemispheric-differences-word-processing/) | Lateralised recognition memory: words studied in LVF or RVF, tested centrally with old/new judgements | Federmeier & Benjamin (2005) |
| [Letter-size-illusion](Letter-size-illusion/) | Compare heights of letters vs. mirror/pseudo-letters; replicates the letter height superiority illusion (two experiments) | New et al. (2015) |
| [lexical_decision](lexical_decision/) | Decide whether a letter string is a word or a non-word (F / J keys); stimuli loaded from a CSV file |  |
| [LoT-geometry](LoT-geometry/) | Comprehension of geometric primitives and rules; reproduces Amalric et al. (2017) | Amalric et al. (2017) |
| [Magnitude-Estimation-Luminosity](Magnitude-Estimation-Luminosity/) | Stevens' magnitude estimation of luminance: assign a number to perceived brightness of grey disks | Stevens (1957) |
| [Memory-for-binary-sequences](Memory-for-binary-sequences/) | Memory and reproduction of auditory binary sequences of varying complexity | Planton et al. (2021) |
| [Memory-Iconic-Sperling](Memory-Iconic-Sperling/) | Partial-report procedure measuring capacity and duration of iconic (visual sensory) memory | Sperling (1960) |
| [Memory-Scanning](Memory-Scanning/) | Hold a set of digits in memory; decide whether a probe was in the set — RT scales with set size | Sternberg (1966) |
| [Memory_span](Memory_span/) | Adaptive staircase measuring immediate serial recall span for digits, letters, or words |  |
| [Mental-Logic-Card-Game](Mental-Logic-Card-Game/) | Mental logic and inference task using a card-game paradigm |  |
| [Mental-Rotation-2D](Mental-Rotation-2D/) | Decide whether two 3-D figures are identical or mirror images; RT increases linearly with angular disparity | Shepard & Metzler (1971) |
| [Mental-Rotation-3D](Mental-Rotation-3D/) | Decide whether two 3D figures (procedurally generated assemblies of cubes) are identical or mirror images; RT increases linearly with angular disparity. | Shepard & Metzler (1971) |
| [Multiple-Object-Tracking](Multiple-Object-Tracking/) | Track a subset of identical moving targets among distractors; evidence for a parallel tracking mechanism | Pylyshyn & Storm (1988) |
| [Number-Comparison](Number-Comparison/) | Compare numerical magnitudes of digits and dot patterns; stimulus group (digits / regular / irregular / random) selected via GetParticipantInfo UI | Buckley & Gillman (1974) |
| [Number-Double-Digits-Comparison](Number-Double-Digits-Comparison/) | Compare two-digit numbers against a fixed standard (55 or 65); two experiments with different response-key mappings | Dehaene et al. (1990) |
| [parity_decision](parity_decision/) | Classify single digits (0–9) as even or odd (F / J keys) |  |
| [Perception-of-Temporal-Patterns](Perception-of-Temporal-Patterns/) | Reproduction of isochronous and non-isochronous rhythmic patterns; tests internal clock induction and coding complexity | Povel & Essens (1985) |
| [Posner-ANT](Posner-ANT/) | Attention Network Task (vertical variant): flanker arrows above/below fixation measure alerting, orienting, and executive attention networks | Fan et al. (2009) |
| [Psychological-Refractory-Period](Psychological-Refractory-Period/) | Two tasks presented in rapid succession; the second response is delayed when the SOA is short | Welford (1952) |
| [Retinotopy](Retinotopy/) | HCP retinotopic mapping paradigm (ported from Python); flickering wedge/ring/bar stimuli for visual cortex mapping; run type selected via GetParticipantInfo UI |  |
| [Sensory-Threshold-Estimation-Auditory](Sensory-Threshold-Estimation-Auditory/) | 1-up/2-down adaptive staircase with 2-IFC to estimate pure-tone hearing thresholds across multiple frequencies | Levitt (1971) |
| [Simon_task](Simon_task/) | Identify colour of a square regardless of its screen position; congruent trials are faster | Simon (1969) |
| [simple_reaction_times](simple_reaction_times/) | 20-trial simple RT task: press any key as quickly as possible when a target appears |  |
| [Statistical-Learning-Auditory](Statistical-Learning-Auditory/) | Statistical learning of tone sequences: exposure to a continuous tone stream with structured transitional probabilities, probed with 2AFC or head-turn preference | Saffran et al. (1999) |
| [Stroop_task](Stroop_task/) | Name the ink colour of colour words; incongruent trials (e.g. RED in blue ink) are slower | Stroop (1935) |
| [Subliminal-Priming](Subliminal-Priming/) | Masked word priming: words rendered invisible by surrounding masks still influence processing | Dehaene et al. (2004) |
| [Temporal-Integration-Word-Recognition](Temporal-Integration-Word-Recognition/) | Alternating odd/even letter components at variable SOA; Exp 1 (subjective report: 0/1/2 words perceived) and Exp 2 (lexical decision with RT); experiment selected via GetParticipantInfo UI | Forget et al. (2010) |
| [Trubutschek_Unconscious_Working_Memory](Trubutschek_Unconscious_Working_Memory/) | Probe access to briefly presented stimuli below and above the threshold of consciousness | Trübutschek et al. (2017) |
| [Visual-Illusion-Lilac-Chaser](Visual-Illusion-Lilac-Chaser/) | Lilac chaser illusion: a ring of disappearing disks produces a rotating green afterimage |  |
| [Visual_Statistical_Learning](Visual_Statistical_Learning/) | Implicit learning of statistical regularities in a shape stream, probed with forced-choice and RT tests | Turk-Browne et al. (2005) |

<!-- END:experiments -->

---

## Demonstrations

Visual illusions, interactive showcases, and minimal templates. Most do not write a data file.

<!-- BEGIN:demos -->

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

<!-- END:demos -->

---

## Technical Tests

Hardware and timing tests live in the [`tests/`](../tests/) directory at the repository root (separate Go module).
