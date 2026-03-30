# Multiple Object Tracking (MOT)

Replication of the paradigm introduced by Pylyshyn & Storm (1988), which demonstrates that humans can simultaneously track several independently moving objects without any visible distinguishing mark — evidence for a parallel, pre-attentive tracking mechanism.

## Trial structure

| Phase | Duration | Description |
|-------|----------|-------------|
| Highlight | 4 s | 10 stationary circles; N targets flash red |
| Motion | variable | All circles turn blue and move in random directions, bouncing off the playfield boundary and each other |
| Response | — | Motion stops; click the N circles you believe were the targets |
| Feedback | brief | Correct = green, wrong = red, missed target = orange |

Eight trials are run: two for each target count N ∈ {4, 5, 6, 7}, in random order.

## Running

```bash
go run main.go              # fullscreen
go run main.go -w           # windowed
go run main.go -w -s 1      # windowed, subject ID 1
go run main.go -speed 80    # custom dot speed (px/s)
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-w` | off | Windowed mode (1024×768 window instead of fullscreen) |
| `-d N` | -1 | Display ID: monitor index where window/fullscreen opens (-1 = primary) |
| `-s` | `0` | Participant ID |
| `-speed` | `50` | Dot speed in pixels per second |
| `-disksize` | `20` | Radius of each circle in pixels |
| `-trialduration` | `8000` | Motion phase duration in milliseconds |

## Controls / Response keys

| Action | Meaning |
|--------|---------|
| Left click | Select / deselect a circle (response phase only) |
| Escape | Quit at any time |

## Output

Data are saved to `goxpy_data/` as a `.csv` file. One row per click, recording trial number, target count, whether the clicked circle was a target, and the running score.

## References

Pylyshyn, Z. W., & Storm, R. W. (1988). Tracking multiple independent targets: Evidence for a parallel tracking mechanism. *Spatial Vision*, 3(3), 179–197.
