# Lilac Chaser (goxpyriment)

This example implements the [Lilac Chaser  illusion](https://en.wikipedia.org/wiki/Lilac_chaser): a ring of lilac disks with one disappearing wedge, creating the perception of a rotating gap and a green afterimage.

## Usage

From the repository root:

```bash
cd examples/lilac_chaser
go run . -w
```

### Flags

- `-w`
  Windowed mode (800×800 window instead of exclusive fullscreen).

- `-d N`
  Display ID: open on monitor N (-1 = primary).

- `-r float`  
  **Radius** of the lilac disks in pixels.  
  Default: `40`.

- `-R int`, `-G int`, `-B int`  
  **RGB components** of the disk color (0–255 each).  
  Defaults: `R=250`, `G=217`, `B=248` (a typical lilac color).

Example:

```bash
go run . -w -r 50 -R 255 -G 180 -B 220
```

## Behavior

- White background (`control.White`).
- 12 disks arranged on a circle of fixed radius.
- On each frame:
  - All disks except one are drawn, creating a moving “gap”.
  - A central fixation cross is shown; you should keep your gaze on it to see the illusion.
- Frames advance every ~100 ms using `clock.Wait(100)`.
- Press `ESC` or close the window to quit.

