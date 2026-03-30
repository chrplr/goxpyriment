// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
	"github.com/chrplr/goxpyriment/apparatus"
)

// SplashScreen displays a logo image above a message string, centred on
// screen, and waits until the timeout elapses or any key is pressed.
//
//   - imageData: PNG/JPG bytes of the logo (e.g. from //go:embed). Pass nil
//     to show the message only.
//   - message: text rendered with the screen's default font. Pass "" to show
//     the image only.
//   - timeoutSec: maximum wait in seconds. Pass 0 to wait indefinitely until
//     a key is pressed.
//
// The image and message are stacked vertically and centred as a group, so the
// display looks good on any resolution.
//
// Returns nil on normal exit (timeout or keypress).
// Returns sdl.EndLoop if ESC or the window-close button was used.
func SplashScreen(screen *apparatus.Screen, imageData []byte, message string, timeoutSec float64) error {
	const (
		gap      = float32(30) // vertical space between image bottom and text top
		msgWidth = int32(700)  // word-wrap width for the message
	)

	// ── 1. Load and preload the image ────────────────────────────────────────

	var pic *Picture
	if imageData != nil {
		pic = NewPictureFromMemory(imageData, 0, 0)
		if err := pic.preload(screen); err != nil {
			return err
		}
	}

	// ── 2. Build and preload the message ─────────────────────────────────────

	// Choose a text colour that contrasts with the screen background.
	textColor := contrastWith(screen.BgColor)

	var txt *TextBox
	if message != "" {
		txt = NewTextBox(message, msgWidth, sdl.FPoint{}, textColor)
		if screen.DefaultFont != nil {
			if err := txt.preload(screen, screen.DefaultFont); err != nil {
				return err
			}
		}
	}

	// ── 3. Layout: centre the (image + gap + text) block vertically ──────────

	imgH := float32(0)
	if pic != nil {
		imgH = pic.Height
	}
	txtH := float32(0)
	if txt != nil {
		txtH = txt.Height
	}

	totalH := imgH + txtH
	if pic != nil && txt != nil {
		totalH += gap
	}

	// Position each element so the block is centred at (0, 0).
	if pic != nil {
		pic.Position.Y = -totalH/2 + imgH/2
	}
	if txt != nil {
		txt.Position.Y = totalH/2 - txtH/2
	}

	// ── 4. Render ─────────────────────────────────────────────────────────────

	if err := screen.Clear(); err != nil {
		return err
	}
	if pic != nil {
		if err := pic.Draw(screen); err != nil {
			return err
		}
	}
	if txt != nil {
		if err := txt.Draw(screen); err != nil {
			return err
		}
	}
	if err := screen.Update(); err != nil {
		return err
	}

	// ── 5. Wait for timeout or keypress ──────────────────────────────────────

	result := splashWait(timeoutSec)

	// ── 6. Release GPU textures ───────────────────────────────────────────────

	if pic != nil {
		_ = pic.Unload()
	}
	if txt != nil {
		_ = txt.Unload()
	}

	return result
}

// splashWait blocks for up to timeoutSec seconds (or indefinitely if
// timeoutSec <= 0) and returns as soon as a key is pressed or the window is
// closed. It returns sdl.EndLoop on ESC / quit, nil otherwise.
func splashWait(timeoutSec float64) error {
	var deadline time.Time
	timed := timeoutSec > 0
	if timed {
		deadline = time.Now().Add(time.Duration(timeoutSec * float64(time.Second)))
	}

	for {
		var ev sdl.Event

		if timed {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return nil // timed out cleanly
			}
			if !sdl.WaitEventTimeout(&ev, int32(remaining.Milliseconds())) {
				return nil // timed out cleanly
			}
		} else {
			if sdl.WaitEvent(&ev) != nil {
				return nil // WaitEvent error — treat as clean exit
			}
		}

		switch ev.Type {
		case sdl.EVENT_QUIT:
			return sdl.EndLoop
		case sdl.EVENT_KEY_DOWN:
			if ev.KeyboardEvent().Key == sdl.K_ESCAPE {
				return sdl.EndLoop
			}
			return nil
		}
	}
}

