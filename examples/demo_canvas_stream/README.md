# Canvas stream (demo)

The [`Shape-Change-Detection`](../Shape-Change-Detection/) task, built a second
way. Each image of the stream (the fixation cross and the two shapes, left and
right) is **pre-rendered into an off-screen `stimuli.Canvas`** before the trial,
and the stream presents that sequence of canvases.

The task, the response handling, the scoring and the data columns are copied
unchanged, so the two programs differ only in how an image is made:

```bash
diff examples/Shape-Change-Detection/main.go examples/demo_canvas_stream/main.go
```

```bash
go run ./examples/demo_canvas_stream -w -s 999
```

## The two approaches

Both put one composite image per slot into the same VSYNC-locked
`stimuli.PresentStreamOfStimuliHooks`. In both, the two sides are drawn in the
same frame and cannot drift apart. What differs is *when* the drawing happens.

| | `Shape-Change-Detection` (`DrawFunc`) | this demo (`Canvas`) |
|---|---|---|
| An image is | a function that draws the cross and both shapes | a texture holding the finished picture |
| Drawing happens | every frame, during the stream | once per image, before the trial |
| Per-frame work | three small stimuli: at most a few hundred triangles | one textured rectangle, whatever the image holds |
| GPU memory | none: shapes are geometry, not textures | one texture per image: width × height × 4 bytes |
| Preparation | none | render 20 canvases per trial (`prep_ms`) |
| Clean-up | none | `Unload` every canvas after the trial |
| Code | about 10 lines | about 25 lines |

### Memory: the canvas version uses *more* texture memory, not less

Neither version loads image files, and the `DrawFunc` version allocates no
texture at all: a circle or a polygon is sent to the GPU as
triangles (a few for a polygon, up to a few hundred for an antialiased
circle) every frame. A canvas, on the other hand, is a render-target texture
of its full pixel size. This demo keeps it small by making each canvas a
horizontal band just tall enough for the largest shape (240 px), not the whole
screen:

| canvas | per canvas | per trial (20 images) |
|---|---|---|
| 1920 × 240 band (this demo, 1920-px-wide display) | 1.8 MB | 37 MB |
| 1920 × 1080, whole screen | 8.3 MB | 166 MB |

These figures are computed from the sizes (RGBA, 4 bytes a pixel), not
measured; a driver may pad textures. The actual size for your display is
written to the run's `-info.txt` (`canvas WxH px, … MB of textures per trial`).
The canvases are destroyed after each trial, so this is the peak, not a total.

### When the canvas approach is the better choice

The canvas wins when an image is **expensive to draw and cheap to show**: a
dot cloud with hundreds of antialiased dots, a scene of many pictures and text
labels, a noise mask computed pixel by pixel. With `DrawFunc`, that cost is
paid on every frame of the stream (30 times per 500 ms image at 60 Hz), and a
scene too heavy for one frame drops frames. With a canvas, it is paid once,
before the trial, and every frame of the stream then costs the same: one
texture copy.

For three simple shapes, as here, a frame is cheap either way, so the
`DrawFunc` version is the simpler one to write and to read, with no textures to
manage. If both versions hold their frames on your machine (see below), that
simplicity is the deciding factor.

### Caveats specific to canvases

- **Render-target contents can be lost.** SDL documents that on some backends
  (Direct3D on Windows) render targets may be reset, for example when the
  device is reset, and emits `SDL_EVENT_RENDER_TARGETS_RESET` when that
  happens. A canvas rendered before the trial would then show garbage or
  black. Rendering the canvases immediately before each trial, as this demo
  does, keeps that window short; it does not close it.
- **`prep_ms` is the CPU side only.** SDL hands the drawing commands to the GPU,
  which may still be executing them when the stream starts.
- **Every canvas must be unloaded**, or GPU memory grows trial after trial.

## Comparing the two on your display

Run both programs on the same machine and display, then compare the
`max_dur_err_ms` column of the two CSV files. It is computed by the same
function in both: the worst difference, per trial, between an image's
on-screen duration (`TimingLog.OffsetNS − OnsetNS`) and its nominal whole-frame
duration. 0 to a fraction of a millisecond means no frame was dropped; about
one frame (16.7 ms at 60 Hz) means one was.

The demo records one column more than the `DrawFunc` version:

| Column | Meaning |
|--------|---------|
| `prep_ms` | Wall-clock time to render the trial's 20 canvases (CPU side) |

The other columns are those of
[`Shape-Change-Detection`](../Shape-Change-Detection/README.md#data).

---

<!-- BEGIN:links -->
## Try it without building

- **[▶ Run it in your browser](https://downloads.pallier.org/builds/latest/wasm/demo_canvas_stream/)** — no download, no install.
- **[Download a prebuilt binary](https://downloads.pallier.org/builds/latest/)** — Windows, macOS, and Linux on x86-64 and arm64.

<sub>This section is generated by `make update-examples-gallery` — edit `meta.yaml`, not these lines.</sub>
<!-- END:links -->
