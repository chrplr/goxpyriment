# Sperling's Iconic Memory Experiment

This example implements the classic partial report procedure developed by **George Sperling (1960)** to investigate the capacity and duration of iconic memory.

## 1. Background

Sperling found that when participants were shown a 3x3 grid of letters for a very short time (50ms), they could only recall about 4-5 items in a "Whole Report" condition. However, if they were cued to recall only a specific row immediately after the grid disappeared ("Partial Report"), they could recall almost all items from that row. This suggested that the entire grid was briefly available in a high-capacity but rapidly decaying sensory store (iconic memory).

## 2. Experimental Task

- **Fixation**: A central cross appears for 500ms.
- **Stimulus**: A 3x3 grid of random uppercase consonants is flashed for **50ms**.
- **Cue**: 
    - In **Partial Report** trials, a tone sounds immediately after the stimulus offset:
        - **High Tone (1000 Hz)**: Recall the Top row.
        - **Medium Tone (500 Hz)**: Recall the Middle row.
        - **Low Tone (250 Hz)**: Recall the Bottom row.
    - In **Whole Report** trials, no specific row tone is played (or a neutral prompt is given).
- **Response**: The participant types the letters they remember into a text input box.

## 3. Implementation Details

- **Language**: Go (using `goxpyriment` framework).
- **Graphics**: 3x3 grid rendering with `stimuli.TextLine`.
- **Audio**: Procedural sine wave tones generated with `stimuli.Tone`.
- **Input**: User response collection with `stimuli.TextInput`.
- **Trial Balancing**: 10 Whole Report trials and 30 Partial Report trials (10 per row) are shuffled.

## 4. Controls

- **Keyboard**: Use the letter keys to type your response and press **ENTER** to submit.
- **ESC**: Quit the experiment at any time.

## 5. How to Run

From the `Memory-Iconic-Sperling` directory:

```bash
go run main.go -w -s [subject_id]
```

Or from the repository root:

```bash
go run examples/Memory-Iconic-Sperling/main.go -w -s [subject_id]
```

- **-w**: Windowed mode (1024×768 window instead of fullscreen).
- **-d N**: Display ID — open on monitor N (-1 = primary).
- **-s**: Subject ID (for data logging).

## References

- Sperling, G. (1960). **The information available in brief visual presentations.** *Psychological Monographs: General and Applied*, 74(11), 1-29.
