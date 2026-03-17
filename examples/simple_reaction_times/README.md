# Simple Reaction Times

A classic **simple reaction time** task: a fixation cross appears, and after a variable foreperiod a target stimulus appears. The participant presses any key as quickly as possible. Twenty trials are run.

This example serves as a minimal but complete experiment and a starting template for RT paradigms.

---

## Trial structure

```
Fixation cross  →  Target stimulus  →  Key press  →  ITI
  variable ms       until response                   500 ms
```

---

## Prerequisites

- Go 1.25+
- SDL3 development libraries (`sudo apt install libsdl3-dev` on Ubuntu/Debian)

---

## Running

```bash
# Fullscreen, participant 1
go run main.go -s 1

# Windowed (development / testing)
go run main.go -s 1 -d
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-s` | `0` | Participant ID (integer) |
| `-d` | off | Development mode: windowed 1024×768 |

---

## Output

Data are saved to `xpd_results/` as a `.xpd` file (CSV with a metadata header). One row per trial:

| Column | Description |
|--------|-------------|
| `trial` | Trial number (1–20) |
| `wait_time` | Foreperiod duration in milliseconds |
| `key` | Key pressed |
| `rt` | Reaction time in milliseconds |
