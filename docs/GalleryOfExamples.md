# Goxpyriment Example Experiments


[Pre-built executable programs](pre-built-examples.md) for all [examples](https://github.com/chrplr/goxpyriment/tree/main/examples).


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

To build all examples (and tests) at once (binaries go to `_build/`):

```bash
./build-all.sh      # from the repo root — works on Linux, macOS, and Windows (Git Bash)
```

(On Linux/macOS you can also use `make examples` or `make all`.)

Programs that open a **GetParticipantInfo** dialog collect all setup interactively (subject ID, monitor dimensions, fullscreen toggle, and any experiment-specific options). Pass `-headless` on the command line to skip the dialog and use field defaults — useful for scripted runs and automated testing. Programs that do not use the dialog still accept `-w` for windowed mode, `-d N` to select a monitor, and `-s <id>` for a subject ID.

---

## Psychological Experiments

Full experiments that record and save behavioural data to an `.csv` file in `goxpy_data/`.

<!-- BEGIN:experiments -->
| Directory | Task | Reference |
|-----------|------|-----------|
| [Attention-Posner-Task](https://github.com/chrplr/goxpyriment/tree/main/examples/Attention-Posner-Task) | Arrow cue directs covert attention; measure cost/benefit on reaction time to a peripheral target | Posner (1980) |
| [Attentional-Blink](https://github.com/chrplr/goxpyriment/tree/main/examples/Attentional-Blink) | RSVP stream; participant detects two targets embedded in a stream of distractors — the second target is often missed within ~500 ms of the first | Raymond et al. (1992) |
| [Change-Blindness](https://github.com/chrplr/goxpyriment/tree/main/examples/Change-Blindness) | Flicker paradigm: alternating original and modified scenes separated by blanks; participant detects what changed | Rensink et al. (1997) |
| [Classification-Posner-Mitchell](https://github.com/chrplr/goxpyriment/tree/main/examples/Classification-Posner-Mitchell) | Classify letter pairs at three levels (physical, name, rule identity); RT increases with depth of processing required | Posner & Mitchell (1967) |
| [Contrast-Detection-QUEST](https://github.com/chrplr/goxpyriment/tree/main/examples/Contrast-Detection-QUEST) | 2-IFC adaptive staircase estimating the contrast detection threshold for a Gabor patch; converges on the 82 % correct point | Watson & Pelli (1983) |
| [Finger-Tapping](https://github.com/chrplr/goxpyriment/tree/main/examples/Finger-Tapping) | Patterned finger-tapping: memorise a key sequence then reproduce it 6 times consecutively as fast as possible; only error-free runs recorded | Povel & Collard (1982) |
| [Go-NoGo](https://github.com/chrplr/goxpyriment/tree/main/examples/Go-NoGo) | Stop-signal task: respond to letters on go-trials; withhold response when a stop-signal tone is played at variable delays | Logan et al. (1984) |
| [Hemispheric-differences-word-processing](https://github.com/chrplr/goxpyriment/tree/main/examples/Hemispheric-differences-word-processing) | Lateralised recognition memory: words studied in LVF or RVF, tested centrally with old/new judgements | Federmeier & Benjamin (2005) |
| [Letter-size-illusion](https://github.com/chrplr/goxpyriment/tree/main/examples/Letter-size-illusion) | Compare heights of letters vs. mirror/pseudo-letters; replicates the letter height superiority illusion (two experiments) | New et al. (2015) |
| [lexical_decision](https://github.com/chrplr/goxpyriment/tree/main/examples/lexical_decision) | Decide whether a letter string is a word or a non-word (F / J keys); stimuli loaded from a CSV file |  |
| [LoT-geometry](https://github.com/chrplr/goxpyriment/tree/main/examples/LoT-geometry) | Comprehension of geometric primitives and rules; reproduces Amalric et al. (2017) | Amalric et al. (2017) |
| [Magnitude-Estimation-Luminosity](https://github.com/chrplr/goxpyriment/tree/main/examples/Magnitude-Estimation-Luminosity) | Stevens' magnitude estimation of luminance: assign a number to perceived brightness of grey disks | Stevens (1957) |
| [Memory-for-binary-sequences](https://github.com/chrplr/goxpyriment/tree/main/examples/Memory-for-binary-sequences) | Memory and reproduction of auditory binary sequences of varying complexity | Planton et al. (2021) |
| [Memory-Iconic-Sperling](https://github.com/chrplr/goxpyriment/tree/main/examples/Memory-Iconic-Sperling) | Partial-report procedure measuring capacity and duration of iconic (visual sensory) memory | Sperling (1960) |
| [Memory-Scanning](https://github.com/chrplr/goxpyriment/tree/main/examples/Memory-Scanning) | Hold a set of digits in memory; decide whether a probe was in the set — RT scales with set size | Sternberg (1966) |
| [Memory_span](https://github.com/chrplr/goxpyriment/tree/main/examples/Memory_span) | Adaptive staircase measuring immediate serial recall span for digits, letters, or words |  |
| [Mental-Logic-Card-Game](https://github.com/chrplr/goxpyriment/tree/main/examples/Mental-Logic-Card-Game) | Mental logic and inference task using a card-game paradigm |  |
| [Mental-Rotation-2D](https://github.com/chrplr/goxpyriment/tree/main/examples/Mental-Rotation-2D) | Decide whether two 3-D figures are identical or mirror images; RT increases linearly with angular disparity | Shepard & Metzler (1971) |
| [Mental-Rotation-3D](https://github.com/chrplr/goxpyriment/tree/main/examples/Mental-Rotation-3D) | Decide whether two 3D figures (procedurally generated assemblies of cubes) are identical or mirror images; RT increases linearly with angular disparity. | Shepard & Metzler (1971) |
| [Mouse-tracking](https://github.com/chrplr/goxpyriment/tree/main/examples/Mouse-tracking) | Mouse-trajectory paradigm: click the image matching a spoken-onset target while the cursor path is sampled at ~36 Hz, revealing continuous attraction toward phonological competitors | Spivey et al. (2005) |
| [Multiple-Object-Tracking](https://github.com/chrplr/goxpyriment/tree/main/examples/Multiple-Object-Tracking) | Track a subset of identical moving targets among distractors; evidence for a parallel tracking mechanism | Pylyshyn & Storm (1988) |
| [Number-Change-Detection](https://github.com/chrplr/goxpyriment/tree/main/examples/Number-Change-Detection) | Two concurrent dot-array streams (5 vs 20 dots) test infants' preference for numerosity change; experimenter codes looking direction in real time | Decarli, Piazza & Izard (2023) |
| [Number-Comparison](https://github.com/chrplr/goxpyriment/tree/main/examples/Number-Comparison) | Compare numerical magnitudes of digits and dot patterns; stimulus group (digits / regular / irregular / random) selected via GetParticipantInfo UI | Buckley & Gillman (1974) |
| [Number-Double-Digits-Comparison](https://github.com/chrplr/goxpyriment/tree/main/examples/Number-Double-Digits-Comparison) | Compare two-digit numbers against a fixed standard (55 or 65); two experiments with different response-key mappings | Dehaene et al. (1990) |
| [parity_decision](https://github.com/chrplr/goxpyriment/tree/main/examples/parity_decision) | Classify single digits (0–9) as even or odd (F / J keys) |  |
| [Perception-of-Temporal-Patterns](https://github.com/chrplr/goxpyriment/tree/main/examples/Perception-of-Temporal-Patterns) | Reproduction of isochronous and non-isochronous rhythmic patterns; tests internal clock induction and coding complexity | Povel & Essens (1985) |
| [picture_naming](https://github.com/chrplr/goxpyriment/tree/main/examples/picture_naming) | Vocal naming-latency task: a picture is named aloud while a microphone voice key measures RT from image onset; per-trial WAV files are saved for offline verification |  |
| [Posner-ANT](https://github.com/chrplr/goxpyriment/tree/main/examples/Posner-ANT) | Attention Network Task (vertical variant): flanker arrows above/below fixation measure alerting, orienting, and executive attention networks | Fan et al. (2009) |
| [Psychological-Refractory-Period](https://github.com/chrplr/goxpyriment/tree/main/examples/Psychological-Refractory-Period) | Two tasks presented in rapid succession; the second response is delayed when the SOA is short | Welford (1952) |
| [Retinotopy](https://github.com/chrplr/goxpyriment/tree/main/examples/Retinotopy) | HCP retinotopic mapping paradigm (ported from Python); flickering wedge/ring/bar stimuli for visual cortex mapping; run type selected via GetParticipantInfo UI |  |
| [Sensory-Threshold-Estimation-Auditory](https://github.com/chrplr/goxpyriment/tree/main/examples/Sensory-Threshold-Estimation-Auditory) | 1-up/2-down adaptive staircase with 2-IFC to estimate pure-tone hearing thresholds across multiple frequencies | Levitt (1971) |
| [shadowing](https://github.com/chrplr/goxpyriment/tree/main/examples/shadowing) | Vocal shadowing-latency task: repeat a played sound aloud while a microphone voice key measures the delay between audio onset and speech onset |  |
| [Simon_task](https://github.com/chrplr/goxpyriment/tree/main/examples/Simon_task) | Identify colour of a square regardless of its screen position; congruent trials are faster | Simon (1969) |
| [simple_reaction_times](https://github.com/chrplr/goxpyriment/tree/main/examples/simple_reaction_times) | 20-trial simple RT task: press any key as quickly as possible when a target appears |  |
| [SNARC-effect](https://github.com/chrplr/goxpyriment/tree/main/examples/SNARC-effect) | Parity judgment on digits 0–9 with reversed key mappings across blocks; demonstrates the SNARC effect | Dehaene, Bossini & Giraux (1993) |
| [Statistical-Learning-Auditory](https://github.com/chrplr/goxpyriment/tree/main/examples/Statistical-Learning-Auditory) | Statistical learning of tone sequences: exposure to a continuous tone stream with structured transitional probabilities, probed with 2AFC or head-turn preference | Saffran et al. (1999) |
| [Statistical-Learning-Community-Structure](https://github.com/chrplr/goxpyriment/tree/main/examples/Statistical-Learning-Community-Structure) | Implicit learning of community structure in a continuous visual sequence, with random-walk exposure and spacebar-based event segmentation | Schapiro et al. (2013) |
| [Statistical-Learning-Visual](https://github.com/chrplr/goxpyriment/tree/main/examples/Statistical-Learning-Visual) | Implicit learning of statistical regularities in a shape stream, probed with forced-choice and RT tests | Turk-Browne et al. (2005) |
| [Stroop_task](https://github.com/chrplr/goxpyriment/tree/main/examples/Stroop_task) | Name the ink colour of colour words; incongruent trials (e.g. RED in blue ink) are slower | Stroop (1935) |
| [Subliminal-Priming](https://github.com/chrplr/goxpyriment/tree/main/examples/Subliminal-Priming) | Masked word priming: words rendered invisible by surrounding masks still influence processing | Dehaene et al. (2004) |
| [Temporal-Integration-Word-Recognition](https://github.com/chrplr/goxpyriment/tree/main/examples/Temporal-Integration-Word-Recognition) | Alternating odd/even letter components at variable SOA; Exp 1 (subjective report: 0/1/2 words perceived) and Exp 2 (lexical decision with RT); experiment selected via GetParticipantInfo UI | Forget et al. (2010) |
| [Trubutschek_Unconscious_Working_Memory](https://github.com/chrplr/goxpyriment/tree/main/examples/Trubutschek_Unconscious_Working_Memory) | Probe access to briefly presented stimuli below and above the threshold of consciousness | Trübutschek et al. (2017) |
| [Visual-Illusion-Lilac-Chaser](https://github.com/chrplr/goxpyriment/tree/main/examples/Visual-Illusion-Lilac-Chaser) | Lilac chaser illusion: a ring of disappearing disks produces a rotating green afterimage |  |
| [Visual-Search](https://github.com/chrplr/goxpyriment/tree/main/examples/Visual-Search) | Feature vs. conjunction visual search across set sizes (4/12/24), measuring how reaction time scales with the number of distractors | Treisman & Gelade (1980) |
<!-- END:experiments -->

---

## Demonstrations

Visual illusions, interactive showcases, and minimal templates. Most do not write a data file.

<!-- BEGIN:demos -->
| Directory | Description |
|-----------|-------------|
| [hello_world](https://github.com/chrplr/goxpyriment/tree/main/examples/hello_world) | Simplest possible goxpyriment program — good starting point for new users |
| [Motion-Blur](https://github.com/chrplr/goxpyriment/tree/main/examples/Motion-Blur) | Motion blur vs. phantom array demo: animated bar demonstrates retinal blur and the strobe effect at 60 Hz |
| [play_two_gvvideos](https://github.com/chrplr/goxpyriment/tree/main/examples/play_two_gvvideos) | Plays two .gv videos side by side, synchronised, logging keypresses with their time relative to video onset |
| [play_two_videos](https://github.com/chrplr/goxpyriment/tree/main/examples/play_two_videos) | Plays two MPEG videos side by side, synchronised, logging keypresses with their time relative to video onset |
| [play_videos](https://github.com/chrplr/goxpyriment/tree/main/examples/play_videos) | Plays a sequence of MPEG video files found in the assets folder |
| [random-dot-stereogram](https://github.com/chrplr/goxpyriment/tree/main/examples/random-dot-stereogram) | Random-dot stereogram that reveals a 3-D shape when fused binocularly |
| [simple-rt-example](https://github.com/chrplr/goxpyriment/tree/main/examples/simple-rt-example) | Minimal 10-trial reaction-time loop using ShowTS / GetKeyEventTS for hardware-timestamped RT |
| [stimuli_extras](https://github.com/chrplr/goxpyriment/tree/main/examples/stimuli_extras) | Showcase of advanced stimuli: visual mask, Gabor patch, dot cloud, stimulus circle, thermometer |
| [Visual-Angle-Calibration](https://github.com/chrplr/goxpyriment/tree/main/examples/Visual-Angle-Calibration) | Draws concentric rings at 2°, 5°, and 10° of visual angle for a quick sanity-check of the `units.Monitor` calibration |
| [Visual-Illusion-Ebbginghaus](https://github.com/chrplr/goxpyriment/tree/main/examples/Visual-Illusion-Ebbginghaus) | Animated Ebbinghaus (Titchener circles) size-contrast illusion |
| [Visual-Illusion-Kanizsa](https://github.com/chrplr/goxpyriment/tree/main/examples/Visual-Illusion-Kanizsa) | Kanizsa illusory-contour square: a square is perceived where none is drawn |
| [Visual-Persistence](https://github.com/chrplr/goxpyriment/tree/main/examples/Visual-Persistence) | Persistence-of-vision illusion: an image seen only through moving vertical slits integrates into a whole when the eyes track a moving dot |
<!-- END:demos -->

---

## Technical Tests

Hardware, timing, and feature tests live in the [`tests/`](../tests/) directory at the repository root (separate Go module). Build them with `make tests`.

<!-- BEGIN:tests -->
| Directory | Description |
|-----------|-------------|
| [bouncing_gv_movies](https://github.com/chrplr/goxpyriment/tree/main/tests/bouncing_gv_movies) | Two .gv movies bounce around the screen, additively blend on overlap, fire beep tones at frame markers, and log full state to the data file on every key press; demonstrates every runtime mutator media.Movie exposes (Pause, PauseWithLoop, SetRate, SeekFrame, SetPosition, SetSize, SetBlendMode) with the multi-movie synchrony invariant preserved via BeginBurst/EndBurst |
| [GvFiles](https://github.com/chrplr/goxpyriment/tree/main/tests/GvFiles) | Plays sequences of .gv movies while flashing a white square and pulsing DLP-IO8 line 0 at frame markers, for photodiode+oscilloscope display/TTL synchrony checks (port of PsyScope WhiteSquareSync) |
| [set_fullscreen](https://github.com/chrplr/goxpyriment/tree/main/tests/set_fullscreen) | Minimal SDL program that toggles exclusive fullscreen mode — sanity check for the fullscreen video path |
| [sync_two_gv_movies](https://github.com/chrplr/goxpyriment/tree/main/tests/sync_two_gv_movies) | Two .gv movies played side by side under a shared MasterClock; demonstrates media.MovieManager multi-movie sync, burst-pause, Movie[At]/Movie[AtDisplay], and Display[Onset/Offset] events with optional TTL-trigger wiring |
| [tearing_test](https://github.com/chrplr/goxpyriment/tree/main/tests/tearing_test) | Full-height white bar sweeping horizontally to reveal screen tearing; prints frame-interval statistics on exit |
| [test_av_sync](https://github.com/chrplr/goxpyriment/tree/main/tests/test_av_sync) | Verifies PlaySyncedWithFlip aligns audio onset with the VSYNC flip (icon flash + 1 kHz tone each second) |
| [test_canvas](https://github.com/chrplr/goxpyriment/tree/main/tests/test_canvas) | Drawing on an off-screen `Canvas` surface before presenting it in one frame |
| [test_follow_mouse](https://github.com/chrplr/goxpyriment/tree/main/tests/test_follow_mouse) | A white dot follows the mouse cursor every frame — minimal real-time input loop |
| [test_ft232h](https://github.com/chrplr/goxpyriment/tree/main/tests/test_ft232h) | Exercises an Adafruit FT232H breakout as an 8-bit TTL trigger device (output + input loopback) |
| [test_fullscreen](https://github.com/chrplr/goxpyriment/tree/main/tests/test_fullscreen) | Sanity check for fullscreen rendering through the control facade |
| [test_getinfo](https://github.com/chrplr/goxpyriment/tree/main/tests/test_getinfo) | Demonstrates the `GetParticipantInfo` dialog: collects participant demographics and monitor characteristics before the experiment window opens |
| [test_joystick](https://github.com/chrplr/goxpyriment/tree/main/tests/test_joystick) | Move a cursor with a joystick (analog axes, dead zone, button-click to stop) — joystick input handling |
| [test_keyboard](https://github.com/chrplr/goxpyriment/tree/main/tests/test_keyboard) | Demonstrates every keyboard input method provided by the framework, section by section |
| [test_labjackt4](https://github.com/chrplr/goxpyriment/tree/main/tests/test_labjackt4) | Exercises a LabJack T4 as an 8-bit TTL trigger device (output + input loopback) |
| [test_linuxgpio](https://github.com/chrplr/goxpyriment/tree/main/tests/test_linuxgpio) | Exercises a Linux GPIO character-device (v2) trigger as an 8-bit TTL device (output + input loopback) |
| [test_menu](https://github.com/chrplr/goxpyriment/tree/main/tests/test_menu) | Demonstrates the stimuli.Menu widget |
| [test_mouse_audio_feedback](https://github.com/chrplr/goxpyriment/tree/main/tests/test_mouse_audio_feedback) | Left/right mouse clicks trigger ping/buzzer audio; useful for testing sound output |
| [test_parallel_port](https://github.com/chrplr/goxpyriment/tree/main/tests/test_parallel_port) | Reads and writes a Linux LPT parallel port via the ppdev kernel interface |
| [test_playgv](https://github.com/chrplr/goxpyriment/tree/main/tests/test_playgv) | Plays a single .gv video through the control facade |
| [test_stream_images](https://github.com/chrplr/goxpyriment/tree/main/tests/test_stream_images) | High-precision RSVP loop streaming a sequence of images (PresentStreamOfImages) |
| [test_stream_text](https://github.com/chrplr/goxpyriment/tree/main/tests/test_stream_text) | High-precision RSVP loop streaming a sequence of text strings |
| [test_text_input](https://github.com/chrplr/goxpyriment/tree/main/tests/test_text_input) | Demonstration of the `TextInput` stimulus collecting free-text keyboard input |
| [Timing-Tests](https://github.com/chrplr/goxpyriment/tree/main/tests/Timing-Tests) | Hardware timing verification suite: eleven sub-tests (frame jitter, true refresh rate, audio latency, photodiode/BBTK/oscilloscope checks) selected with -test <name> |
<!-- END:tests -->
