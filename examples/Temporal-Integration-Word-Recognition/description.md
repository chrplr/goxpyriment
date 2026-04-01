The aim it to write program using goxpyriment to reproduce experiments 1 and 2 from Forget, Buiatti, and Dehaene (2009), described below;


### General Experimental Environment
* **Hardware:** Use a monitor with a **60 Hz** vertical refresh rate to ensure single-frame (16.7 ms) accuracy.
* **Software:** Use Go and goxperiment
* **Viewing Conditions:** Participants should be seated **57 cm** from the display.
* **Visual Stimuli:** * **Font:** Inconsolata, size 20, white text on a black background.
    * **Size:** Stimuli should subtend a maximum of **$0.9^{\circ}$ height** and **$4^{\circ}$ width** of visual angle.
    * **Spacing:** Letter strings must be divided into an **Even component** (2nd, 4th, 6th, and 8th letters) and an **Odd component** (1st, 3rd, 5th, and 7th letters).

 * for the stimuli, use the words listed in the file `words.tsv`

---

### Experiment 1: Subjective Report
This experiment evaluates how participants consciously perceive the alternating components.

#### Stimulus Conditions
1.  **Whole-word:** The merged string is a valid word (6 or 8 letters), but the components themselves are nonwords (e.g., `_A_A_E` + `G_R_G_` = `GARAGE`).
2.  **Component-words:** Each component is a 3- or 4-letter word, but the merged string is a nonword (e.g., `BAR` + `SKI` = `SBKARI`).
3.  **Nonword:** Neither the components nor the merged string are valid words.

#### Trial Protocol
* **Fixation:** A central cross for **1510 ms**.
* **Display Sequence:** 1.  **Even component** for **16 ms** (1 frame).
    2.  **Blank screen (ISI)** for a variable duration.
    3.  **Odd component** for **16 ms** (1 frame).
    4.  **Blank screen (ISI)** for a variable duration.
* **Repetition:** This even-odd sequence repeats **three times**.
* **Mask:** End with a 16-ms masking string of eight "#" signs.
* **Timing (SOA):** Six Stimulus Onset Asynchronies (SOAs) must be used: **50, 67, 83, 100, 117, and 133 ms**.
* **Task:** The participant reports seeing 0, 1, or 2 words using a numeric pad.
* **Trial Count:** 360 target trials (20 per combination of the 6 SOAs and 3 conditions).

---

### Experiment 2: Objective Lexical Decision
This experiment measures reading speed and accuracy to objectively identify the integration threshold.

#### Stimulus Factors
* **SOA:** Use the same six levels as Experiment 1.
* **Word Length:** 4, 6, or 8 letters.
* **Lexicality:** The merged string is either a valid Word or a Pseudoword (pronounceable nonword).

#### Trial Protocol
* **Crucial Difference:** Unlike Experiment 1, the alternating even-odd sequence continues **indefinitely until the participant responds**.
* **Task:** Bi-manual lexical decision. Participants press one button for "Word" (e.g., right index) and another for "Pseudoword" (e.g., left index).
* **Trial Count:** 360 target trials (10 per factorial combination).
* **Data Collection:** Measure response time (RT) from the **onset of the second component string** (when all letters have finally been displayed once).

### Summary of Key Parameters for Programming
| Parameter | Value/Setting |
| :--- | :--- |
| **Refresh Rate** | 60 Hz  |
| **Flash Duration** | 1 frame (16.7 ms)  |
| **ISI Ranges** | ~33 ms to ~117 ms (to achieve 50–133 ms SOAs)  |
| **Exp 1 Repetition** | 3 cycles + Mask  |
| **Exp 2 Repetition** | Infinite cycles until response  |
| **Exp 2 RT Start** | At the start of the 2nd flashed component  |


