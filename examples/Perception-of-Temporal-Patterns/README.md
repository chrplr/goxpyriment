# Perception of Temporal Patterns

Replication of Experiment 1 from Povel & Essens (1985), which investigates how an internal clock influences the perception and reproduction of rhythmic sequences. Patterns that more strongly induce a regular internal beat are reproduced more accurately.

The experiment uses 35 rhythmic sequences — all permutations of the interval set {1, 1, 1, 1, 1, 2, 2, 3, 4} where the unit interval is 200 ms — divided into 7 categories of decreasing clock-induction strength.

## Trial structure

Each sequence has two phases:

1. **Learning phase**: the sequence plays on repeat. The participant taps along and presses Enter when ready to reproduce.
2. **Reproduction phase**: the participant taps Space to produce four complete periods (36 taps → 35 inter-tap intervals).

Metrics recorded: number of presentations before reproduction, and reproduction error (sum of |observed − expected| inter-tap intervals in ms).

## Running

```bash
go run main.go                   # fullscreen, default tone
go run main.go -w                # windowed
go run main.go -w -s 1           # windowed, subject ID 1
go run main.go -sound cymbal     # use cymbal sound instead of tone
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |
| `-s` | `0` | Participant ID |
| `-sound` | `tone` | Sound type: `tone` or `cymbal` |

## Controls / Response keys

| Key | Meaning |
|-----|---------|
| Space | Tap (learning: tap along; reproduction: tapping response) |
| Enter | Transition from learning to reproduction phase |
| Escape | Quit |

## Output

Data are saved to `goxpy_data/` as a `.csv` file. One row per sequence, recording sequence ID, category, number of presentations, and reproduction error in ms.

## Note on sequence accuracy

The sequence table was transcribed from an AI-generated description and may contain errors relative to the original paper. Sequences 11 and 31 are flagged as duplicates of sequences 7 and 24 in the source material. Verify all sequences against the original paper before using this for real research.

## References

Povel, D.-J., & Essens, P. (1985). Perception of temporal patterns. *Music Perception*, 2(4), 411–440. https://doi.org/10.2307/40285311
