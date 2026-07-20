# Test Stream Text

Demonstrates `PresentStreamOfImages` for RSVP word presentation: a sequence of words is displayed at a fixed rate, VSYNC-locked, with timing logs showing actual onset and offset times.

Use this to verify text-RSVP timing accuracy on your hardware.

---

## Prerequisites

- Go 1.25+

---

## Running

```bash
go run main.go
```

---

## Output

Data are saved to `goxpy_data/` as a `.csv` file (CSV with a metadata header):

| Column | Description |
|--------|-------------|
| `word_index` | Position in the stream |
| `word` | The presented word |
| `target_on_ms` | Requested onset time (ms) |
| `actual_onset_ms` | Measured onset time (ms) |
| `actual_offset_ms` | Measured offset time (ms) |

---

## Note

This is a technical test for timing verification. It is mainly useful for framework developers.
