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
  implementation. The currently selected vehicle is outlined in white.
- A status line below the board shows the trial number: `Puzzle n/N — free the
  RED car`.

## 3. Interaction

The vehicle under the cursor is determined by the grid cell the cursor is in.

- **Drag.** Pressing the left button on a vehicle selects it. While the button
  is physically held, the cell under the cursor is projected onto the vehicle's
  axis and the vehicle slides toward it, one cell at a time, stopping at the
  first wall or vehicle in the way. This reproduces the pygame original's
  `MOUSEMOTION` handler exactly.
- **Click.** A press–release pair that produced no displacement leaves the
  vehicle selected (highlighted). The next click on any cell of the board slides
  that vehicle toward the clicked cell, under the same rules, and clears the
  selection.

Move legality (the port of `try_move_car`) is stepwise: a request to move past
an obstacle is not rejected, it is truncated — the vehicle ends up flush against
the obstacle. This makes dragging forgiving without ever allowing an illegal
board state.

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
  and `min_moves` columns).
- **Latency before each move**, from the `t_ms` differences — planning pauses,
  in particular the long pause typically preceding the first move.
- **Selections without displacement** (`press`/`release` pairs at the same
  cell), a trace of vehicles considered and rejected.
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
