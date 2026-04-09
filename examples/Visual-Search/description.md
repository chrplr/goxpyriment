To program a robust version of the **Visual Search experiment** (based on Treisman & Gelade’s Feature Integration Theory), you need to control the relationship between the target and distractors across two main tasks.

Here is the technical specification for implementing this experiment.

---

## 1. Stimuli Design
You need two visual dimensions: **Color** and **Shape**.
* **Colors:** Use two high-contrast colors (e.g., Bright Red and Bright Blue).
* **Shapes:** Use two distinct characters or shapes (e.g., the letters **T** and **L**, or circles and squares).
* **Size:** Stimuli should be small enough to fit many on the screen (e.g., 40x40 pixels).

---

## 2. Experimental Conditions
The experiment is a **2 (Task) × 2 (Target Presence) × 3 (Set Size)** factorial design.

### A. Feature Search (Pop-out)
The target differs from distractors by a **single feature** (e.g., color).
* **Target:** Red **T**
* **Distractors:** Blue **T**s
* *Note: Because the target is the only red thing, it will "pop out" regardless of how many blue distractors are present.*

### B. Conjunction Search (Serial)
The target shares features with two different types of distractors. It is defined by the **combination** of features.
* **Target:** Red **T**
* **Distractors:** Blue **T**s AND Red **L**s
* *Note: To find the Red T, the brain must "bind" color and shape together, which requires serial attention.*



---

## 3. Trial Parameters
To get clean data (slopes), use the following settings:

* **Set Sizes (Total items on screen):** 4, 12, and 24 items.
* **Target Presence:** 50% of trials contain the target; 50% are distractors only.
* **Layout:** Use an invisible **circular grid** (stimuli arranged in a ring around the center) or a **jittered grid**. Avoid a perfect grid to prevent "alignment" cues.
* **Trials per Condition:** Aim for at least 20 trials per "cell" (e.g., 20 trials for Feature/SetSize4/TargetPresent). 
    * *Total Trials Calculation:* 2 (Tasks) × 2 (Presence) × 3 (Set Sizes) × 20 = **240 trials total**.

---

## 4. Procedure (The "Game Loop")
1.  **Fixation:** Display a central cross (+) for **500ms**.
2.  **Stimulus Display:** Show the search array. It stays on screen until the user responds.
3.  **Response:** * `'J'` key for **Target Present**
    * `'F'` key for **Target Absent**
4.  **Feedback:** Brief text ("Correct!" or "Incorrect") for **500ms**. (Optional: provide a "Too Slow!" message if RT > 2000ms).
5.  **Inter-Trial Interval (ITI):** Blank screen for **1000ms** before the next fixation.

---

## 5. Data Analysis (What to plot)
To prove the experiment worked, you should calculate the **Mean Reaction Time (RT)** for correct trials only.
* **Feature Search Results:** The RT graph should be **flat**. Adding more items doesn't slow the user down (Slope $\approx 0$ ms/item).
* **Conjunction Search Results:** The RT graph should show a **linear increase**. Adding items slows the user down (Slope $\approx 20-30$ ms/item).
* **Target Absent vs. Present:** In Conjunction search, the "Target Absent" slope is usually **twice as steep** as the "Target Present" slope because the user has to check every single item before giving up.

---

