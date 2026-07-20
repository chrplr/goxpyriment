# test_menu

An interactive demonstration of the `stimuli.Menu` widget. Run it to see all
the configuration options before using a menu in your own experiment.

## Running

```bash
go run examples/test_menu/main.go -w        # windowed mode (recommended)
go run examples/test_menu/main.go           # fullscreen
```

Press **ESC** at any time to quit.

## What it covers

| Section | What it demonstrates |
|---------|----------------------|
| 1 | Basic menu with default colors — navigate with arrow keys or digit keys |
| 2 | Pre-selected item (`initialSel = 2` starts with "Hard" highlighted) |
| 3 | Custom `TextColor`, `HighlightColor`, and off-center `Pos` |
| 4 | Twelve items — shows how key `0` maps to item 10; items 11–12 need arrows |

## API summary

```go
// Create
m := stimuli.NewMenu([]string{"Option A", "Option B", "Option C"})

// Optional customisation
m.Pos            = sdl.FPoint{X: 0, Y: 0}              // center-based position
m.TextColor      = sdl.Color{R: 200, G: 200, B: 200, A: 255}
m.HighlightColor = sdl.Color{R: 255, G: 220, B: 0, A: 255}
m.LineSpacing    = 48   // pixels between item centers (0 = auto)
m.Font           = myFont  // nil = screen default font

// Display and collect — blocks until selection
idx, err := m.Get(exp.Screen, exp.Keyboard, initialSel)
// idx is 0-based; returns -1 + sdl.EndLoop on ESC/quit
```

## Navigation keys

| Key | Action |
|-----|--------|
| UP / DOWN | Move highlight |
| ENTER or SPACE | Confirm current highlight |
| 1 – 9 | Select and confirm item 1–9 directly |
| 0 | Select and confirm item 10 directly |
| ESC | Abort (returns `sdl.EndLoop`) |
