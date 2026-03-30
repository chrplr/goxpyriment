# Posner Covert Attention Task

A spatial cueing paradigm (Posner, 1980) that measures **covert attentional orienting**. An arrow cue appears at the centre of the screen and points left or right. After a variable delay, a target (star) appears on the left or right side. Participants respond as quickly as possible.

**Valid trials** (cue points to target side) yield faster reactions than **invalid trials** (cue points away), demonstrating the cost/benefit of spatial attention.

---

## Trial structure

```
Fixation  →  Arrow cue  →  Target  →  ITI
 500 ms       500 ms        (response)
```

---

## Prerequisites

- Go 1.25+

---

## Running

```bash
# Fullscreen, participant 1
go run main.go -s 1

# Windowed (development / testing)
go run main.go -s 1 -w
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID (integer) |
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |

---

## Response keys

Press any key as quickly as possible when the target appears.

---

## Output

Trial information is printed to the console (congruency, side, key, reaction time). No `.csv` file is written in the current version.

---

## References

Posner, M. I. (1980). Orienting of attention. *Quarterly Journal of Experimental Psychology*, 32(1), 3–25. https://doi.org/10.1080/00335558008248231
