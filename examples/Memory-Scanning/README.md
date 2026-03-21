# Memory Scanning (Sternberg)

Implements the two experiments from Sternberg (1966) demonstrating **high-speed serial scanning of short-term memory**. A set of digits is memorised; a probe digit then appears and the participant decides whether it was in the set. Reaction time increases linearly with set size, suggesting an exhaustive serial scan of memory.

- **Experiment 1 (varied set):** On each trial, 1–6 digits are shown one at a time; after a delay, a probe digit appears. Respond **F** if it was in the set, **J** if not. Set size varies from trial to trial. 24 practice + 144 test trials.

- **Experiment 2 (fixed set):** Same yes/no task, but the memorised set is fixed for a block (size 1, 2, or 4). Three blocks with 60 practice + 120 test trials each; 3.7 s between response and next trial.

## Running

```bash
# From the Memory-Scanning directory
go run main.go -d -s 1 -exp 1

# From the repository root
go run examples/Memory-Scanning/main.go -d -s 1 -exp 1
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-d` | off | Development mode: windowed 1024×768 |
| `-s` | `0` | Participant ID |
| `-exp` | `0` | Experiment: `1` (varied set), `2` (fixed set), `0` (both) |

## Controls / Response keys

| Key | Meaning |
|-----|---------|
| `F` | Probe was in the memory set (positive) |
| `J` | Probe was NOT in the memory set (negative) |
| Escape | Quit |

## Output

Data are saved to `goxpy_data/` as a `.xpd` file. Columns: `experiment`, `block`, `set_size`, `trial`, `probe`, `positive`, `key`, `rt`, `correct`.

## References

Sternberg, S. (1966). High-speed scanning in human memory. *Science*, 153(3736), 652–654. https://doi.org/10.1126/science.153.3736.652
