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
