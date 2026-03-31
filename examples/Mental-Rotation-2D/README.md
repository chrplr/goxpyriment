# Mental Rotation Task

This example implements a mental rotation experiment inspired by the classic study by **Shepard & Metzler (1971)**.

## 1. Experimental Task

The participant is shown two 2D asymmetrical shapes side-by-side. The task is to determine if the right-hand shape is:
1.  A **rotated version** of the left-hand shape ("Same").
2.  A **mirrored and rotated version** of the left-hand shape ("Different").

### Stimuli
- **Shapes**: Asymmetrical 2D polygons (L-shaped).
- **Angles**: The right-hand shape is rotated by 0, 40, 80, 120, or 160 degrees relative to the left-hand shape.
- **Conditions**: "Same" (rotated) vs "Mirrored" (reflected then rotated).

## 2. Procedure

1.  **Instructions**: An initial screen explains the task.
2.  **Fixation**: A fixation cross is displayed throughout the experiment (during the 500ms fixation period, stimulus presentation, and inter-trial interval).
3.  **Stimulus**: Two shapes appear alongside the central fixation cross. The participant responds as quickly as possible.
4.  **Feedback**: Negative auditory feedback is provided for incorrect responses (Buzzer). Correct responses do not trigger a sound.
5.  **Data Logging**: Accuracy and Reaction Time (RT) are recorded. Typically, RT increases linearly with the rotation angle for "Same" pairs.

## 3. Controls
- **'S'**: Press to indicate the shapes are the **SAME**.
- **'D'**: Press to indicate the shapes are **DIFFERENT** (mirrored).
- **'ESC'**: Quit the experiment.

## 4. How to Run

From the `Mental-Rotation-2D` directory:

```bash
go run main.go -w -s [subject_id]
```

Or from the repository root:

```bash
go run examples/Mental-Rotation-2D/main.go -w -s [subject_id]
```

- **-w**: Windowed mode (1024×768 window instead of fullscreen).
- **-d N**: Display ID — open on monitor N (-1 = primary).
- **-s**: Subject ID (default is 1).

## 5. Notes on 3D Implementation

The original Shepard & Metzler study used perspective drawings of 3D cube assemblies. This implementation uses 2D polygons as a programmatic proxy. To reproduce the exact 3D stimuli, you can replace the `stimuli.Shape` calls with `stimuli.Picture` calls using pre-rendered images of the 3D objects at various rotation angles.

## References

- Shepard, R. N., & Metzler, J. (1971). **Mental rotation of three-dimensional objects.** *Science*, 171(3972), 701-703.
