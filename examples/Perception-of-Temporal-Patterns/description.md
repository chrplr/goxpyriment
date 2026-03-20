Based on the paper by Povel and Essens (1985), here is the detailed experimental design for their three experiments. The study investigates how an internal clock and coding efficiency influence the perception and reproduction of temporal patterns.

---

## General Stimulus Characteristics
Across all experiments, temporal patterns were "pure," meaning they consisted of identical tones where only the onset-to-onset intervals varied.
* **Tone Specifications:** 50-ms square waves.
* **Frequency:** Primarily 830 Hz (except where noted).
* **Rise/Fall Times:** 5 ms.
* **Smallest Interval (Unit):** 200 ms (referred to as "1"). Larger intervals were integer multiples: 400 ms (2), 600 ms (3), and 800 ms (4).


** Important your programs should take a argument on the command line specifying a sound file that will be used in lieu of tones.**
---

## Experiment 1: Clock Induction and Reproduction
**Goal:** To test if patterns that more strongly induce an internal clock are reproduced more accurately.

### 1. Stimuli
* **Composition:** 35 sequences, all permutations of the interval set: **1 1 1 1 1 2 2 3 4**.
* **Categories:** Stimuli were divided into 7 categories (5 sequences each) based on "induction strength" calculated by the authors' computer model.
    * **Category 1:** Clock ticks coincide only with perceived accented elements (strongest induction).
    * **Higher Categories:** Clock ticks increasingly coincide with unaccented elements or silence (weaker induction).

### 2. Procedure
* **Participants:** 24 university students.
* **Learning Phase:** Subjects listened to a sequence repeating indefinitely. They were encouraged to tap along and could listen as long as they wished.
* **Reproduction Phase:** Once ready, the subject pressed a button to stop the stimulus and then tapped four periods of the pattern on a response key.
* **Feedback:** Each tap produced the same 830-Hz tone as the stimulus.
* **Metrics:** Researchers measured **acquisition time** (number of presentations) and **reproduction accuracy** (sum of absolute differences between stimulus and response intervals).

---

## Experiment 2: Manipulating Clock Induction
**Goal:** To see if providing an external "clock" (isochronous tones) improves the reproduction of difficult patterns.

### 1. Stimuli
* **Patterns:** 20 sequences from Experiment 1 (Categories 1–4). 
* **The "Clock" Tone:** A low-pitched isochronous sequence (125 Hz) played simultaneously with the high-pitched pattern.
    * **Intensity:** 10 dB lower than the pattern.
    * **Timing:** Fixed interval of 800 ms (unit "4"), synchronized so that every low-pitched tone coincided with a tone in the high-pitched pattern.

### 2. Procedure
* Identical to Experiment 1.
* **Task:** Subjects were specifically instructed to reproduce the **high-pitched** patterned sequence, ignoring the low-pitched "clock".
* **Participants:** The same 24 subjects from Experiment 1.

---

## Experiment 3: Complexity and Coding
**Goal:** To test if temporal patterns are coded relative to an induced clock, making the same pattern seem different in different clock contexts.

### 1. Stimuli
* **High-Pitched Pattern:** 1044-Hz tones. The total sequence length was always 12 units (2400 ms).
* **Clock Variations:** Each pattern was paired with two different low-pitched isochronic sequences (261 Hz) in separate trials.
    * **3-Clock:** Low tones every 3 units (600 ms).
    * **4-Clock:** Low tones every 4 units (800 ms).
* **Constraint:** All clock ticks had to coincide with a tone in the high-pitched sequence.


### 2. Procedure
* **Participants:** 25 university students.
* **Task:** Pairwise comparison. Subjects listened to two "double sequences" (the same high-pitched rhythm but different low-pitched clocks).
* **Controls:** Subjects could toggle between the two versions as often as they liked using "call" buttons.
* **Judgment:** Subjects had to indicate which version was "simpler".

---

