Magnitude estimation is a classic psychophysical method developed by S.S. Stevens. Unlike tasks where participants just say "yes" or "no," this requires them to assign numerical values to their sensations, which allows us to map the relationship between physical stimulus intensity and perceived magnitude.

To have an AI program this, you need to define the **trial structure**, the **scaling method**, and the **data output**. Here is a detailed specification you can provide.

---

## Experiment Specification: Magnitude Estimation of Luminance

### 1. Environment & Setup
* **Background:** Solid mid-gray background (e.g., RGB 128, 128, 128).
* **Stimulus:** A central disk (diameter: 5° of visual angle).
* **Stimulus Set:** 7 levels of luminance. If using 8-bit RGB, use values like: $10, 25, 50, 100, 150, 200, 255$.
* **The Modality:** Free magnitude estimation (no fixed "standard" or "anchor").

### 2. The Procedure
The experiment should consist of 5 blocks. In each block, all 7 stimuli are presented once in a randomized order (35 trials total).

**Trial Flow:**
1.  **Fixation:** A small black cross at center for 500ms.
2.  **Stimulus Presentation:** A gray disk appears at the center for 1000ms.
3.  **Response Window:** The disk disappears. A text input box appears. 
    * *Instruction:* "Assign a number that represents the brightness of the disk you just saw. If it felt twice as bright as a previous one, give it double the number. You may use any positive number (integers or decimals)."
4.  **Inter-Trial Interval (ITI):** 1000ms of blank screen before the next fixation.



### 3. Logic for the AI Coder
* **Randomization:** Use a "shuffled deck" approach for each block to ensure no luminance level is repeated until all others have been shown.
* **Input Validation:** Ensure the participant can only input positive numerical values.
* **Calibration Note:** Remind the AI to use a library that handles "Linear Gamma" if possible (like PsychoPy), otherwise the RGB values won't map linearly to actual physical light output.

### 4. Data Logging
The output file (CSV) must record the following for every trial:
* `Participant_ID`
* `Trial_Number`
* `Stimulus_Luminance` (The RGB value or $cd/m^2$)
* `Participant_Response` (The assigned number)
* `Reaction_Time` (From stimulus offset to "Enter" key press)


