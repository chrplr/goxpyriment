# Psychological Refractory Period (PRP)

Implements the classic **Psychological Refractory Period** paradigm. Two tasks overlap in time with a variable Stimulus Onset Asynchrony (SOA). The PRP effect — the finding that RT to Task 2 increases as SOA decreases — demonstrates a central processing bottleneck: the brain cannot simultaneously execute two decision-making processes.

## Task design

| Task | Stimulus | Response |
|------|----------|----------|
| Task 1 (auditory) | Low tone (400 Hz) or High tone (900 Hz) | `S` = Low, `D` = High |
| Task 2 (visual) | Letter 'A' or letter 'B' | `K` = A, `L` = B |

SOA values (time between Task 1 and Task 2 stimuli): 50, 100, 200, 400, 800 ms, randomly interleaved.

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
| `S` | Task 1 — Low tone |
| `D` | Task 1 — High tone |
| `K` | Task 2 — Letter A |
| `L` | Task 2 — Letter B |
| Escape | Quit |

Respond to each task as quickly as possible. Task 1 must be responded to before Task 2.

## Output

Data are saved to `goxpy_data/` as a `.csv` file. One row per trial, recording SOA, Task 1 stimulus and RT, Task 2 stimulus and RT, and whether each response was correct.

## References

Pashler, H. (1994). Dual-task interference in simple tasks: Data and theory. *Psychological Bulletin*, 116(2), 220–244. https://doi.org/10.1037/0033-2909.116.2.220
