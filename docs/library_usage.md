# Library usage: reducing direct SDL use

The examples in `examples/` sometimes call the underlying **go-sdl3** (`sdl`) library directly. The goxpyriment library provides wrappers and re-exports so that typical experiment code can avoid importing `github.com/Zyko0/go-sdl3/sdl` for common operations.

## Run-loop exit and error handling

- **Return** from your `exp.Run(...)` callback with `control.EndLoop` for normal exit (e.g. ESC or window close), instead of `sdl.EndLoop`.
- **Check** the error after `exp.Run(...)` with `control.IsEndLoop(err)` instead of comparing to `sdl.EndLoop`:

  ```go
  err := exp.Run(func() error {
      // ...
      return control.EndLoop
  })
  if err != nil && !control.IsEndLoop(err) {
      log.Fatal(err)
  }
  ```

## Key and button constants

Use **control** key and button constants so you don’t need the `sdl` import for key names:

- Keys: `control.K_SPACE`, `control.K_ESCAPE`, `control.K_S`, `control.K_F`, `control.K_J`, `control.K_Q`, `control.K_R`, `control.K_G`, `control.K_B`, `control.K_Y`, `control.K_N`, `control.K_1`–`control.K_4`, `control.K_KP_1`–`control.K_KP_4`.
- Buttons: `control.BUTTON_LEFT`, `control.BUTTON_RIGHT`.
- Type: `control.Keycode` (alias for `sdl.Keycode`).

Example: `exp.Keyboard.WaitKeys([]sdl.Keycode{control.K_SPACE}, -1)` can stay as-is (Keycode is the same type), or use `[]control.Keycode{control.K_SPACE}` if you only import control.

## Points and colors

- **Points:** use `control.Point(x, y)` or `control.Origin()` instead of constructing `sdl.FPoint{X: x, Y: y}` or `sdl.FPoint{X: 0, Y: 0}`.
- **Colors:** use `control.RGB(r, g, b)` or `control.RGBA(r, g, b, a)` for ad-hoc colors, and predefined colors like `control.DarkGray`, `control.LightGray`, `control.Black`, `control.White`, etc.

## Screen

- Use **`exp.Screen.ClearAndUpdate()`** instead of calling `exp.Screen.Clear()` then `exp.Screen.Update()` when you clear the backbuffer and present in one step.

## Timing

- Use **`clock.Wait(ms)`** for delays (milliseconds) instead of `sdl.Delay(ms)`. This keeps timing in one place and avoids pulling in SDL for the main loop throttle (e.g. `clock.Wait(10)` in event loops).

## When you still need SDL

You will still need the `sdl` import for:

- Constructing **sdl.FRect** or other SDL types not yet wrapped (e.g. in `play_two_videos` for destination rects).
- Any SDL API not yet exposed by goxpyriment (e.g. raw `sdl.PollEvent` in special cases like loading screens).
- Passing SDL types into stimuli that accept them (e.g. `sdl.Color` is still used by stimuli; control re-exports colors and RGB/RGBA helpers).

Over time, more of these can be moved behind goxpyriment APIs so that most experiments only need `control`, `io`, `misc`, and `stimuli`.
