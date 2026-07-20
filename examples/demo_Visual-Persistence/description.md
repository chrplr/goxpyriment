The aim is to create a go program, relying on the `goxpyriment` module, that reproduces the "Persistence of Vision" illusion from http://TestUFO.com/persistence


It perfectly demonstrates how our brains blend discrete spatial slices over time to form a continuous image when tracking motion.

Here is a language-independent, step-by-step programming blueprint to implement this effect.

---

## 1. High-Level Concept & Logic

The core of the illusion relies on two synchronized components:

1. **The Grid (The "Slits"):** A static or moving mask that only allows thin vertical strips of an image to be visible.
2. **The Animation (The "Shift"):** The underlying source image moves horizontally across the screen behind (or within) those slits.

When a viewer's eyes remain stationary, they see a fragmented image. When their eyes **track** the moving fragments, the brain integrates the pieces over time, making the full image magically appear.

---

## 2. Core Setup & Variables

Before writing the loop, define these fundamental variables in your program:

* `WINDOW_WIDTH` and `WINDOW_HEIGHT`: The dimensions of your display canvas.
* `IMAGE`: The source texture/image you want to display.
* `IMAGE_X`: The current horizontal position of the image (starts at `0` or negative width).
* `SPEED`: The number of pixels the image shifts per frame (e.g., `4` pixels/frame).
* `SLIT_WIDTH`: The width of the visible vertical strips (e.g., `2` or `4` pixels).
* `SLIT_GAP`: The dark space between the visible strips (e.g., `16` or `32` pixels).
* `PERIOD`: The total repeating pattern width (`SLIT_WIDTH + SLIT_GAP`).

---

## 3. The Rendering Algorithm (Two Approaches)

You can implement this either by using a **Pixel-by-Pixel Mask** or by using **Texture/Image Slicing**. The slicing method is generally much faster and hardware-accelerated.

### Method A: Vertical Strip Slicing (Recommended)

Instead of drawing the whole image, you iterate across the screen and only draw the portions of the image that fall into the "slits."

```text
For every column X coordinate from 0 to WINDOW_WIDTH, stepping by PERIOD:
    
    1. Calculate the destination rectangle on the screen:
       - Start X: X
       - Width: SLIT_WIDTH
       - Height: WINDOW_HEIGHT
       
    2. Calculate the corresponding source rectangle from the moving IMAGE:
       - Start X in image: X - IMAGE_X
       - Width: SLIT_WIDTH
       - Height: WINDOW_HEIGHT
       
    3. Draw/Blit only that slice of the IMAGE onto the screen at the destination X.

```

### Method B: Pixel Masking (Shader or Per-Pixel Loop)

If you are writing a fragment shader or modifying a pixel buffer directly, use a mathematical modulo condition to determine whether to show the image or show black.

```text
For every pixel at coordinate (Screen_X, Screen_Y):

    1. Check if the pixel falls within a slit:
       - If (Screen_X modulo PERIOD) is less than SLIT_WIDTH:
           // We are inside a slit
           Sample the pixel from the IMAGE at (Screen_X - IMAGE_X, Screen_Y)
           Color the screen pixel with the sampled color
       - Else:
           // We are in the gap between slits
           Color the screen pixel BLACK

```

---

## 4. The Update Loop (Motion)

To make the illusion work, the image *must* move. In your game loop or animation frame update:

1. **Advance the Position:** Increase `IMAGE_X` by `SPEED` every frame.
2. **Handle Boundaries (Looping):** * If `IMAGE_X` exceeds the `WINDOW_WIDTH`, reset `IMAGE_X` to `-IMAGE_WIDTH` so it seamlessly scrolls back onto the screen from the left.
3. **Clear and Redraw:** Clear the screen to black, execute your rendering method from Step 3, and swap the display buffers.

---

## 5. Tips for Maximizing the Illusion

* **Frame Rate Consistency:** The illusion relies heavily on smooth motion. Target a consistent **60Hz or higher** (matching the monitor's refresh rate perfectly via VSync).
* **The "Gap to Slit" Ratio:** Start with a `SLIT_WIDTH` of 2 pixels and a `SLIT_GAP` of 18 pixels (Ratio of 1:9). If the gap is too small, the image is too easily visible without eye tracking. If the gap is too large, the image becomes too dim.
* **High Contrast Images:** Use source images with bold shapes, text, or high contrast (like a black-and-white checkerboard or bright cartoon character). Fine details get lost easily.
* **Eye-Tracking Guide:** Consider drawing a small, bright dot that moves horizontally at the exact same `SPEED` as the image. Tell the programmer to instruct users to stare at and track the dot—this forces the eyes to move at the correct speed to unlock the illusion.
