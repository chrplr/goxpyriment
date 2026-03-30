# Simple Example

A minimal goxpyriment experiment demonstrating the core trial loop: fixation cross, stimulus, and reaction-time recording. Five trials are run.

This is the recommended starting point for understanding the framework structure before building your own experiment.

---

## Trial structure

```
Fixation cross  →  Red rectangle  →  Key press  →  ITI
    500 ms           until key                      500 ms
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
