# Language of Geometry (LoT-geometry) - Task Version

This example reproduces the experimental task from the study **"The language of geometry: Fast comprehension of geometrical primitives and rules in human adults and preschoolers"** (Amalric et al., 2017).

## 1. Experimental Design

The experiment is organized into blocks, each featuring a spatial sequence of **16 locations** on a regular octagon.

- **Sequence Types**:
    - **Repeat**: Clockwise (CW) and Counter-clockwise (CCW).
    - **Alternate**: Alternating steps (+2, -1).
    - **2squares**: Two nested squares.
    - **2arcs**: Two arcs mirrored by symmetry.
    - **4segments**: Four parallel segments (testing H, V, A, B symmetries).
    - **4diagonals**: Four diameters.
    - **2rectangles / 2crosses**: Complex hierarchical structures.
    - **Irregular**: Random sequences with no geometric regularity.
- **Randomization**:
    - The first two trials are always "Repeat" (randomized CW/CCW).
    - Subsequent trials are presented in a randomized order.
    - The starting point of each sequence is randomized.

## 2. Task Procedure

1.  **Introduction**: A sequence starts by flashing the first **2 locations**.
2.  **Guessing**: The subject must click on the location where they think the **next** circle will appear.
3.  **Feedback**:
    - **Correct**: The correctly guessed location flashes briefly, and the subject proceeds to guess the next location in the sequence.
    - **Incorrect**: The sequence restarts from the beginning, flashing all locations up to the correct one where the mistake was made. The subject then proceeds to guess the *next* location in the sequence.
4.  **Completion**: Each sequence continues until all 16 locations have been revealed.

## 3. Data Collection

The program automatically saves results in the `xpd_results` directory.
Logged variables include:
- `trial_idx`: The position of the sequence in the experiment.
- `seq_name`: The type of sequence being tested.
- `step`: The ordinal position in the 16-item sequence.
- `target_idx`: The index (0-7) of the correct location.
- `click_idx`: The index (0-7) of the location clicked by the subject.
- `is_correct`: Boolean indicating if the guess was correct.
- `rt`: Reaction time in milliseconds from the start of the guessing phase to the mouse click.

## 4. How to Run

From the `examples` directory:

```bash
go run ./LoT-geometry/ -d -s [subject_id]
```

- **-d**: Developer mode (windowed display).
- **-s**: Subject ID (default is 1).
- **ESC**: Quit the experiment.

## References

- Amalric, M., Wang, L., Pica, P., Figueira, S., Sigman, M., & Dehaene, S. (2017). **The language of geometry: Fast comprehension of geometrical primitives and rules in human adults and preschoolers.** *PLoS Computational Biology*, 13(1), e1005273.
