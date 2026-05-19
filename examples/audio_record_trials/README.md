# Audio capture — 10 trials × 1 s

Records ten 1-second clips (`trial_01.wav` … `trial_10.wav`) after the participant selects a microphone. Trial metadata (including `recording_device`) is written to the session CSV.

## Running

```bash
go run main.go -w
```

| Flag | Description |
|------|-------------|
| `-w` | Windowed mode (recommended for development) |
| `-s N` | Subject ID |
| `-d N` | Display index |

## Output

| Column | Description |
|--------|-------------|
| `trial` | Trial number (1–10) |
| `wav_file` | Filename written in the data directory |
| `pcm_bytes` | Captured PCM size |
| `recording_device` | Name chosen in the microphone menu |
