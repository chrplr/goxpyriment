# Code Review: goxpyriment

*Reviewed 2026-03-25. Covers core library packages, newly added examples, and the overall framework design.*

*Updated 2026-03-25: sections 1, 2, and 5.2 addressed; remaining open items below.*

---

## Summary

The framework is well-structured and the layered package architecture is sound. The SDL3 integration is thoughtful, and the CLAUDE.md files provide unusually good internal documentation.

Severity scale used below: **[SERIOUS]** = likely to cause a bug or incorrect data; **[MINOR]** = quality, consistency, or polish issue.

---

## 3. Documentation Gaps

### `control/experiment.go` — `PollEvents` reset behavior is not documented

The `EventState.LastKey` and `LastMouseButton` fields are reset to zero at the start of each `PollEvents` call (line 476–479), but `QuitRequested` is sticky. This asymmetry is described only in the `EventState` struct comment. If `PollEvents` is called twice in rapid succession within the same trial loop, the first call's key/button values are silently discarded. This should be prominently noted in the `PollEvents` docstring.

---

### `control/getinfo.go` — cache behaviour for `subject_id` not fully documented

The docstring states "subject_id is always reset to its default." However, it does not clarify that the *default* is the empty string `""`, so pressing OK without entering an ID stores an empty string. The next run will also start with an empty field (not the previous session's ID), which is the intended behaviour but may surprise users.

---

### `stimuli/stream.go` — timing precision on different platforms not documented

`PresentStreamOfImages` disables GC and aligns to VSYNC, but there is no documentation of the expected timing precision on different platforms (e.g. whether VSYNC locking is reliable on macOS Metal vs Linux X11 vs Windows). Researchers deploying this function in timing-critical RSVP experiments have no guidance on expected jitter.

---

### `control/experiment.go` — `ShowSplash` silently returns `nil` on internal errors

Lines 667 and 671 return `nil` on errors that prevent the splash from displaying (font load failure, IOStream error). The comment says "Non-fatal" but callers cannot distinguish a successful display from a silently skipped one.

---

## 4. Code Quality and Maintenance

### `control/experiment.go` — inline color literals in `getinfo.go` **[MINOR]**

Colors in the participant info dialog (`colBg`, `colBorder`, etc., lines 178–185 of `getinfo.go`) are defined as inline `sdl.Color` literals with no semantic names. If the dialog's visual style is ever updated, each literal must be found and changed individually.

---

### `examples/Number-Double-Digits-Comparison/main.go` — trial count mismatch with source paper

The description (`description.md`) states 242 experimental trials for Experiment 1, but the trial list produced by `buildExp1Trials()` yields 232 trials (44 numbers × 2 + 28 numbers in [41,69] × 4 − the excluded 55). The discrepancy of 10 trials is unexplained. Either the frequency rules in the description are incorrect or the range boundaries differ from those in the original paper.

---

## 5. Cross-Platform Concerns

### macOS — dylib double-load workaround is fragile

`control/getinfo.go` and `control/experiment.go` share a `sharedSDLLoader`/`sharedTTFLoader` cache to avoid loading two copies of `libSDL3.dylib` on macOS (which causes duplicate Objective-C class registration and a crash). The mechanism works but depends on `GetParticipantInfo` being called before `Initialize`. If the call order is reversed, or if `GetParticipantInfo` is called twice (e.g. after recovering from a validation error), the loaders are transferred to the `Experiment` on the first `Initialize` call; any subsequent `Initialize` call loads a fresh dylib. This is an invisible contract with no runtime check or documentation in the public API.

---

## 6. Newly Added Examples — Additional Notes

- **No inter-trial key drain** (`Keyboard.Clear()`): Stale keypresses from the fixation or ITI period can bleed into `WaitKeysEventRT`. The other examples (e.g. `parity_decision`) do not drain the queue either, so this is a framework-wide pattern, but it is worth flagging.

- **`Number-Comparison`: digit stimuli share `TextLine` objects between left and right positions.** A digit appears at most once per trial (all pairs have `nLeft ≠ nRight`), so there is no draw-order aliasing. This is correct as written but would silently break if pairs were ever allowed to include equal values.

---

*End of review.*