// TwoLineSplash displays an optional image above two centred text lines
// rendered at different font sizes, then waits until the timeout elapses or
// any key is pressed.
//
//   - imageData: PNG/JPG bytes for an icon shown above the text. Pass nil to omit.
//   - titleFont / title:         upper text line (usually larger).
//   - subtitleFont / subtitle:   lower text line (usually smaller).
//   - timeoutSec: maximum wait in seconds; pass 0 to wait indefinitely.
//   - splitLayout: when true, the title is centred at Y=0 independently, and
//     the image+subtitle block is placed in the lower third of the screen.
//     When false, all three elements are stacked and centred as a group.
//
// Returns sdl.EndLoop on ESC or window-close; nil on timeout or any other key.
func TwoLineSplash(screen *apparatus.Screen, imageData []byte, titleFont *ttf.Font, title string, subtitleFont *ttf.Font, subtitle string, timeoutSec float64, splitLayout bool) error {
	const gap = float32(20)

	textColor := contrastWith(screen.BgColor)

	// ── Image ────────────────────────────────────────────────────────────────
	var pic *Picture
	if imageData != nil {
		pic = NewPictureFromMemory(imageData, 0, 0)
		if err := pic.preload(screen); err != nil {
			return err
		}
	}

	// ── Title text ───────────────────────────────────────────────────────────
	var titleTxt *TextBox
	if title != "" && titleFont != nil {
		titleTxt = NewTextBox(title, 900, sdl.FPoint{}, textColor)
		if err := titleTxt.preload(screen, titleFont); err != nil {
			return err
		}
	}

	// ── Subtitle text ────────────────────────────────────────────────────────
	var subTxt *TextBox
	if subtitle != "" && subtitleFont != nil {
		subTxt = NewTextBox(subtitle, 700, sdl.FPoint{}, textColor)
		if err := subTxt.preload(screen, subtitleFont); err != nil {
			return err
		}
	}

	// ── Layout ───────────────────────────────────────────────────────────────
	imgH := float32(0)
	if pic != nil {
		imgH = pic.Height
	}
	titleH := float32(0)
	if titleTxt != nil {
		titleH = titleTxt.Height
	}
	subH := float32(0)
	if subTxt != nil {
		subH = subTxt.Height
	}

	if splitLayout {
		// Title centred at Y=0; image+subtitle block in the lower third.
		if titleTxt != nil {
			titleTxt.Position.Y = 0
		}
		bottomElems := 0
		for _, h := range []float32{imgH, subH} {
			if h > 0 {
				bottomElems++
			}
		}
		bottomH := imgH + subH + float32(max(bottomElems-1, 0))*gap
		// Centre of bottom block at 1/3 of the way from screen centre to bottom.
		blockCenterY := -float32(screen.Height) / 3
		cursor := blockCenterY - bottomH/2
		if pic != nil {
			pic.Position.Y = cursor + imgH/2
			cursor += imgH + gap
		}
		if subTxt != nil {
			subTxt.Position.Y = cursor + subH/2
		}
	} else {
		// All elements stacked and centred at (0, 0).
		elems := 0
		for _, h := range []float32{imgH, titleH, subH} {
			if h > 0 {
				elems++
			}
		}
		totalH := imgH + titleH + subH + float32(max(elems-1, 0))*gap

		cursor := -totalH / 2
		if pic != nil {
			pic.Position.Y = cursor + imgH/2
			cursor += imgH + gap
		}
		if titleTxt != nil {
			titleTxt.Position.Y = cursor + titleH/2
			cursor += titleH + gap
		}
		if subTxt != nil {
			subTxt.Position.Y = cursor + subH/2
		}
	}

	// ── Render once ───────────────────────────────────────────────────────────
	if err := screen.Clear(); err != nil {
		return err
	}
	if pic != nil {
		if err := pic.Draw(screen); err != nil {
			return err
		}
	}
	if titleTxt != nil {
		if err := titleTxt.Draw(screen); err != nil {
			return err
		}
	}
	if subTxt != nil {
		if err := subTxt.Draw(screen); err != nil {
			return err
		}
	}
	if err := screen.Update(); err != nil {
		return err
	}

	result := splashWait(timeoutSec)

	if pic != nil {
		_ = pic.Unload()
	}
	if titleTxt != nil {
		_ = titleTxt.Unload()
	}
	if subTxt != nil {
		_ = subTxt.Unload()
	}
	return result
}

// contrastWith returns white on dark backgrounds and black on light ones,
// using the standard luminance formula.
func contrastWith(c sdl.Color) sdl.Color {
	lum := 0.299*float32(c.R) + 0.587*float32(c.G) + 0.114*float32(c.B)
	if lum > 128 {
		return sdl.Color{R: 0, G: 0, B: 0, A: 255}
	}
	return sdl.Color{R: 255, G: 255, B: 255, A: 255}
}
