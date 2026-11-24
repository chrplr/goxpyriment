# Frame syncing: presentation-timing APIs

**goxpyriment uses none of the APIs listed on this page.** It is a note on the
operating-system and GPU interfaces that can report when a frame was actually
displayed, kept for reference should the library ever stamp onsets from them.

## What goxpyriment does today

- **Every `Screen.Update` holds to the frame boundary**, whether or not
  `SDL_RenderPresent` blocks (`apparatus/screen_present_notjs.go`, `paceToFrame`).
  The onset `FlipTS` reports is the present's own return when the driver
  blocked, or the scheduled frame boundary when it did not. See
  `apparatus/CLAUDE.md` and [Timing Tests](TimingTests.md).
- **With `GOXPY_VBLANK=on`, onsets are anchored on the kernel's vblank
  timestamps:** `DRM_IOCTL_WAIT_VBLANK` on Linux and CVDisplayLink on macOS
  (`vblank/`). It is opt-in, because photodiode runs found it no better than the
  default (see `vblank.Enabled`).
- **On Linux, SDL is steered toward its Vulkan renderer** to avoid an NVIDIA/X11
  fullscreen bug (`apparatus/CLAUDE.md`). goxpyriment has no Vulkan code of its
  own and calls no Vulkan extension.

## Presentation-feedback interfaces, by platform

These report the time a frame reached the display rather than when it was
submitted. Listed roughly from most to least capable:

| Interface | Platform | Gives |
|---|---|---|
| `VK_EXT_present_timing` | Vulkan, recent drivers (NVIDIA, AMD, Intel) | Past presentation times and scheduling of future ones |
| `VK_KHR_present_wait` | Vulkan, widely supported | Blocking "wait until this present was displayed" |
| `VK_GOOGLE_display_timing` | Vulkan, older Android and mobile Linux | Past presentation times; a legacy fallback |
| `wp_presentation` (presentation-time protocol) | Wayland compositors | Presentation timestamp and refresh period per frame |

Using any of them would need access to the swapchain or the Wayland surface,
which SDL's 2D renderer does not expose.
