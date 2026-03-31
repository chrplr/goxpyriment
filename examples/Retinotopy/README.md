# Retinotopy Experiment

This is a Go implementation of the HCP Retinotopic Mapping experiment, ported from the Python version developed by Christophe Pallier and Bosco Taddei.

## Prerequisites

1.  **Stimuli Extraction:** The experiment requires the stimuli to be extracted from the HDF5 file. If you haven't done so, run the extraction script:
    ```bash
    python3 Retinotopy/h5topng.py
    ```

## How to Run

By default, the experiment starts in **FULLSCREEN** mode.

### Standard Run (Fullscreen)
Starts in **1280x1024** fullscreen mode with the **768x768** stimulus centered.
```bash
go run examples/Retinotopy/main.go -s 0 -r 1
```

### Windowed Mode
Starts in a **900x900** window with the **768x768** stimulus centered.
```bash
go run examples/Retinotopy/main.go -s 0 -r 1 -w
```

### Command Line Options
- `-s <id>`: Subject ID (default: 0).
- `-r <id>`: Run ID (1-6).
  - `1`: RETBAR1 (Swiping Bars)
  - `2`: RETBAR2 (Swiping Bars)
  - `3`: RETCCW (Counter-Clockwise Wedge)
  - `4`: RETCW (Clockwise Wedge)
  - `5`: RETEXP (Expanding Circles)
  - `6`: RETCON (Contracting Circles)
- `-w`: Windowed mode (900×900 window instead of fullscreen).
- `-d N`: Display ID — open on monitor N (-1 = primary).
- `--scaling <factor>`: Scaling factor for stimuli, grid and fixation dot (default: 1.0).
- `-F`: Force Fullscreen (default behavior).

## Controls
- **ESC:** Interrupt and exit the experiment gracefully (data up to the current frame will be saved).
- **Any Key:** Records a keypress event in the data file (used for the fixation dot color change task).

## Data Collection
Results are saved as `.csv` files in the `data/` directory, containing frame-by-frame timing and event logs.
