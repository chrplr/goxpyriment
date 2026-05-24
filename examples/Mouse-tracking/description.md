Based on the methodology described in the Spivey et al. (2005) paper, here is a detailed technical specification for a coder to implement a modified version of the experiment using written words instead of audio. [cite: 1, 4]

## Experiment Implementation Specification

### 1. Visual Layout and Environment
The experiment is conducted on a two-dimensional computer screen coordinate system. [cite: 12]
* **Target/Distractor Positions:** Place two color images in the **upper-left** and **upper-right** corners of the screen. [cite: 36]
* **Starting Position:** Define a "Start Box" at the **bottom-center** of the screen. [cite: 49] This area will also serve as the location for the written word stimulus.
* **Response Regions:** Define rectangular "Click Areas" around the target and distractor images. [cite: 93] A trial ends only when the participant clicks within one of these two regions. [cite: 93]

### 2. Experimental Conditions
You will need two types of trials to replicate the "attractor dynamics" found in the study: [cite: 12]
* **Cohort Condition:** The distractor object's name shares initial phonological overlap with the target word (e.g., Target: "Candle", Distractor: "Candy"). [cite: 36, 37]
* **Control Condition:** The distractor object's name is phonologically dissimilar to the target word (e.g., Target: "Candle", Distractor: "Jacket"). [cite: 36, 48]

### 3. Trial Procedure and Timing
To allow for "midflight" motor influence, the timing must encourage movement before the word is fully processed. [cite: 52]
* **Step 1:** The participant clicks the bottom-center "Start Box" to begin. [cite: 49]
* **Step 2:** Immediately display the two images (top-left and top-right). [cite: 50]
* **Step 3:** Introduce a **500 ms delay** (asynchrony) after the images appear before displaying the written word at the bottom center. [cite: 50, 51]
* **Step 4:** Instruct participants to begin moving the mouse "straight upward" as soon as they click the start box, even before the word appears. [cite: 52]

### 4. Data Collection Requirements
The key to this experiment is capturing the **continuous trajectory**, not just the final click. [cite: 11, 24]
* **Sampling Rate:** Sample the x, y screen coordinates of the mouse at a minimum of **36 Hz** (roughly every 27-28 ms). [cite: 53]
* **Data Points:** Each trial should ideally yield between **30 to 60 data points** representing the path of the hand. [cite: 54, 56]
* **Normalization:** For later analysis, you will need to resample the collected time vector into **101 equally time-spaced values** (0-100%) using linear interpolation. [cite: 63, 65]

### 5. Expected Analytical Metrics
To verify the implementation, the coder should be able to calculate:
* **Euclidean Proximity:** The proximity to the target and distractor at each time slice, calculated as $1 - (\text{distance} / \text{max\_distance})$. [cite: 127]
* **Trajectory Curvature:** The area (in pixels) between the actual mouse path and a straight line connecting the start and endpoints. [cite: 142]
* **Total Response Time:** The duration from visual onset of the images to the final correct mouse click. [cite: 61]


