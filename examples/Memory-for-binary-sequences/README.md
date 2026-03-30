# Memory for Binary Sequences

Replication of Experiment 1 from Planton et al. (2021), which investigates how humans memorise and detect violations in binary auditory sequences. The experiment provides evidence for a mental compression algorithm: sequences that can be described by a short recursive rule are learned more easily and reproduced more accurately.

The experiment has two parts:

1. **Complexity rating** (30 trials): listen to each of 10 binary sequences and rate its complexity on a 1–9 scale.
2. **Violation detection** (10 sessions): the sequence repeats; press Space whenever a deviant tone is heard.

## Stimuli

Each sequence is 16 items long, built from two complex tones (low and high pitch, randomly assigned to items A and B per session). Tones are 50 ms with 5 ms ramps; ISI = 200 ms; ITI = 600 ms. Deviant tones replace an expected tone with a "super-deviant" pitch level.

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
| `1`–`9` | Complexity rating (part 1) |
| Space | Violation detected (part 2) |
| Escape | Quit |

## Output

Data are saved to `goxpy_data/` as a `.csv` file. One row per trial event, recording phase, sequence ID, trial type (standard / deviant / super-deviant), deviant position, response, and RT.

## References

Planton, S., van Kerkoerle, T., Abbih, L., Maheu, M., Meyniel, F., Sigman, M., Wang, L., Figueira, S., Romano, S., & Dehaene, S. (2021). A theory of memory for binary sequences: Evidence for a mental compression algorithm in humans. *PLoS Computational Biology*, 17(1), e1008598. https://doi.org/10.1371/journal.pcbi.1008598
