# Simon Task Implementation Details

The Simon task is implemented in `examples/Simon_task/main.go` using the `goxpyriment` framework.

## Experiment Flow

1.  **Instructions**: The experiment starts with a screen explaining the task. The user presses the spacebar to begin.
2.  **Trial Structure**:
    - **Fixation**: A white fixation cross remains in the center of the screen throughout the trial.
    - **Random Interval**: A random duration between 500ms and 1500ms before the stimulus appears.
    - **Stimulus**: A red or green square (100x100 pixels) appears 300 pixels to either the left or right of the center. The fixation cross remains visible.
    - **Response**: The user must indicate the **color** of the square as quickly as possible:
        - **Red**: Press 'F' (left index finger).
        - **Green**: Press 'J' (right index finger).
    - **Feedback & Repetition**: If an incorrect key is pressed, a buzzer sounds, "WRONG!" is displayed, and the trial is re-queued at a random position later in the experiment.
3.  **Completion**: The experiment concludes after 100 successful trials.

## Data Logging

The following variables are logged for each trial in the `.csv` results file:
- `trial`: The sequence number of the trial.
- `color`: The color of the stimulus ("red" or "green").
- `position`: The position of the stimulus ("left" or "right").
- `key`: The keycode of the response.
- `rt`: Reaction time in milliseconds.
- `correct`: Boolean indicating if the response was correct.
- `congruency`: Whether the trial was "congruent" (e.g., red square on the left) or "incongruent" (e.g., red square on the right).

## Key Constants

- `NTrials`: 100
- `SquareSize`: 100px
- `SquareOffset`: 300px from center
- `RedKey`: `control.K_F`
- `GreenKey`: `control.K_J`
