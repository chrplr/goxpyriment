# Kanizsa Illusory Square (goxpyriment)

This example reproduces the classic **Kanizsa square** illusion: four “pacman” disks on a uniform background with a central square mask that induces the perception of an illusory contour.

## Usage

From the repository root:

```bash
cd examples/kanizsa-square
go run . -w
```

### Flags

- `-w`
  Windowed mode (800×600 window instead of exclusive fullscreen).

- `-d N`
  Display ID: open on monitor N (-1 = primary).

- `-s int`  
  Subject ID (not used for data here; kept for API consistency).

- `-r float`  
  **Radius** of the inducing circles in pixels.  
  Default: `50`.

- `-w float`  
  **Size** (width and height) of the central square in pixels.  
  Default: `200`.

Example:

```bash
go run . -w -r 60 -w 250
```

## Behavior

- Light gray background (set via `control.NewExperiment`).
- Four black circles arranged at the corners of an invisible square of side `w`.
- A central light-gray square mask of size `w × w`.
- A short instruction (`"Kanizsa illusory square – press any key to exit"`) shown below the figure.
- The program waits for a key press and then exits.

