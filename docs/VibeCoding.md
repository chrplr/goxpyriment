# How to vibe-code an experiment

You can let an AI coding agent (Claude, Gemini, etc.) write the experiment for
you. Open the agent inside the `goxpyriment` folder and describe your experiment
(stimuli, design, etc.) in plain language. Asking it to add a new experiment to
the `examples` folder leads it to read the existing examples for context.
Recommendation: save your prompt in a `description.md` file.

## Example: reproducing an experiment from a paper

Create a folder for the new experiment, copy the paper into it, and start the
agent from the repository root:

```bash
cd goxpyriment
mkdir -p examples/Finger-Tracking
cp Dotan_Cognition.pdf examples/Finger-Tracking
claude
```

Switch the agent to plan mode, so that it proposes a plan before writing any
code:

```text
❯ /plan
  ⎿ Enabled plan mode
```

Then describe what you want:

```text
❯ Can you read the method section of the pdf file located in
  examples/Finger-Tracking, then create a plan to reproduce their
  finger-tracking experiment where the trajectory of the finger on a touch
  screen, or of the mouse, is measured while the participants must attain a
  target on a horizontal numerical line at the top of the screen? You can ask
  me questions if anything is unclear from the method section of the paper.
```

The
[Finger-Tracking](https://github.com/chrplr/goxpyriment/tree/main/examples/Finger-Tracking)
example in the [gallery](GalleryOfExamples.md) implements this experiment.
