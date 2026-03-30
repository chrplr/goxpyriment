# Subliminal Priming (Dehaene et al.)

Replication of the stimulus stream from Dehaene et al. (2001), investigating **visual masking**: a four-letter word flashed for a few tens of milliseconds remains readable in isolation, but becomes invisible when surrounded by visual masks — even though it can still influence subsequent processing.

## Prerequisites

This experiment requires a commercial font that cannot be redistributed:

1. Download **Octin College Regular** (free for personal use) from https://www.fontspring.com/fonts/typodermic/octin-college
2. Place `octin_college_rg.ttf` in `assets/font/`

## Trial types

| Condition | Context frames | Target |
|-----------|---------------|--------|
| `visible_word` | Blank frames (71 ms each) | Word |
| `visible_blank` | Blank frames | Blank |
| `masked_word` | Mask frames (71 ms each) | Word |
| `masked_blank` | Mask frames | Blank |

Each 2400 ms trial embeds target sequences at 500 ms intervals within a continuous filler stream (72 % masks, 28 % blanks, each 43, 57, or 71 ms).

## Running

```bash
go run main.go              # fullscreen
go run main.go -w           # windowed
go run main.go -w -s 1      # windowed, subject ID 1
go run main.go -targets 4   # 4 target sequences per trial (default 1)
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |
| `-s` | `0` | Participant ID |
| `-targets` | `1` | Number of target sequences per trial |

## Controls / Response keys

| Key | Meaning |
|-----|---------|
| Any key | Report whether a word was seen (as instructed) |
| Escape | Quit |

## Output

Data are saved to `goxpy_data/` as a `.csv` file. One row per trial:

| Column | Description |
|--------|-------------|
| `subject_id` | Participant ID |
| `trial_num` | Trial number |
| `condition` | Trial type (visible_word, etc.) |
| `word` | The embedded word (or empty for blank conditions) |
| `word_duration_ms` | Target presentation duration |
| `response` | Participant's key press |
| `rt_ms` | Reaction time in milliseconds |
| `reported_word` | Word reported by participant |

## References

Dehaene, S., Changeux, J.-P., Naccache, L., Sackur, J., & Sergent, C. (2006). Conscious, preconscious, and subliminal processing: A testable taxonomy. *Trends in Cognitive Sciences*, 10(5), 204–211.

Dehaene, S., Naccache, L., Cohen, L., Bihan, D. L., Mangin, J.-F., Poline, J.-B., & Rivière, D. (2001). Cerebral mechanisms of word masking and unconscious repetition priming. *Nature Neuroscience*, 4(7), 752–758.
