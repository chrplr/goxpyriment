The aim is to create accurate **Shepard-Metzler (1971)** stimuli, respecting the specific "branching" logic and geometric rules used in the original paper. The objects are not random; they are composed of exactly **10 cubes** forming a rigid structure with **three right-angle bends**.

Here are the instructions to build a function in Go generator for these pairs, as png images. The name of the file in the paier should indicate the condition (left, right, Same/Mirror condition, and rotation parameters) 

### Prompt/Instructions for the Coder

**Objective:** Develop a script to generate pairs of 3D stimuli based on the Shepard & Metzler (1971) Mental Rotation task.

#### 1. Geometry Construction Logic
The "Object" must be a single connected string of **10 cubes** following these rules:
* **The Chain:** The object consists of a linear sequence of cubes. 
* **The Bends:** There must be exactly **three right-angle bends** in the chain. 
* **The Segments:** The sequence is divided into 4 segments (e.g., 2-3-3-2 cubes per segment, though the original varied). Each segment must be orthogonal to the previous one.
* **No Self-Intersection:** The algorithm must check that no two cubes occupy the same 3D coordinate space.
* **Asymmetry:** Ensure the resulting 10-cube shape is asymmetrical so that it cannot be mapped onto its mirror image through rotation alone.

#### 2. The Experimental Pair (Stimulus vs. Comparison)
For every trial, generate a pair of images (Left and Right):
* **Left Image (Reference):** The base object at a "home" orientation.
* **Right Image (Comparison):**
    * **Condition A (Same):** The *exact same* 3D object as the Reference.
    * **Condition B (Mirror/Chiral):** A *mirror-image* version of the Reference object (invert the coordinates on one axis, e.g., $x = -x$).
* **Rotation Parameters:** * Apply a rotation to the **Right Image** relative to the Left.
    * **Rotation Type:** Allow for "Picture Plane" rotation (Z-axis) and "Depth" rotation (X or Y axis).
    * **Angles:** Generate pairs at $0^\circ, 20^\circ, 40^\circ, \dots, 180^\circ$ increments.



#### 3. Visual Rendering Style
To match the "seminal" look, use these rendering settings:
* **Material:** Use a matte white or light gray material for the cubes.
* **Outlines:** Apply a thin black edge/outline (Sobel filter or mesh wireframe) to each cube to clearly define the 3D structure.
* **Lighting:** Use a single directional light source (top-left) to create shadows that provide depth cues.
* **Projection:** Use an **Orthographic camera** (not Perspective) to ensure that the parallel lines of the cubes do not converge, matching the original 1971 drawings.

#### 4. Metadata Output
For every generated pair, the script should save a JSON file containing:
* `trial_id`: unique identifier.
* `is_same`: Boolean (True if rotated version, False if mirror version).
* `rotation_angle`: The angular disparity between the two objects.
* `cube_coordinates`: The $(x, y, z)$ list of the 10 cubes used to build the object.

---

### Implementation Tip for the Coder
If using Python, the **`numpy`** library is best for managing the 3D cube coordinates, and **`matplotlib`** (Voxels) or **`Trimesh`** can be used for the actual rendering. The "random walker" approach is best for generating the objects:
1.  Start at $(0,0,0)$.
2.  Pick a random axis ($x, y,$ or $z$).
3.  Add $N$ cubes in that direction.
4.  Pick a *new* axis (orthogonal to the last).
5.  Repeat until 10 cubes and 3 bends are reached.
6.  Validate for self-intersection and asymmetry.
