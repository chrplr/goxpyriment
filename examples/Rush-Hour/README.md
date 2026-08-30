# Rush Hour — sliding-block problem solving

A Go/goxpyriment port of the **Rush Hour** puzzle (Nob Yoshigahara / ThinkFun),
turned into a behavioural experiment: a series of trials, each a different 6×6
traffic jam, with **every mouse action recorded**.

Rush Hour is a classic task in the study of planning and insight: the state
space is small enough to be solved exhaustively (so each puzzle has an exact
minimum-move count) but large enough that people plan, backtrack, and get stuck.
Because the results file holds the complete action sequence, the analysis is not
limited to "solved / time taken" — the solution path, the detours away from the
optimal line, the pauses before each move, and the vehicles the participant
touches without moving are all recoverable.

```bash
go run ./examples/Rush-Hour                  # first 12 puzzles, fullscreen
go run ./examples/Rush-Hour -w -s 3          # windowed, subject 3
go run ./examples/Rush-Hour -n 20            # first 20 puzzles
go run ./examples/Rush-Hour -n 0             # the whole 49-puzzle library
```

---

## Task

Each trial shows a 6×6 grid of vehicles. The **red** car sits on row 2 and must
be driven out through the opening in the right wall. Vehicles slide only along
their own axis (a horizontal car left/right, a vertical car up/down) and cannot
pass through each other or the walls.

A trial ends **only when the puzzle is solved** — there is no time limit and no
skip key, so every trial contributes a complete solution path. `ESC` (or closing
the window) ends the whole session; data collected up to that point is kept,
since the file is flushed after every puzzle.

### Moving a vehicle

**One click, one cell.** Click on the side of a vehicle that points the way you
want it to go: anywhere left of its midline sends a horizontal vehicle one cell
left, anywhere right of it one cell right, and likewise above/below for a
vertical one. The split is on the vehicle's midline, not on cell boundaries, so
3-cell vehicles have no inert middle. A step into a wall or into another vehicle
leaves the board unchanged.

There is no selection state and no dragging. The vehicle under the cursor is
outlined in white as a hover cue, but that highlight carries no state — it only
says which vehicle a click would act on.

This differs from the pygame original, which drags. One click = one cell makes
every move a discrete, unambiguous event, which is what the data file records.

---

## Puzzles

`puzzles.txt` is embedded in the binary (`//go:embed`) and holds a library of
**49 puzzles in increasing difficulty, from 3 to 51 moves**, one per line:

```
<name>: <minimum moves>: <board>

p02:  4: BCCCoo BoooDo oAAEDo oooEoo FFoEoo ooGGGo   #  7 vehicles
p49: 51: GBBoLo GHIoLM GHIAAM CCCKoM ooJKDD EEJFFo   # 13 vehicles
```

One character per cell, read row-major: `o` (or `.`) is an empty cell, `A` is
the red target car, and the other letters are the remaining vehicles (2 or 3
cells each). Whitespace is ignored, so a board can also be written as six
6-character groups, as above.

The minimum-move count counts one slide of a vehicle, over any distance, as a
single move. It is written to every data row (`min_moves`), which makes
"excess moves over the optimum" a per-trial measure requiring no extra
analysis. `n_moves` is on the same scale: because a click only ever displaces a
vehicle by one cell, consecutive clicks on the same vehicle in the same
direction are counted as a single slide. The raw click count is recoverable by
counting `click_move` rows.

A full session of 49 puzzles is long — the last ones take many minutes each —
so `-n N` presents only the first *N* (default 12, `-n 0` for all). Because the
file is ordered easy to hard, a prefix is a graded curriculum.

The boards come from **Michael Fogleman's exhaustive Rush Hour database**
(2,577,412 puzzles, [michaelfogleman.com/rush](https://www.michaelfogleman.com/rush/)):
wall-free positions with at least 8 vehicles, one per difficulty level, re-scored
with the solver in `board_test.go` (Fogleman's own move metric differs). `p02` is
the board of the original pygame program.

Every line is parsed and validated at startup — a vehicle that is bent, of the
wrong length, or a target car off row 2 aborts the program before the first
trial rather than producing an unsolvable board. `TestEmbeddedPuzzles` goes
further: it solves each puzzle by breadth-first search and fails if one is
unsolvable, duplicated, mis-labelled, or out of order.

To use a different puzzle set, edit `puzzles.txt` and rebuild. The declared
move count may be omitted (`name: board`); the test then only checks
solvability and ordering.

---

## Results

One CSV row per **action**, plus a summary row per puzzle.

| Column | Meaning |
|---|---|
| `trial`, `puzzle` | Trial number (1-based) and puzzle name from `puzzles.txt` |
| `min_moves` | Length of the shortest solution for this puzzle |
| `event` | `trial_start`, `click_move`, `click_blocked`, `click_empty`, `trial_end` |
| `t_ms` | Milliseconds since the onset of the puzzle |
| `event_ts_ns` | SDL3 hardware timestamp (ns) — on the three `click_*` rows only |
| `mouse_x`, `mouse_y` | Cursor position, center-relative, +Y up |
| `car`, `orientation` | Vehicle letter and `H`/`V` |
| `from_row`, `from_col` | Position of the vehicle before the move |
| `to_row`, `to_col` | Position after the move |
| `n_moves`, `solved`, `trial_ms` | Summary — filled on the `trial_end` row only (`-1` / `false` elsewhere) |

Every click produces exactly one row. `click_move` is a click that displaced a
vehicle by one cell; `click_blocked` is a click on a vehicle that could not move
that way (a wall or a neighbouring vehicle); `click_empty` is a click
that hit no vehicle at all. The latter two are the record of hesitations and
failed attempts. Because the board is deterministic, replaying the `click_move`
rows in order reconstructs the exact board state at any point in the trial.

---

## Implementation notes

| File | Role |
|---|---|
| `board.go` | Puzzle logic — parsing, move legality, win test. No SDL, fully unit-tested |
| `render.go` | Drawing and the cell ↔ screen-coordinate mapping |
| `main.go` | Trial loop, input state machine, data logging |
| `puzzles.txt` | The embedded puzzle set |

Differences from the pygame original (`rush_hour/rush.py`): a series of puzzles
instead of a single hardcoded board, action logging, one-click/one-cell moves in
place of dragging, and square vehicle corners (SDL's `RenderFillRect` has no
border radius).

## Reference

- Nob Yoshigahara, *Rush Hour* (ThinkFun, 1996).
- M. Fogleman, ["Solving Rush Hour, the Puzzle Game"](https://www.michaelfogleman.com/rush/)
  (2018) — exhaustive analysis of the state space, the board notation used here,
  and the puzzle database the library is drawn from.

---

<!-- BEGIN:links -->
## Try it without building

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/Rush-Hour/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
