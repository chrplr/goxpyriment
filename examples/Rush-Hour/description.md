# Rush Hour — specification

## 1. Task

Rush Hour is a sliding-block puzzle played on a 6×6 grid. Vehicles occupy 2 or 3
contiguous cells, either horizontally or vertically, and slide only along their
own axis. They cannot overlap, pass through each other, or leave the grid — with
one exception: the red target car, which is always horizontal on row 2, leaves
the board through an opening in the right wall. Freeing it solves the puzzle.

The participant solves a series of such puzzles, one per trial, presented in a
fixed order of increasing difficulty (minimum solution length). The embedded
library holds 49 puzzles spanning 3 to 51 moves; `-n N` limits a session to the
first *N* of them (default 12).

## 2. Display

- Logical resolution 1024 × 768, light grey background (240, 240, 240).
- The 6×6 board is drawn with 90 px cells (540 × 540 px), centered horizontally
  and shifted up by 40 px to leave room for the status line.
- Grid lines in mid-grey; a red bar on the right wall of row 2 marks the exit.
- Vehicles are filled rectangles with a black outline; the target car is red,
  the others take colors cycling through the palette of the original pygame
  implementation. The vehicle under the cursor is outlined in white.
- A status line below the board shows the trial number: `Puzzle n/N — free the
  RED car`.

## 3. Interaction

The vehicle under the cursor is determined by the grid cell the cursor is in.

Interaction is a single mode: **one click moves one vehicle by one cell.** The
side of the vehicle's midline on which the click lands selects the direction
along the vehicle's own axis: left of the midline sends a horizontal vehicle one
cell left, right of it one cell right, above/below likewise for a vertical one.
A step into a wall or into another vehicle leaves the board unchanged. There is
no selection state and no dragging, so no click depends on any earlier one, and
the white outline is a hover cue only.

The split is geometric rather than by cell index. For 2-cell vehicles the two
are identical, since the midline falls on the boundary between the cells; for
3-cell vehicles a cell-index rule would leave the middle cell — a third of the
vehicle's surface — with no direction to give, and therefore inert. Every point
of every vehicle now yields a direction, and each direction offers a target 1.5
cells wide.

This departs from the pygame original, which drags. The gain is that every move
is a discrete event with an unambiguous onset: no press–release pairing, and no
mid-drag intermediate positions to disentangle in the log.

Move legality (the port of `try_move_car`) remains stepwise: a request to move
past an obstacle is truncated rather than rejected. With one-cell steps the
distinction is invisible in practice, but it keeps `Board.TryMove` usable for
longer displacements (the breadth-first solver in `board_test.go` relies on it).

## 4. Trial structure

1. 800 ms blank inter-trial interval.
2. The board appears; the clock starts; a `trial_start` row is written.
3. The participant manipulates the board freely. Every action is logged as it
   happens. There is no time limit and no way to skip a puzzle.
4. When the red car reaches the right wall: a `trial_end` row with the summary,
   then the solved board with the caption `PUZZLE SOLVED!` and a confirmation
   sound for 1.2 s.
5. The data file is flushed, and the next puzzle begins.

`ESC` or closing the window terminates the session at any point; everything
recorded so far is preserved.

## 5. Measures

The primary record is the ordered list of actions. From it one can derive:

- **Solution time** and **move count** per puzzle (also given directly on the
  `trial_end` row).
- **Excess moves** — moves made minus the minimum for that puzzle (the `n_moves`
  and `min_moves` columns). Both are on the slide metric: consecutive clicks on
  the same vehicle in the same direction count as one move.
- **Latency before each move**, from the `t_ms` differences — planning pauses,
  in particular the long pause typically preceding the first move.
- **Ineffective clicks** (`click_blocked`, `click_empty`) — vehicles pushed
  against an obstacle or considered and abandoned, a trace of the search.
- **Backtracking** — replaying the moves reconstructs the board state at each
  point, so returns to previously visited states can be counted.

## 6. Puzzle set

49 puzzles, embedded in the binary, one at each minimum solution length from 3
to 51 moves (the metric counts one slide of a vehicle, over any distance, as one
move). Each board carries 7 to 15 vehicles.

They were drawn from Michael Fogleman's exhaustive Rush Hour database
(https://www.michaelfogleman.com/rush/), which enumerates all 2,577,412
"interesting" 6x6 configurations with up to two walls. The selection kept
wall-free positions with at least 8 vehicles, sampled candidates across the
whole difficulty range, and re-scored each one with the breadth-first solver in
`board_test.go`, since Fogleman's published move count uses a different metric.
For each resulting level, the candidate with the most vehicles was kept.

`p02` is the board of the original pygame program (4 moves).

The declared minimum is not merely documentation: it is written to every data
row (`min_moves`), so the excess of a participant's move count over the optimum
is available per trial without further analysis. `TestEmbeddedPuzzles` re-solves
the whole library on every test run and fails if a board is unsolvable,
duplicated, mis-labelled, or out of order.
