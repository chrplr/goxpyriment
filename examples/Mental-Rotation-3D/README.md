# Mental Rotation Task (3D)

This experiment is a 3D implementation of the classic **Shepard & Metzler (1971)** mental rotation task.

## 1. Experimental Task

The participant is shown two 3D cube assemblies side-by-side. The task is to determine if the right-hand shape is:
1.  A **rotated version** of the left-hand shape ("Same").
2.  A **mirrored and rotated version** of the left-hand shape ("Different").

### Stimuli
- **Shapes**: Procedurally generated 3D assemblies of $N$ cubes with 3 right-angle bends.
- **Angles**: The right-hand shape is rotated by 0 to 180 degrees (in 20-degree steps) relative to the left-hand shape.
- **Conditions**: "Same" (rotated) vs "Mirrored" (reflected then rotated).

## 2. Procedure

1.  **Generation**: Upon startup, the program procedurally generates a new 3D shape and renders all necessary rotation pairs as PNG images in a `stimuli/` directory (if they don't already exist).
2.  **Instructions**: An initial screen explains the task.
3.  **Fixation**: A fixation cross is displayed throughout the experiment.
4.  **Stimulus**: Two 3D shapes appear. The participant responds as quickly as possible.
5.  **Feedback**: Negative auditory feedback (buzzer) for incorrect responses.
6.  **Data Logging**: Accuracy, Reaction Time (RT), and the number of cubes used are recorded.

## 3. Controls
- **'S'**: Press to indicate the shapes are the **SAME**.
- **'D'**: Press to indicate the shapes are **DIFFERENT** (mirrored).
- **'ESC'**: Quit the experiment.

## 4. How to Run

From the `Mental-Rotation-3D` directory:

```bash
go run main.go -nc 10 -w -s [subject_id]
```

### Command-line Arguments
- **-nc N**: Number of cubes used to generate the 3D shapes (default: 5).
- **-scaling S**: Scaling factor for the stimulus size (default: 1.0).
- **-w**: Windowed mode (1024×768 window instead of fullscreen).
- **-d N**: Display ID — open on monitor N (-1 = primary).
- **-s**: Subject ID (default is 0).

## 5. Implementation Details

The program features a custom 3D procedural generator and an orthographic rasterizer.
- **Asymmetry (Chirality):** The generator ensures that every assembly is 3D-chiral; it cannot be mapped onto its mirror image through rotation alone.
- **Visibility Verification:** A pixel-level visibility check verifies that every single cube in the assembly has at least one visible facet for all experimental rotation angles and conditions. If a shape has hidden cubes, it is automatically discarded, and a new one is generated.
- **Rendering:** The renderer uses fixed gray shading for X, Y, and Z faces and black outlines to maximize structural clarity, matching modern online behavioral platforms.
- **Dynamic Background:** The experiment runs on a light gray background to optimize contrast and visibility of the shaded stimuli.

## References

- Shepard, R. N., & Metzler, J. (1971). **Mental rotation of three-dimensional objects.** *Science*, 171(3972), 701-703.
