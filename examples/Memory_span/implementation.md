# Memory Span Implementation Details

The Memory Span experiment is implemented in `examples/Memory_span/main.go` using the `goxpyriment` framework.

## Experiment Design

- **Stimulus Types**: The experiment uses three types of stimuli:
  - **Digits**: 0-9
  - **Letters**: A-Z
  - **Words**: A set of 15 simple 3-letter words.
- **Trial Structure**: 30 trials in total, with 10 trials for each stimulus type, presented in random order.
- **Adaptive Staircase**:
  - The initial sequence length is 3 for all types.
  - If a participant correctly recalls the entire sequence in order, the length for that stimulus type increases by 1 for the next trial of that type.
  - If a mistake is made, the length decreases by 1 (minimum length is 1).

## Procedure

1.  **Instructions**: Explains the task and wait for SPACEBAR.
2.  **Presentation Phase**:
    - Items are presented one by one in the center of the screen.
    - Each item is shown for 1000ms, followed by a 200ms blank interval.
3.  **Response Phase**:
    - A grid of buttons is displayed, showing all possible items in the pool (e.g., all 10 digits or all 26 letters).
    - The participant must click the buttons in the exact same order the items were presented.
    - Visual feedback shows the current sequence of items clicked.
4.  **Feedback Phase**:
    - "CORRECT!" is shown in green if the sequence matches.
    - "WRONG!" is shown in red along with the correct sequence if it doesn't match.
    - A buzzer sound plays on errors.
    - Feedback stays for 1500ms before the next trial.

## Technical Details

- **Buttons**: Implemented as a custom `Button` struct combining a `stimuli.Rectangle` and a `stimuli.TextLine`.
- **Hit Detection**: Uses `exp.PollEvents` with a callback to capture precise mouse coordinates (`sdl.EVENT_MOUSE_BUTTON_DOWN`) and compare them against button bounds.
- **Layout**: Buttons are automatically arranged in a centered grid based on the pool size.

## Data Logging

The following data is recorded in the `.csv` file:
- `trial`: Sequential trial number.
- `type`: Stimulus type (Digit, Letter, or Word).
- `length`: Sequence length for this trial.
- `sequence`: The actual sequence presented (space-separated).
- `response`: The participant's response sequence (space-separated).
- `correct`: Boolean indicating if the response was perfectly correct.
