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

The program automatically saves results in the `goxpy_data` directory.
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
go run ./LoT-geometry/ -w -s [subject_id]
```

- **-w**: Windowed mode (1024×768 window instead of fullscreen).
- **-d N**: Display ID — open on monitor N (-1 = primary).
- **-s**: Subject ID (default is 1).
- **ESC**: Quit the experiment.

### 4.1 Running in a Web Browser (WebAssembly)

This experiment can also be run directly in a modern web browser thanks to WebAssembly (Wasm). A pre-compiled version is available in the `web/` folder.

To run the web version:
1.  Navigate to the `examples/LoT-geometry/web` directory.
2.  Serve the files using a local web server (Wasm cannot be loaded directly from `file://` for security reasons). For example:
    ```bash
    # Using Python
    python3 -m http.server 8080
    
    # Or using a Go-based server (if installed)
    serve .
    ```
3.  Open `http://localhost:8080` in your browser.

**Data Saving**: When the experiment ends in the browser, the `.csv` result file will be automatically triggered as a **download** to your computer's `Downloads` folder.

To re-compile the Wasm version yourself:
```bash
GOOS=js GOARCH=wasm go build -o web/main.wasm main.go
```

## References

- Amalric, M., Wang, L., Pica, P., Figueira, S., Sigman, M., & Dehaene, S. (2017). **The language of geometry: Fast comprehension of geometrical primitives and rules in human adults and preschoolers.** *PLoS Computational Biology*, 13(1), e1005273.
