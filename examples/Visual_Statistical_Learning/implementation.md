# Visual Statistical Learning (VSL) Implementation Details

This example implements the five experiments described in Turk-Browne et al. (2005), "The Automaticity of Visual Statistical Learning".

## Core Design

### Stimulus Construction
- **Novel Shapes**: 24 unique star-like polygons are procedurally generated for each session.
- **Color Pools**: 12 shapes are assigned to a "Red" pool and 12 to a "Green" pool.
- **Triplets**: Within each color, shapes are grouped into 4 fixed sequences (triplets) of 3 shapes each.
- **Foils**: For the 2IFC test phase, "foil" sequences are constructed by taking the 1st, 2nd, and 3rd shapes from different triplets of the same color, ensuring they never appeared in that order during familiarization.

### Familiarization Phase (Learning)
- **Interleaved Streams**: Two independent streams (Red and Green) are generated. Each color stream consists of 24 presentations of each of its 4 triplets (96 triplet occurrences total).
- **Cover Task**: Participants are instructed to attend to shapes of one color (Red or Green, counterbalanced by subject ID) and press SPACEBAR when a shape in that color repeats. 
- **Repetitions**: To support the cover task, 24 occurrences of the 3rd shape in a triplet are immediately repeated.
- **Interleaving Constraint**: The two streams are randomly interleaved into a single sequence of 624 shapes, with the constraint that the number of remaining shapes in one color cannot exceed the other by more than 6.

## Experiment Variants

Select the variant using the `-exp` flag:

- **1A** (Baseline Attention): Fast presentation (200ms stimulus + 200ms blank ISI = 400ms SOA). Test phase uses 2IFC with black shapes.
- **1B** (Increased Exposure): Slower presentation (800ms stimulus + 200ms blank ISI = 1000ms SOA). Test phase uses 2IFC with black shapes.
- **2A** (Maintaining Color Context): Same as 1B, but test phase shapes retain their original colors (Red or Green).
- **2B** (Swapping Color Context): Same as 1B, but test phase shapes have their colors swapped (Red becomes Green, Green becomes Red).
- **3** (Implicit RT Measure): Same as 1B for learning. Test phase is a rapid detection task where participants press SPACEBAR when they see a specific target shape. Reaction times are analyzed by the position of the shape in its original triplet.

## Usage

```bash
# Run Experiment 1B (Default)
go run main.go

# Run Experiment 1A in windowed mode
go run main.go -exp 1A -w

# Run Experiment 3 for Subject 5
go run main.go -exp 3 -s 5
```

## Data Logging

Results are saved in `goxpy_data/` and include:
- `phase`: "familiarization", "test_2ifc", or "test_rt".
- `trial`: Trial index.
- `shape_idx`: Index of the shape shown (0-23).
- `color`: Shape color during presentation.
- `rt`: Reaction time in milliseconds.
- `hit`: Whether a target was correctly detected or a correct choice was made.
- `congruency`: (Relevant for other tasks, here we log `attended` status).
- `pos_in_triplet`: (For Exp 3) 1st, 2nd, or 3rd position in the original triplet.
