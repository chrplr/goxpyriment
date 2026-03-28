# Attentional Blink Experiment

This example implements the **Attentional Blink** paradigm.

## 1. Background

The "attentional blink" is a phenomenon where the second of two targets cannot be detected or identified when it appears shortly after the first target (typically within 200-500ms). This suggests a temporary bottleneck in attention processing.

## 2. Experimental Task

- **RSVP Stream**: 19 letters are presented rapidly in the center of the screen.
- **Timing**: Each letter is shown for **100ms**.
- **Targets**: The letters **'J'** and **'K'**.
- **Conditions**:
    - **Lag**: The number of items between 'J' and 'K' (varies from 1 to 8).
    - **Presence**: Trials may contain both 'J' and 'K', only one, or neither.
- **Response**: After the stream, the participant reports what they saw.

## 3. Controls

- **'J'**: Saw only 'J'.
- **'K'**: Saw only 'K'.
- **'B'**: Saw BOTH ('J' and 'K').
- **'N'**: Saw NEITHER.
- **'ESC'**: Quit the experiment.

## 4. How to Run

From the `Attentional-Blink` directory:

```bash
go run main.go -w -s [subject_id]
```

Or from the repository root:

```bash
go run examples/Attentional-Blink/main.go -w -s [subject_id]
```

- **-w**: Windowed mode (1024×768 window instead of fullscreen).
- **-d N**: Display ID — open on monitor N (-1 = primary).
- **-s**: Subject ID for data logging.

## References

- Raymond, J. E., Shapiro, K. L., & Arnell, K. M. (1992). **Temporary suppression of visual processing in an RSVP task: An attentional blink?** *Journal of Experimental Psychology: Human Perception and Performance*, 18(3), 849.
