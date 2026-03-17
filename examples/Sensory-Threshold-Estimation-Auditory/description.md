Finding an auditory threshold using a **staircase (or adaptive) procedure** is a standard clinical and research method. The staircase method "hunts" for the lowest volume a person can hear by adjusting the intensity based on previous answers.

The **1-up/2-down** or **1-up/3-down** rule is the gold standard for finding the 70–80% detection threshold.

---

## Experiment Specification: Adaptive Auditory Thresholding

### 1. Acoustic Setup
* **Stimulus Type:** Pure sine wave tones (500ms duration, with 50ms fade-in/out ramps to prevent "clicking" sounds).
* **Frequencies:** by default, test the following tone frequencies: 50Hz, 250Hz, 500Hz, 1000Hz, 2000Hé, 4000Hz, 8000Hz. Provide a command line option to read the list of frequencies on the command line.
* **Initial Intensity:** Start at a clearly audible level (e.g., -20 dBFS or a mid-range system volume).

### 2. The Staircase Logic (1-up / 2-down)
This is the "engine" the AI needs to code. The volume changes based on the participant's accuracy:
1.  **If the participant misses a tone (Incorrect):** Increase the volume by $X$ dB (**1-up**).
2.  **If the participant hears the tone twice in a row (Correct):** Decrease the volume by $X$ dB (**2-down**).
3.  **Step Sizes:** * *Phase 1:* Use a 4 dB step size until the first 2 "reversals" (changing from getting louder to quieter or vice-versa).
    * *Phase 2:* Reduce the step size to 2 dB for the remainder of the task to increase precision.
4.  **Termination:** Stop the staircase for a specific frequency after **8–10 reversals**.



### 3. Trial Structure
Since this is a threshold task, you must use a **2-Interval Forced Choice (2-IFC)** design to ensure the participant isn't just guessing.

* **Interval 1:** 500ms. (Either silence OR the tone).
* **Gap:** 400ms.
* **Interval 2:** 500ms. (Whichever the first interval wasn't).
* **Response:** "In which interval did you hear the tone? (Press 1 or 2)."
* **Feedback:** (Optional but recommended) A brief color flash (Green/Red) to keep the participant engaged.

### 4. Logic 
* **Interleaving:** Tell the AI to **interleave** the 6 staircases. Instead of finishing 60Hz then moving to 146Hz, the code should randomly pick one of the 6 frequencies for each trial. This prevents the participant from predicting the pitch.
* **Audio Safety:** Ensure the code initializes at a low system volume to protect hearing.
* **Threshold Calculation:** The final threshold for each frequency is the **average intensity of the last 4 reversals**.



### 5. Data Output Requirements
The xpd results CSV file should contain:
* `Frequency_Hz`
* `Trial_Number`
* `Current_Intensity_dB`
* `Response_Correct` (True/False)
* `Reversal_Occurred` (Yes/No)
* `Final_Calculated_Threshold` (Per frequency)


