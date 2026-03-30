# Stop-Signal Task

Implementation of **Experiment 1** from Logan, Cowan & Davis (1984), the foundational study establishing the horse-race model of response inhibition.

Participants perform letter-identification tasks and must occasionally withhold their response when they hear a stop-signal tone. Because the go process and the stop process race independently, response inhibition is probabilistic: at short delays the stop process usually wins; at long delays the go process usually wins. The resulting **inhibition function** — P(inhibit) as a function of stop-signal delay — is the central dependent variable.

---

## Trial structure

```
Fixation dot      →    Letter      →    (tone at delay)    →    ITI
   500 ms            until resp             optional           2500 ms
                    or 1000 ms
```

On **20 %** of trials a 900 Hz, 500 ms tone is played at a fixed delay after letter onset. The participant must try to stop their response when they hear it.

---

## Tasks

### Simple RT
Press **SPACE** for any letter (E, F, H, L).

Stop-signal delays: **50 / 100 / 150 / 200 ms**

### Choice RT
Press **F** or **J** depending on which letter appeared (mapping assigned in the setup dialog).

Stop-signal delays: **100 / 200 / 300 / 400 ms**

---

## Response keys

| Task | Key | Meaning |
|------|-----|---------|
| Simple RT | `SPACE` | Go — respond to any letter |
| Choice RT | `F` | Go — one letter group |
| Choice RT | `J` | Go — other letter group |

Six balanced letter-to-key mappings are available (all partitions of {E, F, H, L} into two pairs); the mapping is selected in the setup dialog and counterbalanced across participants.

---

## Design

- **8 blocks** of 80 trials each: 4 simple-RT blocks + 4 choice-RT blocks
- Block order counterbalanced by subject ID (odd → simple first; even → choice first)
- Per block: 64 go trials (16 per letter) + 16 stop-signal trials (1 per letter × delay combination)
- Stop-signal delays are fixed throughout the session (Experiment 1)

---

## Prerequisites

- Go 1.25+
- Audio output device (for the stop-signal tone)

---

## Running

```bash
go run .
```

A graphical setup dialog opens first. Select the subject ID, the choice-task letter mapping, and whether to run fullscreen.

### Command-line flags

| Flag | Default | Description |
|------|---------|-------------|
| `-w` | off | Windowed mode (1024×768) — useful for testing |
| `-d N` | -1 | Display ID: monitor index (-1 = primary) |

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file. One row per trial:

| Column | Description |
|--------|-------------|
| `trial` | Trial number (1–640) |
| `block` | Block number (1–8) |
| `task` | `simple` or `choice` |
| `letter` | Letter shown (`E`, `F`, `H`, or `L`) |
| `stop_signal` | Whether a stop signal occurred (`true` / `false`) |
| `stop_delay_ms` | Stop-signal delay in ms (0 on go trials) |
| `response_key` | SDL keycode of the key pressed (`none` if no response) |
| `rt_ms` | Reaction time in ms from letter onset (0 if no response) |
| `correct` | `true` if response was correct (or successfully inhibited) |
| `inhibited` | `true` if stop signal occurred and no response was made |

### Key analyses

- **P(inhibit) vs delay**: proportion of `inhibited=true` trials at each `stop_delay_ms`
- **Mean go-RT**: mean `rt_ms` where `stop_signal=false`
- **Signal-respond RT**: mean `rt_ms` where `stop_signal=true` and `inhibited=false`
- **SSRT estimate** (horse-race model): integrate the go-RT distribution until the area equals P(inhibit), then subtract stop-signal delay

---

## References

Logan, G. D., Cowan, W. B., & Davis, K. A. (1984). On the ability to inhibit simple and choice reaction time responses: A model and a method. *Journal of Experimental Psychology: Human Perception and Performance*, 10(2), 276–291. https://doi.org/10.1037/0096-1523.10.2.276

Logan, G. D. (1994). On the ability to inhibit thought and action: A users' guide to the stop signal paradigm. In D. Dagenbach & T. H. Carr (Eds.), *Inhibitory processes in attention, memory, and language* (pp. 189–239). Academic Press.
