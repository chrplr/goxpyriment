To implement the experiments described in the paper by New et al. (2015), a programmer will need to focus on precise timing, stimulus randomization, and response logging.

Below are the technical specifications for both experiments.

---

## Experiment 1: Letter Height Perception

This experiment investigates if individual letters are perceived as taller than pseudoletters or mirror-image letters.

### 1. Stimuli and Assets

* 
**Target Stimuli:** 9 lowercase letters in **Times New Roman** (a, c, e, m, r, s, v, w, z).


* **Control Stimuli:**
* 
**Mirror Letters:** Vertical mirror symmetry transformations of the 9 target letters.


* 
**Pseudoletters:** Reconfigured features of the original letters, matched for height, pixel count, and contiguous pixels.




* **Sizes:**
* 
**Small:** $0.28^{\circ}$ vertical visual angle.


* 
**Tall:** $0.30^{\circ}$ vertical visual angle.




* 
**Training Set:** 3 letters (u, n, x) not used in the main session.



### 2. Trial Procedure (Logic Flow)

1. 
**Fixation:** Display a central cross for **200 ms**.


2. 
**Stimulus Display:** Present two horizontally aligned stimuli simultaneously for **700 ms**.


* Position: 2.75° to the left and right of the center.


* Pairing: One stimulus is always a letter; the other is a letter, mirror letter, or pseudoletter.




3. 
**Response Window:** Clear the screen and wait for a keyboard input:


* 
**Down Arrow:** Stimuli are identical height.


* 
**Left/Right Arrow:** Indicates which stimulus (left or right) is taller.




4. 
**Inter-Trial Interval (ITI):** **750 ms** after response.



### 3. Experimental Design

* 
**Total Trials:** 648 (216 unique configurations repeated 3 times).


* **Conditions:**
* 
**Same Height (50%):** Both stimuli are the same physical size.


* 
**Different Height (50%):** One stimulus is tall ($0.30^{\circ}$), one is small ($0.28^{\circ}$).




* 
**Randomization:** Counterbalance the screen position (left/right) of the letter vs. control.



---

## Experiment 2: Word Height Perception

This experiment scales the task to full words to see if lexical status (being a real word) increases the height illusion.

### 1. Stimuli and Assets

* 
**Target Stimuli:** 9 uppercase French words (BATEAU, BUREAU, CAMION, CANAL, GENOU, JARDIN, LAPIN, PARFUM, TUYAU) in **Times New Roman**.


* **Control Stimuli:**
* 
**Mirror Words:** Vertical mirror symmetry of the target words.


* 
**Reversed-Syllable Pseudowords:** Syllables swapped (e.g., CANAL becomes NALCA).


* 
**Nonwords:** Strings of pseudoletters matched for width, height, and pixel number.




* **Sizes:**
* 
**Small:** $0.4^{\circ}$ vertical visual angle.


* 
**Tall:** $0.44^{\circ}$ vertical visual angle.




* 
**Training Set:** 3 words (RADIO, PAPIER, MAISON) not used in the main session.



### 2. Trial Procedure (Logic Flow)

The logic is identical to Experiment 1 with two timing changes:

1. 
**Stimulus Display:** Shortened to **500 ms**.


2. 
**Eccentricity:** Stimuli are placed **0.9°** from the center.


3. 
**Response:** Same arrow key mapping as Experiment 1.



### 3. Experimental Design

* 
**Total Trials:** 864 (288 unique configurations repeated 3 times).


* **Conditions:**
* 
**Same Height (50%):** Word and control stimulus are identical in size.


* 
**Different Height (50%):** Word and control stimulus differ in size.




* 
**Randomization:** Position (left/right) must be counterbalanced.



---

### Programmer Checklist

* 
**Fixation Point:** Ensure the cross is exactly centered.


* 
**Logging:** Record the trial type, the physical heights used, the user's choice (Left, Right, or Same), and the reaction time.


* 
**Training Module:** Implement a block that requires **80% accuracy** on same-category comparisons (letter-letter or word-word) before the main experiment starts.


* 
**Visual Angle:** The programmer must calculate pixel heights based on the participant's distance from the monitor (**64 cm**) to ensure the vertical angles ($0.28^{\circ}$–$0.44^{\circ}$) are accurate.

To help your programmer build this study, here is a structured data template and a technical specification for the experiment logic.

### Stimulus Pairing Template

The programmer will need a configuration file (like a CSV or JSON) to manage the trials. Below is a sample structure for the pairings described in the paper:

| Stimulus A (Anchor) | Stimulus B (Comparison) | Trial Type | Condition (Experiment 1) | Source Reference |
| --- | --- | --- | --- | --- |
| 'a' | 'a' | Same Height | Letter-Letter

 |  |
| 'a' | Pseudoletter 'a' | Same Height | Letter-Pseudoletter

 |  |
| 'a' | Mirror 'a' | Different Height | Letter-Mirror Letter

 |  |
| 'CANAL' | 'NALCA' | Same Height | Word-Reversed Syllable

 |  |
| 'CANAL' | Nonword 'CANAL' | Different Height | Word-Nonword

 |  |

---

### Implementation Guide for the Programmer

#### 1. Visual Geometry Calculations

The programmer must calibrate the screen to ensure the "height" is based on **Visual Angle**, not just raw pixels.

* 
**Participant Distance ($d$):** 64 cm.


* **Vertical Angle ($\theta$):** Use the formula 
$$Height = 2 \times d \times \tan(\theta/2)$$


* 
**Experiment 1 Heights:** Small = $0.28^{\circ}$, Tall = $0.30^{\circ}$.


* 
**Experiment 2 Heights:** Small = $0.4^{\circ}$, Tall = $0.44^{\circ}$.



#### 2. Trial Timing & Loop

The experiment should follow a strict "State Machine" logic to maintain millisecond precision:

* 
**State 1: Fixation (200 ms).** Display a central cross.


* 
**State 2: Stimulus (700 ms for Exp 1; 500 ms for Exp 2).** Display two items horizontally aligned.


* 
**Eccentricity:** Exp 1 uses 2.75° from center; Exp 2 uses 0.9° from center.




* 
**State 3: Response (Infinite/Until Keypress).** Clear stimuli and show a blank white screen.


* 
**State 4: ITI (750 ms).** Inter-trial interval before the next fixation.



#### 3. Response Mapping & Data Collection

* **Input Keys:**
* 
`Down Arrow`: "Identical height".


* 
`Left Arrow`: "Left stimulus is taller".


* 
`Right Arrow`: "Right stimulus is taller".




* 
**Data to log:** Participant ID, Trial Number, Stimulus A type, Stimulus B type, Physical Height A, Physical Height B, Response Key, Reaction Time (ms).



#### 4. The Training Requirement

Before the main data collection, the code must include a loop that:

* Uses specific training stimuli (Letters: u, n, x; Words: RADIO, PAPIER, MAISON).


* Provides **immediate feedback** (Correct/Incorrect) after each response.


* Prevents the participant from starting the actual experiment until they reach an **accuracy of 80%**.




