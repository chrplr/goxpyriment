# Test Stream Images

Demonstrates `PresentStreamOfImages`: a high-precision RSVP (Rapid Serial Visual Presentation) loop that presents PNG images frame-by-frame, VSYNC-locked, with sub-millisecond timing accuracy.

Timing logs are written to verify that actual onset and offset times match the requested durations.

---

## Prerequisites

- Go 1.25+
- PNG image files in the working directory (or embedded in the binary)

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
| `image_index` | Position in the stream |
| `filename` | Image filename |
| `target_on_ms` | Requested onset time (ms) |
| `actual_onset_ms` | Measured onset time (ms) |
| `actual_offset_ms` | Measured offset time (ms) |

---

## Note

This is a technical test for timing verification. It is mainly useful for framework developers.
