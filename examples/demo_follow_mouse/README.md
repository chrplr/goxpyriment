# Follow Mouse

A minimal **mouse-tracking** demo. A white dot follows the mouse cursor in real time. Press ESC to exit.

This example illustrates:

- Using `exp.Screen.MousePosition()` to read the current cursor position in the center-based coordinate system (0, 0 = screen center), correctly handling HiDPI and logical scaling.
- A rendering loop with no waiting: `exp.Run` pumps SDL events every frame automatically, keeping the window responsive and updating mouse state without any explicit `exp.Wait` call.

---

## Running

```bash
# Fullscreen
go run main.go

# Windowed (development / testing)
go run main.go -w
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display index (-1 = primary) |
