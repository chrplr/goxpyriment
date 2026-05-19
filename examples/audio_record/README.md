# Audio capture (demo)

Single-session microphone capture via SDL3. After choosing an input device from a menu, press **SPACE** to record ~2 s; the clip is saved as `session_mic.wav` next to the session CSV.

## Running

```bash
go run main.go -w
```

| Flag | Description |
|------|-------------|
| `-w` | Windowed mode (recommended for development) |
| `-s N` | Subject ID |
| `-d N` | Display index |

## Notes

- Use the device menu to pick a physical microphone if **System default** is silent on your machine (common on Linux/PipeWire).
