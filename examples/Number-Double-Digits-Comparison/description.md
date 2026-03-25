To program a replication of Experiments 1 and 2 from Dehaene et al. (1990), you will need to build a timed two-digit number comparison task.

Below are the technical specifications for your program

---

## Experiment 1: Standard "55"
[cite_start]This experiment establishes a baseline for the **distance effect** (where RTs decrease as the distance between the target and standard increases)[cite: 15, 58].

* [cite_start]**Fixed Standard:** 55[cite: 93, 98].
* [cite_start]**Stimuli:** All two-digit numbers from 11 to 99, excluding the standard (55)[cite: 98].
* **Frequency of Targets:**
    * [cite_start]Numbers between **41 and 69**: Present **4 times** each[cite: 99].
    * [cite_start]Numbers **outside** that range: Present **2 times** each[cite: 99].
* [cite_start]**Total Trials:** 242 experimental trials, preceded by a 10-trial training block[cite: 102, 104].
* **Response Mapping:**
    * [cite_start]**Right Key:** Target > 55[cite: 94].
    * [cite_start]**Left Key:** Target < 55[cite: 94].

## Experiment 2: Standard "65"
[cite_start]This experiment investigates whether discontinuities in RTs are caused by linguistic properties or the specific digits of the standard[cite: 250, 255].

* [cite_start]**Fixed Standard:** 65[cite: 255, 267].
* [cite_start]**Stimuli:** Two-digit numbers ranging from **31 to 99**, excluding 65[cite: 271].
* [cite_start]**Frequency of Targets:** Present each number **4 times**[cite: 271].
* [cite_start]**Total Trials:** 282 experimental trials, preceded by a 10-trial training block[cite: 272].
* **Response Mapping (Between-Subjects Factor):**
    [cite_start]You must program two versions or a toggle for the response side[cite: 265, 281]:
    1.  [cite_start]**Group 1 (LR):** Right hand for "Larger," Left hand for "Smaller"[cite: 267].
    2.  [cite_start]**Group 2 (LL):** Left hand for "Larger," Right hand for "Smaller"[cite: 268].

---

## Universal Program Logic (Both Experiments)

### 1. Trial Timing
[cite_start]The timing must be precise to capture differences in milliseconds[cite: 95]:
* [cite_start]**Stimulus Duration:** Display the two-digit number for **2 seconds**[cite: 97].
* [cite_start]**Inter-Stimulus Interval (ISI):** A blank screen for **2 seconds**[cite: 97].
* [cite_start]**Total Trial Rate:** One stimulus every 4 seconds[cite: 97].

### 2. Randomization Constraints
The trial list should be pseudorandomized with the following hard-coded rules:
* [cite_start]**No Repeats:** The same target number cannot appear twice in a row[cite: 100].
* [cite_start]**Response Balance:** The subject should never press the same response key more than three times in a row[cite: 100].

### 3. Data Collection
For every trial, your program must record:
1.  [cite_start]**Reaction Time (RT):** Measured from stimulus onset in milliseconds[cite: 95].
2.  [cite_start]**Accuracy:** Whether the response was correct or an error[cite: 106].
3.  [cite_start]**Numerical Distance:** The absolute difference between the target and the standard ($|Target - Standard|$)[cite: 108].

