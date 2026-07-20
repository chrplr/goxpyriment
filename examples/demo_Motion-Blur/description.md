# Motion Blur & Phantom Array Stimulus

**Objective:** Use the goxyperiment framework to create a high-precision psychophysical demonstration of the ["TestUFO" effect](https://testufo.com/) (Motion Blur vs. Phantom Array) optimized for a **60Hz refresh rate**.


### 1. Core Engine (VSYNC & Timing)
* **Framework:** Use goxpyriment. If it is missing functions that you would deem reusable for other exepriment, you can add them, or even modify the framework (making sure that the other examples in the example folder can be adapted if necessary) 
* **Precision:** Use `performance.now()` to calculate `deltaTime`. Movement must be calculated as `velocity * deltaTime` to ensure smooth motion regardless of minor frame drops.
* **Sub-pixel Rendering:** Use floating-point coordinates for all horizontal positions to prevent "pixel snapping" or jitter.

### 2. The Stimulus Layout
Divide the canvas into two horizontal lanes (Top and Bottom) on a solid black background:

* **Lane 1: Static Fixation (The "Phantom Array" Demo)**
    * Place a small white **fixation cross** in the center of this lane.
    * A thin vertical white bar (2px wide) moves horizontally across the lane at a default velocity of **800 pixels/second**, wrapping around the edges.
* **Lane 2: Smooth Pursuit (The "UFO Blur" Demo)**
    * A "Target" (a 20px green square) moves at the exact same velocity as a second vertical white bar.
    * The bar should be positioned 50px behind or in front of the green square.

### 3. The "Sync-Strobe" Logic (Every Other Frame)
Implement a global toggle called `strobeMode`.
* **When `strobeMode` is OFF:** Draw the vertical bars on every single frame.
* **When `strobeMode` is ON:** * Use a frame counter ($n$).
    * Draw the white bars ONLY when $n \% 2 == 0$. 
    * On odd frames ($n \% 2 != 0$), **do not draw the bars** (only draw the fixation cross and the green square). This simulates a 50% duty cycle.

### 4. Interactive Controls (GUI)
Provide a simple overlay or sidebar with the following:
* **Toggle Strobe:** A button to flip `strobeMode` on/off.
* **Velocity Slider:** Range from 100px/sec to 1500px/sec.
* **Bar Width Slider:** Range from 1px to 10px.
* **Instructions Text:** * *"Stare at the Cross: Notice the bar splits into a 'Phantom Array' of two ghost bars."*
    * *"Follow the Green Square: Notice the bar becomes wide and blurry (Retinal Blur)."*

### 5. Technical Requirements for the AI
* Ensure the canvas scales to the full window width.
* The background must be a true black (`#000000`) to maximize the persistence effect on LCD/OLED screens.
* Include a "Frame Rate Counter" in the corner to verify the browser is hitting a steady 60FPS.

---

**Would you like me to add a section on how to record "Perceived Blur Width" so you can turn this into a formal experiment?*
6. Experimental Measurement (Matching Task)
Add a "Measurement Mode" where the participant can "freeze" their perception into a data point.

The Comparison Stimulus: * Create a static (non-moving) white rectangle at the bottom of the screen.

Give the participant control over its width (using the Left/Right arrow keys).

The Task (Method of Adjustment):

The participant tracks the moving "UFO" and observes the blurred bar.

They then adjust the width of the static rectangle until it looks exactly as wide as the moving blurred bar.

Press 'Enter' to save the Perceived_Width (in pixels) and the Current_Velocity.

The Goal: This allows you to plot a graph showing that Perceived Blur Width increases linearly with Velocity.

7. Data Logging for the AI
The AI should generate a downloadable CSV file at the end of the session with the following columns:

Trial_ID

Velocity_Px_Per_Sec

Actual_Bar_Width (Fixed at 2px)

Strobe_Status (On/Off)

Perceived_Width_Px (The value the user matched)

Timestamp
