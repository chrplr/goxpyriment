# MEG left-right: a two-button arrow task

A dark-grey fixation cross sits on a mid-grey background. Every 3–8 s it is
replaced for 800 ms by a **`<`** (press the **left** button) or a **`>`**
(press the **right** button). The arrows have the cross's height (16 px), line
width (3 px) and colour, so only the shape changes.

The run lasts `-duration` seconds (default 300) and holds as many arrows as
the timing rules allow, about one every 6.3 s; exactly half ask for the left
button and half for the right, in random order (an odd count loses its last
arrow). A press counts only within 2 s of an arrow's onset, and arrows are at
least 3.8 s apart, so every press belongs to at most one arrow.

This is the visual task of [`../MEG-localizer3/`](../MEG-localizer3/) without
the retinotopic stimulation and the sounds. It yields left/right motor
responses, time-locked to marked visual onsets.

## Usage

```bash
# From the repo root (go.work resolves the workspace).

# A full run, fullscreen.
go run ./examples/MEG-left-right -s 1

# MEG: pulse line 0 of a DLP-IO8 at each '<', line 1 at each '>' (the
# defaults), response box sending keys b and y.
go run ./examples/MEG-left-right -s 1 -ttl dlpio8 -left-key b -right-key y

# No photodiode square (it is on by default).
go run ./examples/MEG-left-right -s 1 -photodiode=false

# A short unattended run, for testing.
go run ./examples/MEG-left-right -w -s 999 -autostart -duration 30
```

Besides the standard `-w`, `-d N` and `-s <id>`:

| Flag | Default | Meaning |
|---|---|---|
| `-duration S` | 300 | Run duration, in s |
| `-autostart` | off | Skip the instruction and end screens (unattended runs) |
| `-seed N` | 1 | Seed for the arrow schedule (combined with the subject ID) |
| `-left-key K` / `-right-key K` | `left` / `right` | Keys sent by the two response buttons: `left`, `right`, `up`, `down`, `space`, `a`–`z`, `0`–`9` |
| `-wait-trigger` | off | Wait for the scanner pulse (key `t`) before the run |
| `-ttl KIND[:PORT]` | none | TTL device for the onset markers: `dlpio8`, `dlpio20`, `megttlbox:/dev/ttyACM0`, `mmbts:/dev/ttyACM0`, `parallel`, `gpio`, `ft232h`, `labjackt4:HOST`, `null` |
| `-ttl-arrow-left N` / `-ttl-arrow-right N` | 0 / 1 | TTL line (0–7) pulsed at the onset of a `<` / `>` (codes 1 / 2 by default); they must differ |
| `-ttl-sync-code N` | 4 | TTL code (1–255) written at the start and at the end of the run, for a clock-drift check; `0` disables |
| `-ttl-ms N` | 5 | Pulse width in ms; must be shorter than an arrow |
| `-photodiode` | on | Flash a white square in the top-left corner for one frame at every arrow onset, drawn in the same flip as the onset; the square is black on every other frame. `-photodiode=false` disables it |
| `-photodiode-size N` | 100 | Side of the photodiode square, in pixels |
| `-marker-frames N` | 1 | Frames the square stays white from each onset |

The photodiode square and the TTL pulse are the library's onset marker
(`control.MarkerFlags`, `exp.Marker`). The pulse is raised right after the
onset flip returns, which is one to three frames *before* the arrow reaches
the screen; the photodiode measures that lag. Responses are logged in the CSV
only, not sent as TTL. Hardware triggers are desktop-only: the browser build
refuses `-ttl`; `-photodiode` works there.

## Data

One row per event, in `goxpy_data/MEG-left-right_sub-NNN_date-….csv`.

| Column | Description |
|---|---|
| `event_type` | `arrow_left`, `arrow_right`, `response`, `miss`, or `sync_start` / `sync_end` |
| `onset_ms` | Flip time of the arrow onset, or time of the press, in ms from the start of the run |
| `side` | The button the arrow asks for: `left` or `right` (blank for a false alarm) |
| `button` | For a response, the button pressed |
| `outcome` | For a response: `correct`, `wrong` (other button), `extra` (a second press to the same arrow) or `false_alarm` (no arrow in the last 2 s); `miss` on a miss row |
| `rt_ms` | For a response attributed to an arrow, ms since that arrow's onset |

A `miss` row is written when an arrow's 2 s window closes with no press; its
`onset_ms` is the arrow's onset. The end of the `-info.txt` summarises events,
correct, wrong and misses per side.

At the end of the run a result screen shows, for `<` and `>`, the number of
correct responses over the number of arrows. An arrow counts as correct only
if the first press in its window was the right button. The screen stays up
until **SPACE** is pressed, so a late press on a response button cannot
dismiss it. `-autostart` skips it.

### Clock-drift check

With `-ttl`, the code `-ttl-sync-code` (4 by default) is written once when the
run starts and once when it ends, even if the run is aborted with ESC. The
two writes are logged as `sync_start` and `sync_end` rows, timed on the SDL
clock just before each write. The `-info.txt` file gives their raw times
(`sync_start_ns`, `sync_end_ns`) and the interval between them
(`sync_interval_ms`). Compare that interval with the interval between the two
events in the MEG file: their difference is the drift between the two clocks
over the run. With the default lines the arrows send codes 1 and 2, so the
sync code 4 is distinct from both.

`onset_ms` comes from `Screen.FlipTS`, and response times from the SDL event
clock, so onsets and responses share one clock.

<!-- BEGIN:links -->
## Try it without building

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/MEG-left-right/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
