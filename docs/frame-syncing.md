To ensure your software has perfect pacing on as many systems as possible, you should follow this hierarchy of detection in your Vulkan renderer:
* Priority Method Hardware/OS Support 
* Primary VK_EXT_present_timing Best for 2026+ systems (NVIDIA, AMD, Intel).
* Secondary VK_KHR_present_waitGood for simpler "wait until displayed" logic; widely supported.
* Legacy VK_GOOGLE_display_timingKeep as a fallback for older Android-based or mobile Linux setups.
* System Wayland wp_presentationIf running natively on Wayland, use the presentation-time protocol.
