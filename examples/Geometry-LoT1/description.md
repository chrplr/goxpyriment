# Geometric intruder task — Experiment 2 of Sablé-Meyer et al. (2021)

Specification for a goxpyriment reproduction of **Experiment 2** ("French adults,
experiment 2", n = 117) of:

> Sablé-Meyer, M., Fagot, J., Caparos, S., van Kerkoerle, T., Amalric, M. &
> Dehaene, S. (2021). Sensitivity to geometric shape regularity in humans and
> baboons: A putative signature of human singularity. *PNAS* 118(16),
> e2023123118. https://doi.org/10.1073/pnas.2023123118

Sources: main paper (Fig. 1, *Results* §"Design of the Geometric Intruder Task",
*Materials and Methods* §"Reference Shapes", "Deviant Shapes", "Variations in
Orientation and Size") and SI Appendix (§"Adults, experiment 1", "Adults,
experiment 2", "Possible role of feedback", Table S1). Original implementation:
jsPsych, online, at https://neurospin-data.cea.fr/exp/mathias-sable-meyer/oddball/.

Everything the paper leaves unspecified is listed in §7 with the value chosen
for this implementation, so the program can be written without further
guesswork and the choices can be revisited later.

---

## 1. Task in one paragraph

On every trial six white quadrilaterals are shown on a black screen in two rows
of three. Five of them are the *same* shape (up to a random rotation and a
random scale change) and one is an *intruder* whose bottom-right vertex has been
displaced. The participant clicks (mouse or touch) on the intruder as fast and as
accurately as possible; the clicked shape is then coloured green (correct) or
red (error), the correct shape is coloured green, and a rising (correct) or
falling (error) tone is played. The measure of interest is the error rate (and
RT) as a function of the geometric regularity of the reference shape: 11
quadrilaterals from square to fully irregular.

## 2. The 11 reference shapes (Table S1 of the SI)

Coordinates are in arbitrary units, **bottom-left vertex at (0, 0)**, y pointing
**up** (same convention as goxpyriment). Each shape is the closed polygon
`BL → TL → TR → BR` (listed clockwise). The bottom edge `BL→BR` lies on the
x-axis; its length is 1.5 for 9 shapes (the square and rhombus are the
exceptions — they have too few degrees of freedom). The average of the 6
pairwise vertex distances is 1.434 for every shape (this is the size-matching
criterion the authors used). Perimeter and area are *not* matched (columns are
relative to the rectangle).

| shape          | TL (x, y)        | TR (x, y)       | BR (x, y)  | avg pair dist | perimeter | area  | # properties |
|----------------|------------------|-----------------|------------|---------------|-----------|-------|--------------|
| square         | (0.000, 1.260)   | (1.260, 1.260)  | (1.260, 0) | 1.434 | 1.008 | 1.058 | 19 |
| rectangle      | (0.000, 1.000)   | (1.500, 1.000)  | (1.500, 0) | 1.434 | 1.000 | 1.000 | 15 |
| rhombus        | (−0.908, 0.931)  | (0.392, 0.931)  | (1.300, 0) | 1.434 | 1.040 | 0.807 | 9  |
| parallelogram  | (−0.517, 0.896)  | (0.983, 0.896)  | (1.500, 0) | 1.434 | 1.014 | 0.896 | 7  |
| right-kite     | (0.529, 1.404)   | (1.500, 1.038)  | (1.500, 0) | 1.434 | 1.015 | 1.038 | 7  |
| iso-trapezoid  | (0.365, 1.362)   | (1.109, 1.362)  | (1.500, 0) | 1.433 | 1.014 | 1.019 | 5  |
| kite           | (0.766, 1.290)   | (1.770, 1.007)  | (1.500, 0) | 1.434 | 1.017 | 1.007 | 5  |
| right-hinge    | (−0.296, 0.634)  | (1.064, 1.268)  | (1.500, 0) | 1.433 | 1.008 | 0.984 | 2  |
| hinge          | (−0.248, 0.533)  | (0.980, 1.393)  | (1.500, 0) | 1.434 | 1.015 | 0.986 | 1  |
| trapezoid      | (−0.227, 1.200)  | (0.724, 1.200)  | (1.500, 0) | 1.434 | 1.020 | 0.980 | 1  |
| irregular      | (−0.450, 1.058)  | (0.227, 1.240)  | (1.500, 0) | 1.434 | 1.026 | 0.886 | 0  |

The row order above is the "predicted regularity" order of Fig. 1A (number of
geometric properties: parallel sides, equal sides, equal angles, right angles).
The empirical difficulty order in Exp. 2 (Fig. 1C/D, error rate rising from
≈2 % to ≈40 %) is: square, rectangle, iso-trapezoid, parallelogram, rhombus,
kite, right-kite, right-hinge, hinge, trapezoid, irregular.

All 11 reference polygons **and all 44 deviants** (§3) are convex (checked
numerically from the coordinates above), so the triangle-fan fill of
`stimuli.Shape` renders them correctly. The training polygons of §5.2 include a
concave arrow-head, which needs a two-triangle split along its inner diagonal
(or a `PolyLine`-free custom fill) — the only non-convex shape in the whole
experiment.

**Note on orientation.** *Materials and Methods* says the 0° orientation is "the
one where the top edge was horizontal", while Table S1 (whose coordinates are
what the online experiment used) puts the *bottom* edge on the x-axis — the two
statements agree only for the 6 shapes whose TL and TR share a y. This
implementation takes the Table S1 coordinates as-is as the 0° orientation
(bottom edge horizontal). The random rotations of ±5/15/25° make the difference
invisible to the participant.

## 3. Deviant (intruder) shapes

For each reference shape, four deviants are generated by moving **only the
bottom-right vertex BR**, by a fixed Euclidean distance

    d = 0.30 × 1.434 = 0.430   (30 % of the matched average pairwise distance)

(the sequence experiment used 55 %; every other experiment in the paper,
including Exp. 2, used 30 %). Let `L = |BR|` be the length of the bottom edge
(1.5, or 1.26 / 1.3 for square / rhombus).

| deviant id | name        | new BR                                             | what it breaks            |
|------------|-------------|----------------------------------------------------|---------------------------|
| 0 | `shorter`  | (L − d, 0)                                                   | length of bottom edge (and angles) |
| 1 | `longer`   | (L + d, 0)                                                   | length of bottom edge (and angles) |
| 2 | `rot_up`   | rotate BR about BL=(0,0) by +θ, i.e. (L cos θ, +L sin θ)      | direction of bottom edge (parallelism, angles); length preserved |
| 3 | `rot_down` | rotate BR about BL by −θ, i.e. (L cos θ, −L sin θ)            | same, other way |

with `θ = 2·asin(d / (2L))` so that the chord `|BR' − BR| = d`
(θ = 16.49° for L = 1.5, 19.66° for the square, 19.05° for the rhombus). The
paper says "along a circle centred on the bottom-left vertex" and "the distance
of the deviant position from the correct vertex was fixed"; if arc length were
meant instead of chord length the angle would differ by < 0.1°, which is
immaterial. TL and TR are untouched, so the deviant of a canonical shape is not
in the set of reference shapes (e.g. a rectangle "shorter" deviant is a
right-trapezoid).

This is the diagram in the bottom-right cell of Fig. 1A: two orange dots on the
line of the bottom edge (inside and outside), two on the circle through BR
centred on BL (above and below).

## 4. Display of one trial

* Background black, shapes filled solid **white** (Exp. 1 used black on white;
  Exp. 2 switched to white on black to match the baboon set-up).
* Six slots arranged in **2 rows × 3 columns** (Fig. 1B), centred on the
  screen. Slot index convention: 0 1 2 on the top row (left→right), 3 4 5 on the
  bottom row.
* **Composition (the "presentation" factor):**
  * `canonical` — 5 instances of the reference shape + 1 deviant;
  * `swapped`   — 5 instances of the *deviant* + 1 reference shape.
  In both cases the participant must click the odd one out; in the swapped
  case that is the *reference* shape. This factor was introduced in Exp. 2
  precisely so that "the most irregular shape" is not always the answer.
* **Outlier position:** uniformly random among the 6 slots (the paper says
  "randomized"; balance it so that each slot is the outlier equally often
  within each participant — see §7).
* **Per-shape transformations** (Materials and Methods, "Variations in
  Orientation and Size"). For each trial draw one random permutation of the six
  rotation angles and, independently, one random permutation of the six scale
  factors, and assign the i-th element of each to slot i — so within a display
  all six shapes have *different* rotations and *different* sizes, and every
  display uses each value exactly once:

        rotations (degrees, about the shape's centre): [−25, −15, −5, +5, +15, +25]
        scales (multiplying every edge length):        [0.875, 0.925, 0.975, 1.025, 1.075, 1.125]

  0° is deliberately absent so that no shape is ever aligned with the screen
  edges, and the range stays below ±45° so that a rotated square still reads as a
  square and not as a rhombus (the same does not hold for a trapezoid).
  Positive angle = counter-clockwise (mathematical convention, y up).
* Each shape is **centred on its centre of mass** in its slot: translate the
  polygon so that its area centroid is at the slot centre, then scale, then
  rotate about that centre. (Table S1 coordinates are relative to BL, not
  centred; for the deviants the centroid must be recomputed from the deviant
  vertices, otherwise the outlier could be spotted by its offset.)
* Absolute size is not given in the paper (it ran in the participants' browsers;
  the sequence experiment mentions 150–225 px). See §7 for the value chosen.
* Before the click the mouse cursor is visible (the task is a pointing task).
  No fixation cross is mentioned; none is used.

## 5. Procedure

Overall order: instructions → training block A → training block B → 88 test
trials → end screen. Total ≈ 15 min online in the original.

### 5.1 Instructions
Exp. 1 wording (Exp. 2 identical): six shapes are shown, five are identical
(apart from size and orientation) and one is different; click on the one that is
different as fast and as accurately as possible. Mention that a green/red colour
and a sound will indicate whether the answer was right.

### 5.2 Training (SI, "Adults, experiment 2", point 4)
Two consecutive blocks, each repeated until the participant reaches **≥ 80 %
correct** on the block:

* **Block A — 10 trials with pictures.** Same 6-slot oddball layout and feedback
  as the test, but each trial uses one of the 10 *image pairs* used to train the
  baboons (Fig. 3B, "initial training", stage "Train 5"): five copies of one image
  of a pair plus one copy of the other. The ten baboon pairs were: orange vs
  cyan blurred disc; watermelon slice vs apple; letters "A" vs "K"; two
  different 4-point star shapes; horizontal vs vertical grating; two half-disc
  shapes; filled vs outline Y-shape; checkerboard vs dot texture; grey gradient
  disc vs grey polygon; dotted vs solid vertical line. The exact bitmaps are not
  published; any 10 pairs of clearly distinguishable pictures serve the purpose
  (the block exists only to teach the click-the-odd-one-out rule). Images get the
  same random rotations as the shapes (and, in the original, no scaling — an
  acknowledged bug in the human versions; scale them anyway, it is harmless).
  Each of the 10 pairs is used once; which member of the pair is the intruder
  is random.
* **Block B — 6 trials with easy polygons.** The three "generalization 2"
  polygon pairs of the baboon experiment (Fig. 3A/B): (concave arrow-head vs
  regular pentagon), (square vs right-pointing triangle), (right-pointing
  triangle vs pentagon). Each pair is used twice, once with each member as the
  intruder, with the standard random rotation and scaling. Exact vertex lists
  are not published; use a regular pentagon, a square, an isosceles triangle
  and a chevron/arrow-head of comparable size.

Training data are saved (flagged with `phase = trainA` / `trainB` and the
repetition count) but are not part of the analysis.

### 5.3 Test block: 88 trials
Full factorial of

    11 shapes × 4 deviants × 2 presentations (canonical, swapped) = 88 trials,

each combination exactly once, in a **single fully randomised order** (not
blocked by shape: the SI analyses "each participant's very first trial with a
given shape", which presupposes interleaving). Outlier slot, rotation
permutation and scale permutation are drawn per trial as in §4.

No breaks are mentioned; none are needed for ~10 minutes of testing.

### 5.4 One trial
1. Clear to black; optional inter-trial blank (see §7).
2. Draw the six shapes; flip and record the onset timestamp.
3. Wait for a mouse-button press (or touch) with no timeout. The response is
   the slot whose *shape* contains the click point. (Choice for this
   implementation: a click on empty background is ignored and the wait
   continues — see §7; alternatively accept the nearest slot.)
4. RT = press timestamp − onset timestamp (SDL event clock, via the goxpyriment
   mouse API — not wall-clock deltas).
5. Feedback, shown for a fixed duration (§7): the clicked shape is recoloured
   **green if correct, red if incorrect**, and the correct shape is coloured
   green (so on an error two shapes change colour; SI "Possible role of
   feedback": "filled in green for geometric shapes, surrounded with a green
   square for training images"). Simultaneously play a **rising tone** (correct)
   or **falling tone** (incorrect) — e.g. two short pure tones, 400→800 Hz vs
   800→400 Hz, or a frequency sweep; the exact sounds are not published.
6. Log the trial (§6).

ESC aborts the experiment at any time (goxpyriment default).

## 6. Data to record (one row per trial)

`subject_id` is prepended by `results`. Columns:

| column | content |
|---|---|
| `trial` | 1-based index within the phase |
| `phase` | `trainA`, `trainB`, `test` |
| `block_repeat` | how many times this training block has been (re)started (0 for test) |
| `shape` | reference shape name (or image/polygon pair id in training) |
| `deviant` | `shorter` / `longer` / `rot_up` / `rot_down` (empty in training) |
| `presentation` | `canonical` / `swapped` |
| `outlier_slot` | 0–5 |
| `outlier_rotation`, `outlier_scale` | the values that landed on the outlier (the SI analyses these two as factors: Table S2) |
| `rotations`, `scales` | the full six-element permutations, slot order, as `;`-separated strings |
| `response_slot` | slot clicked (0–5) |
| `correct` | bool |
| `rt` | ms from display onset to button press |
| `onset_ts` | display onset timestamp (µs, SDL clock) for later audit |

Analysis in the paper: trials with RT above the overall 99th percentile removed;
error rate per shape (Fig. 1D), per shape × deviant (44 cells, used for the
symbolic and CNN model fits); ANOVAs on shape, outlier slot, deviant type,
outlier scale, outlier rotation (Table S2).

## 7. Values not given in the paper — choices for this implementation

| parameter | choice | rationale |
|---|---|---|
| Shape size on screen | 1 table unit = `H/8` px where H is the window height (so the 1.5-unit bottom edge ≈ 144 px at 768 lines, ≈ 200 px at 1080). Scale 1.0 shapes then fit in a slot ≈ `H/4` tall with the ±12.5 % scale and ±25° rotation. | Fig. 1B proportions; comparable to the 150–225 px of the sequence experiment. |
| Slot grid | columns at x = −W/3.5, 0, +W/3.5 ; rows at y = +H/4, −H/4 (centre-relative, y up) — recomputed from the actual window size | fills the display like Fig. 1B without shapes touching |
| Slot assignment of the outlier | balanced: within the test block each slot is the outlier on 88/6 ≈ 14–15 trials (shuffle a list of slots), not i.i.d. uniform | Table S2 treats slot as a 6-level factor |
| Inter-trial blank | 500 ms black | jsPsych default-like; none stated |
| Feedback duration | 700 ms | not stated; long enough to see the colours and hear the tone |
| Feedback tones | rising: 440→880 Hz, falling: 880→440 Hz, 2 × 100 ms segments (or a linear sweep), generated with the audio package, no external files | "ascending/descending pitch" |
| Click on background | ignored, keep waiting | avoids scoring an accidental click; RT still measured from onset |
| Click hit-test | point-in-polygon on the *transformed* vertices (no bounding box), falling back to nearest slot centre if the click is within 1.5 × the slot half-size of a centre | shapes are the targets; small misses near the edge should still count |
| Response device | mouse button 1 (touch events map to mouse in SDL) | "mouse or touchscreen" |
| Training images | 10 embedded PNG pairs (`//go:embed`), our own drawings | originals unpublished |
| Response timeout | none | online original had none; 99th-percentile trimming in analysis |
| Windowed/fullscreen, display, subject id | standard `-w`, `-d`, `-s` flags of `control.NewExperimentFromFlags` | repo convention |
| Which shape is the "reference" in a `swapped` trial | the shape at the outlier slot is the reference; the five others are the deviant | by definition |

## 8. Machine-readable parameters

```json
{
  "experiment": "Sable-Meyer2021_Exp2_intruder",
  "units": "arbitrary; bottom-left vertex at origin, y up, polygon order BL,TL,TR,BR",
  "shapes": {
    "square":        {"TL": [0.000, 1.260], "TR": [1.260, 1.260], "BR": [1.260, 0], "nProperties": 19},
    "rectangle":     {"TL": [0.000, 1.000], "TR": [1.500, 1.000], "BR": [1.500, 0], "nProperties": 15},
    "rhombus":       {"TL": [-0.908, 0.931], "TR": [0.392, 0.931], "BR": [1.300, 0], "nProperties": 9},
    "parallelogram": {"TL": [-0.517, 0.896], "TR": [0.983, 0.896], "BR": [1.500, 0], "nProperties": 7},
    "right-kite":    {"TL": [0.529, 1.404], "TR": [1.500, 1.038], "BR": [1.500, 0], "nProperties": 7},
    "iso-trapezoid": {"TL": [0.365, 1.362], "TR": [1.109, 1.362], "BR": [1.500, 0], "nProperties": 5},
    "kite":          {"TL": [0.766, 1.290], "TR": [1.770, 1.007], "BR": [1.500, 0], "nProperties": 5},
    "right-hinge":   {"TL": [-0.296, 0.634], "TR": [1.064, 1.268], "BR": [1.500, 0], "nProperties": 2},
    "hinge":         {"TL": [-0.248, 0.533], "TR": [0.980, 1.393], "BR": [1.500, 0], "nProperties": 1},
    "trapezoid":     {"TL": [-0.227, 1.200], "TR": [0.724, 1.200], "BR": [1.500, 0], "nProperties": 1},
    "irregular":     {"TL": [-0.450, 1.058], "TR": [0.227, 1.240], "BR": [1.500, 0], "nProperties": 0}
  },
  "avgPairwiseDistance": 1.434,
  "deviant": {
    "movedVertex": "BR",
    "displacementFraction": 0.30,
    "displacement": 0.430,
    "types": ["shorter", "longer", "rot_up", "rot_down"],
    "rotationAngleRule": "theta = 2*asin(d/(2*L)), L = |BR|, chord length = d"
  },
  "display": {
    "background": "black",
    "shapeColor": "white",
    "feedbackCorrect": "green",
    "feedbackIncorrect": "red",
    "layout": {"rows": 2, "cols": 3, "slotOrder": "row-major, top-left = 0"},
    "rotationsDeg": [-25, -15, -5, 5, 15, 25],
    "scales": [0.875, 0.925, 0.975, 1.025, 1.075, 1.125],
    "permutationRule": "one random permutation of each list per trial, element i to slot i",
    "centering": "area centroid of the (possibly deviant) polygon at the slot centre",
    "unitPx": "windowHeight/8",
    "slotX": ["-W/3.5", "0", "+W/3.5"],
    "slotY": ["+H/4", "-H/4"]
  },
  "design": {
    "factors": {"shape": 11, "deviant": 4, "presentation": ["canonical", "swapped"]},
    "testTrials": 88,
    "order": "fully random",
    "outlierSlot": "balanced random over 6 slots",
    "training": [
      {"name": "trainA", "trials": 10, "stimuli": "10 image pairs", "criterion": 0.80},
      {"name": "trainB", "trials": 6,  "stimuli": "3 easy polygon pairs, each member intruder once", "criterion": 0.80}
    ]
  },
  "timing": {
    "interTrialBlankMs": 500,
    "responseTimeoutMs": null,
    "feedbackMs": 700,
    "feedbackTone": {"correct": "rising 440->880 Hz", "incorrect": "falling 880->440 Hz", "durationMs": 200}
  },
  "response": {"device": "mouse/touch", "button": 1, "hitTest": "point-in-transformed-polygon, nearest-slot fallback", "backgroundClick": "ignored"}
}
```

## 9. Experiment 1 variant (`-exp 1`)

Exp. 1 (n = 605) is Exp. 2 minus the five changes listed in the SI, so the
same program runs it from a flag:

| | Exp. 1 | Exp. 2 |
|---|---|---|
| colours | black shapes on white background | white on black |
| layout | six shapes on a circle "as big as the screen permitted", fixation mark at the centre (Fig. 1B left) — implemented as slot *i* at angle 60°·*i* from the right, radius `H/2 − 1 unit` | 2 × 3 grid |
| composition | canonical only | canonical + swapped |
| test trials | 11 × 4 = 44 | 88 |
| training | 2 trials, pairs drawn at random from the 3 easy-polygon pairs; no criterion is stated, so the block runs once | 10 + 6, each block repeated to ≥ 80 % |
| lottery page | — | (recruitment only, not part of the task) |

Everything else — shapes, deviants, 30 % displacement, rotations, scales,
click response, green/red + rising/falling feedback, 99th-percentile RT
trimming — is shared. Error rates across the 11 shapes correlated at r² = 0.97
between the two experiments (Fig. 1D).

## 10. Optional extras worth building alongside

* A `-demo` / `-shapes` flag that draws the 11 reference shapes with their four
  deviants in a grid (Fig. 1A) — the quickest way to check the coordinate
  transcription and the deviant geometry by eye.
* The **symbolic model** of the paper (SI, "Definition of the symbolic model")
  as an `analyse/` script: for each quadrilateral a 22-bit vector — 6 bits equal
  edge lengths (pairwise), 6 bits parallel edges, 6 bits equal angles, 4 bits
  right angles — with a 12.5 % tolerance (lengths: ratio − 1 < θ; angles:
  difference < θ × 90°); predicted difficulty = L1 distance between reference
  and deviant vectors. It reproduces the human error ordering (r² ≈ 0.59 on
  Exp. 2) and is a natural sanity check of collected data.
