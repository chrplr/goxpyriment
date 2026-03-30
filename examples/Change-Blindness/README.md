# Change Blindness — Rensink Flicker Paradigm

Demonstrates **change blindness**: large changes to a visual scene are surprisingly hard to detect when the change is hidden by a brief blank interval between alternating images. Participants press Space as soon as they spot the change.

A 5×5 grid of coloured squares flickers between two versions (A and A′) with blank screens in between. Exactly one cell differs in colour between A and A′. The flicker cycle repeats until the participant responds or a 10-second timeout occurs.

## Timing (Rensink formula)

| Phase | Duration |
|-------|----------|
| Image A | 240 ms |
| Blank | 80 ms |
| Image A′ | 240 ms |
| Blank | 80 ms |

## Running

```bash
go run main.go          # fullscreen
go run main.go -w       # windowed
go run main.go -w -s 1  # windowed, subject ID 1
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |
| `-s` | `0` | Participant ID |

## Controls / Response keys

| Key | Meaning |
|-----|---------|
| Space | Change detected |

If no response is given within 10 seconds the trial is recorded as a miss.

## Output

Data are saved to `goxpy_data/` as a `.csv` file. One row per trial:

| Column | Description |
|--------|-------------|
| `trial` | Trial number |
| `change_row` | Row index of the changed cell (0-based) |
| `change_col` | Column index of the changed cell (0-based) |
| `color_before` | Cell colour in image A |
| `color_after` | Cell colour in image A′ |
| `rt_ms` | Reaction time in milliseconds (−1 if timed out) |
| `detected` | `true` if participant responded within the timeout |

## References

Rensink, R. A., O'Regan, J. K., & Clark, J. J. (1997). To see or not to see: The need for attention to perceive changes in scenes. *Psychological Science*, 8(5), 368–373.
