Programming the experiments from Saffran.learning.tones.cognition.1999.pdf

To program these experiments, an AI coding agent needs to simulate the **Exposure (Familiarization) Phase** and the **Testing Phase** for each study. The core logic relies on generating continuous auditory streams where the only cues to boundaries are **transitional probabilities (TPs)**[cite: 233, 236].

---

## **Experiment 1: Adult Tone Segmentation (Words vs. Non-words)**
This experiment tests whether adults can segment continuous tones using the same statistical mechanisms as speech[cite: 320, 321].

### **1. Stimuli Generation**
* **Elements**: Use 11 pure sine wave tones from a single octave (starting at middle C)[cite: 344].
* **Duration**: Each tone must be exactly **0.33 s** with no silent gaps between them[cite: 344, 349].
* **Structure**: Group tones into six 3-tone "words" (e.g., ADB, DFE)[cite: 344].
* **TP Calculation**: 
    * **Within-word TPs**: Average **0.64**[cite: 358].
    * **Between-word TPs**: Average **0.14**[cite: 359].
* Total exposure time is **21 minutes** (three 7-minute sessions with breaks)[cite: 351, 392, 394].

### **2. Testing Phase (Two-Alternative Forced-Choice)**
* **Trials**: 36 trials[cite: 364].
* **Trial Structure**: Present a **Word** (from the exposure language) vs. a **Non-word** (3 tones that never occurred in that order during exposure, TP = 0.0)[cite: 365, 366].
* **Intervals**: 0.75 s pause between the two sequences; 5 s interval between trials[cite: 372].
* **Goal**: The agent should record which sequence (1 or 2) the subject identifies as more "familiar"[cite: 395].

---

## **Experiment 2: Adult Tone Segmentation (Words vs. Part-words)**
This experiment increases difficulty by using "part-words"—sequences that actually occurred during exposure but spanned word boundaries[cite: 483, 498].

### **1. Stimuli Generation**
* **Language One**: Use the same words as Experiment 1[cite: 481].
* **Language Two (Part-words)**: Create 3-tone sequences using two tones from a Word plus one "illegal" tone (e.g., if ADB is a word, G#DB is a part-word)[cite: 485, 489].
* **TP Constants**: 
    * **Within-word**: ~0.56[cite: 494].
    * **Across-boundary**: ~0.15[cite: 494].

### **2. Testing Phase**
* **Comparison**: Force-choice between a **Word** and a **Part-word**[cite: 495, 498].
* **Counterbalancing**: Ensure Language A's words are Language B's part-words to control for melodic bias[cite: 488, 496].
* **Logic**: Record if subjects can distinguish high-probability sequences (Words) from lower-probability sequences that were still heard during exposure (Part-words)[cite: 541, 544].

---

## **Experiment 3: Infant Tone Segmentation (Preferential Listening)**
This experiment adapts the "part-word" test for 8-month-old infants using a looking-time paradigm[cite: 614, 616, 621].

### **1. Stimuli Generation**
* **Duration**: Tones are **0.33 s**[cite: 639]. 
* **Language Structure**: Four 3-tone words (e.g., AFB, F#A#D)[cite: 640].
* **TP Constants**: Within-word TPs = **1.0**; Part-word TPs = **0.33** for the first pair and **1.0** for the second[cite: 655].
* **Exposure**: A continuous **3-minute** stream (180 word tokens total)[cite: 641, 643].

### **2. Testing Phase (Preferential Listening Procedure)**
* **Logic**: The computer must control a central blinking light and two side lights[cite: 677, 678].
* **Trial Trigger**: 
    1.  Blink central light until infant fixates[cite: 678].
    2.  Extinguish center, blink a side light[cite: 678].
    3.  When infant turns head (30°), play a repeated test string (Word or Part-word) with 500ms gaps[cite: 679, 680].
* **Termination**: Stop the sound/light when the infant looks away for **2 consecutive seconds** or reaches **15 seconds** of total looking[cite: 680].
* Success is indicated by longer looking times for "novel" Part-words[cite: 624, 686].

---

### **Summary Table for AI Coding Agent**

| Feature | Adults (Exp 1 & 2) | Infants (Exp 3) |
| :--- | :--- | :--- |
| **Input Elements** | Tones [cite: 639] |
| **Exposure Time** | 3 Minutes [cite: 641] |
| **Test Format** | Looking-time (head-turn) [cite: 663] |
| **Primary Variable** | Duration of fixation (seconds) [cite: 683] |
| **Test Stimuli** | Words vs. Part-words [cite: 650] |


