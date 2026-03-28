# Unconscious Working Memory Task

This example implements the experimental paradigm described in Trübutschek, D., Marti, S., Ojeda, A., King, J.-R., Mi, Y., Tsodyks, M., & Dehaene, S. (2017). A theory of working memory without consciousness or sustained activity. Elife, 6, e23871.

## 1. Experimental Task

The task is a spatial delayed-response task designed to assess the retention of a target location under varying levels of subjective visibility.

### Trial Structure:
1.  **Fixation**: A central fixation cross (500 ms).
2.  **Target**: A faint gray square flashed for 17 ms in 1 of 20 positions along a circle. On 20% of trials, no target is shown (blank).
3.  **Post-target Fixation**: 17 ms.
4.  **Mask**: Four squares surrounding each of the 20 possible locations (233 ms).
5.  **Delay**: A variable delay period (2.5, 3.0, 3.5, or 4.0 s). On 50% of trials, an unmasked distractor square appears 1.5 s into the delay.
6.  **Response Screen**: 20 letters appear at the 20 positions (2.5 s). In the original study, participants spoke the letter at the target location.
7.  **Visibility Rating**: The word "Vu?" (French for "Seen") appears. Participants rate visibility on the 4-point PAS scale (1-4).

## 2. Controls
- **'1'-'4'**: Press to rate visibility on the Perceptual Awareness Scale (PAS).
- **'ESC'**: Quit the experiment.

## 3. How to Run

From the `Trubutschek_Unconscious_Working_Memory` directory:

```bash
go run main.go -w -s [subject_id]
```

Or from the repository root:

```bash
go run examples/Trubutschek_Unconscious_Working_Memory/main.go -w -s [subject_id]
```

- **-w**: Windowed mode (1024×768 window instead of fullscreen).
- **-d N**: Display ID — open on monitor N (-1 = primary).
- **-s**: Subject ID for data logging.