## Summary for Reproduction
To reproduce this, you will need:
1.  **Software:** A system to generate square waves and precisely time audio onset intervals.
2.  **Model Logic:** To replicate the "Categories," you must apply the authors' **Accent Rules**:
    * Isolated tones are accented.
    * The second tone of a cluster of two is accented.
    * The first and last tones of clusters of three or more are accented.



The following details provide the specific interval sequences for the 35 stimuli used in **Experiment 1**. These patterns are all permutations of the same set of nine intervals: **five** of 200 ms, **two** of 400 ms, **one** of 600 ms, and **one** of 800 ms.

### Experiment 1 Stimuli (Interval Sequences)
The numbers 1, 2, 3, and 4 represent onset intervals of **200, 400, 600, and 800 ms**, respectively. 

| Category | Stimulus No. | Interval Sequence (Permutation)  |
| :--- | :--- | :--- |
| **1** | 1–5 | (1) 1 1 2 2 2 2 3 4 1; (2) 1 1 2 1 2 1 3 1 4; (3) 1 1 2 2 1 1 3 1 4; (4) 2 1 1 2 1 3 1 4 1; (5) 2 2 1 1 1 3 1 4 1 |
| **2** | 6–10 | (6) 1 3 2 1 2 1 1 4 1; (7) 1 1 2 1 1 2 1 4 3; (8) 1 1 2 1 2 3 1 4 1; (9) 3 1 1 1 1 2 2 4 1; (10) 3 2 1 1 2 1 1 4 1 |
| **3** | 11–15 | (11) 1 1 2 1 1 2 1 4 3; (12) 1 1 1 2 1 3 2 4 1; (13) 2 1 1 1 1 2 3 1 4 1; (14) 2 1 1 2 1 1 3 4 1; (15) 1 3 1 2 2 1 1 4 1 |
| **4** | 16–20 | (16) 1 1 3 1 1 2 2 4 1; (17) 2 1 1 1 2 1 1 4 3; (18) 1 2 1 1 1 2 1 4 3; (19) 1 2 1 1 1 3 2 4 1; (20) 1 3 1 2 1 1 1 4 2 |
| **5** | 21–25 | (21) 1 3 1 1 1 2 1 4 2; (22) 1 1 1 1 2 1 2 3 4; (23) 1 1 1 2 3 1 1 2 4; (24) 1 1 3 1 2 1 1 2 4; (25) 1 2 1 3 2 1 1 1 4 |
| **6** | 26–30 | (26) 3 1 2 1 1 2 1 1 4; (27) 1 1 1 2 2 3 1 1 4; (28) 2 1 1 1 2 3 1 1 4; (29) 2 3 1 1 1 2 1 1 4; (30) 1 1 2 1 2 3 1 1 4 |
| **7** | 31–35 | (31) 1 1 3 1 2 1 1 2 4; (32) 1 1 1 2 1 1 3 2 4; (33) 1 1 1 3 1 2 1 2 4; (34) 2 1 1 1 1 3 1 2 4; (35) 1 2 3 1 1 1 1 2 4 |

---

### Implementation Details for the "Internal Clock"
The study assumes that the internal clock is determined by **accented events**. To reproduce the model's logic for categorizing these sequences, you must apply the following perceptual marking rules:



1.  **Isolated Tones:** A tone that is relatively isolated is perceived as accented.
2.  **Clusters of Two:** The **second** tone of a cluster of two tones is accented.
3.  **Clusters of Three or More:** The **initial and final** tones of a cluster are accented.

### Key Performance Metrics
* **Acquisition:** Measured as the total number of times a subject listened to a sequence before attempting reproduction.
* **Reproduction Error:** Calculated by summing the absolute differences (in ms) between the stimulus intervals and the corresponding reproduced intervals.
* **Learning Trend:** You should observe that Category 1 sequences require the fewest presentations (approx. 7) and have the lowest error (approx. 145 ms), while higher categories (6 and 7) require more presentations (approx. 13-14) and yield higher errors (approx. 220 ms).


