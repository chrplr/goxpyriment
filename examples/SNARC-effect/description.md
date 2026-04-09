To replicate Experiment 1 from Dehaene et al. (1993), you will need to program a parity judgment task involving one-digit Arabic numerals. Below are the technical specifications based on the study's methodology.

## Subjects and Groups
While the original study used 20 right-handed French students divided into two groups of 10 (Literary vs. Scientific), a general replication would focus on the following[cite: 175, 176, 177]:
* **Handedness:** Subjects should be right-handed[cite: 175].
* **Instruction:** Emphasize both **speed and accuracy** in their responses[cite: 178].

---

## Stimuli and Apparatus
* **Target Stimuli:** Arabic digits in the range **0–9**[cite: 164, 178].
* **Visual Dimensions:**
    * **Fixation Frame:** An empty rectangular frame ($22\text{ mm} \times 32\text{ mm}$) centered on the screen[cite: 187].
    * **Digits:** The target numbers should be approximately $10\text{ mm} \times 17\text{ mm}$[cite: 188].
* **Response Hardware:** The original used two Morse keys **26 cm apart**[cite: 186]. For a computer replication, use two keys on opposite sides of the keyboard (e.g., 'A' and 'L') to ensure bimanual response[cite: 186].
* **Timing Precision:** The system must record response times (RT) with **1-ms precision**[cite: 188].

---

## Experimental Design
The experiment consists of **two blocks** with reversed response mappings[cite: 179, 180].

### Block Structure
1.  **Block A:** Odd response = Right key; Even response = Left key[cite: 179].
2.  **Block B:** Odd response = Left key; Even response = Right key[cite: 180].

* **Counterbalancing:** Half of the subjects should perform Block A then Block B; the other half should perform Block B then Block A[cite: 181].
* **Trials per Block:** 102 total trials (12 training trials followed by 90 experimental trials)[cite: 184].

---

## Trial Procedure
Each trial should follow this sequence:
1.  **Fixation:** Display the empty rectangular frame for **300 ms**[cite: 187].
2.  **Target Display:** The target digit appears inside the frame[cite: 188].
3.  **Response Window:** Record the response for a maximum of **1,300 ms**[cite: 188].
4.  **Inter-Trial Interval (ITI):** After the response (or the 1,300 ms timeout), erase the frame and digit. The screen remains blank for **1,500 ms** before the next trial starts[cite: 188, 190].

---

## Randomization and List Generation
To replicate the exact statistical distribution used in the study, follow these constraints for each block:
* **Frequency:** Each digit (0–9) must be presented exactly **9 times** in the experimental phase[cite: 183].
* **Transition Constraint:** Generate a random list such that **every number follows every other number once and only once** (e.g., the sequence "1, 2" occurs once, "1, 3" occurs once, etc.)[cite: 182].
* **No Repetition:** Consecutive presentations of the same number (e.g., "2, 2") are **not allowed**[cite: 183].

> [!TIP]
> To program the transition constraint, you can create a $10 \times 10$ matrix of all possible digit pairings (excluding same-digit pairs) and shuffle these pairs to create a continuous chain.

