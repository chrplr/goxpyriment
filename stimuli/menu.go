// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

// menu.go — keyboard-navigable numbered menu widget.
//
// Menu presents a vertical list of labelled items and lets the participant
// select one by pressing the matching number key (1–9, 0 for tenth) or by
// navigating with the UP/DOWN arrow keys and confirming with ENTER or SPACE.
// The currently highlighted item is rendered in a distinct color and prefixed
// with ">". All other items use the normal text color and a blank prefix so
// that text remains horizontally aligned.

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
	"github.com/chrplr/goxpyriment/apparatus"
)

// Menu is a keyboard-navigable numbered list widget.
//
// Construct with NewMenu, optionally adjust public fields, then call Get.
//
// Example:
//
//	m := stimuli.NewMenu([]string{"Easy", "Medium", "Hard"})
//	idx, err := m.Get(exp.Screen, exp.Keyboard, 0)
//	// idx is 0-based: 0=Easy, 1=Medium, 2=Hard
type Menu struct {
	// Items are the labels shown in the list.
	Items []string

	// Pos is the center of the menu block in center-based screen coordinates.
	// Default (0, 0) = screen center.
	Pos sdl.FPoint

	// Font to use for all items. nil = use the screen's default font.
	Font *ttf.Font

	// TextColor is the color for unselected items.
	TextColor sdl.Color

	// HighlightColor is the color for the currently selected item.
	HighlightColor sdl.Color

	// LineSpacing is the vertical distance (pixels) between consecutive item
	// centers. 0 = auto: 1.6 × the font's line height.
	LineSpacing float32

	// Caption is optional text drawn above the list (supports newlines).
	Caption string
}

// NewMenu creates a Menu with sensible visual defaults.
func NewMenu(items []string) *Menu {
	return &Menu{
		Items:          items,
		TextColor:      sdl.Color{R: 200, G: 200, B: 200, A: 255},
		HighlightColor: sdl.Color{R: 255, G: 220, B: 0, A: 255},
		LineSpacing:    0,
	}
}

// lineSpacing returns the effective vertical spacing between item centers.
func (m *Menu) lineSpacing(screen *apparatus.Screen) float32 {
	if m.LineSpacing > 0 {
		return m.LineSpacing
	}
	f := m.Font
	if f == nil {
		f = screen.DefaultFont
	}
	if f != nil {
		return float32(f.Height()) * 1.6
	}
	return 36 // safe fallback
}

// draw renders all items, highlighting the one at index sel.
func (m *Menu) draw(screen *apparatus.Screen, sel int) error {
	if m.Caption != "" {
		w := int32(float32(screen.Width) * 0.85)
		if w < 400 {
			w = 400
		}
		captionY := float32(screen.Height) * 0.32
		tb := NewTextBox(m.Caption, w, sdl.FPoint{X: m.Pos.X, Y: captionY}, m.TextColor)
		tb.Font = m.Font
		if err := tb.Draw(screen); err != nil {
			return err
		}
	}

	n := len(m.Items)
	ls := m.lineSpacing(screen)
	totalH := float32(n-1) * ls

	// Resolve the font once.
	font := m.Font
	if font == nil {
		font = screen.DefaultFont
	}

	// Measure each item's text so we can left-align them.
	// All items use the same prefix width, so measuring with the highlight
	// prefix ("> N. text") is correct — it is the widest of the two variants.
	var maxW float32
	widths := make([]float32, n)
	if font != nil {
		for i, item := range m.Items {
			numKey := i + 1
			if numKey == 10 {
				numKey = 0
			}
			text := fmt.Sprintf("> %d. %s", numKey, item)
			w, _, err := font.StringSize(text)
			if err == nil {
				widths[i] = float32(w)
				if float32(w) > maxW {
					maxW = float32(w)
				}
			}
		}
	}

	// Left edge of the block: items' left edges all align here.
	// If measurement failed (no font), fall back to centering at Pos.X.
	leftX := m.Pos.X - maxW/2

	for i, item := range m.Items {
		// Y: item 0 at the top (highest Y), item n-1 at the bottom.
		y := m.Pos.Y + totalH/2 - float32(i)*ls

		numKey := i + 1
		if numKey == 10 {
			numKey = 0
		}

		var text string
		var color sdl.Color
		if i == sel {
			text = fmt.Sprintf("> %d. %s", numKey, item)
			color = m.HighlightColor
		} else {
			text = fmt.Sprintf("  %d. %s", numKey, item)
			color = m.TextColor
		}

		// Center the TextLine so its left edge sits at leftX.
		var cx float32
		if maxW > 0 {
			cx = leftX + widths[i]/2
		} else {
			cx = m.Pos.X
		}

		line := NewTextLine(text, cx, y, color)
		line.Font = m.Font
		if err := line.Draw(screen); err != nil {
			return err
		}
	}
	return nil
}

// Get displays the menu and blocks until the participant makes a selection.
//
// initialSel is the 0-based index of the item that is highlighted on entry.
// Clipped to [0, len(Items)-1].
//
// Navigation:
//   - UP / DOWN arrows move the highlight.
//   - ENTER or SPACE confirms the current highlight.
//   - Number key 1–9 (or 0 for the tenth item) selects and confirms directly.
//   - ESC or window-close returns (-1, sdl.EndLoop).
//
// Returns the 0-based index of the selected item and nil on success.
func (m *Menu) Get(screen *apparatus.Screen, kb *apparatus.Keyboard, initialSel int) (int, error) {
	n := len(m.Items)
	if n == 0 {
		return -1, nil
	}

	sel := initialSel
	if sel < 0 {
		sel = 0
	}
	if sel >= n {
		sel = n - 1
	}

	for {
		// Render current state.
		if err := screen.Clear(); err != nil {
			return -1, err
		}
		if err := m.draw(screen, sel); err != nil {
			return -1, err
		}
		if err := screen.Update(); err != nil {
			return -1, err
		}

		// Block until the next input event.
		var ev sdl.Event
		if sdl.WaitEvent(&ev) != nil {
			continue
		}

		switch ev.Type {
		case sdl.EVENT_QUIT:
			return -1, sdl.EndLoop

		case sdl.EVENT_KEY_DOWN:
			key := ev.KeyboardEvent().Key
			switch {
			case key == sdl.K_ESCAPE:
				return -1, sdl.EndLoop

			case key == sdl.K_UP:
				sel = (sel - 1 + n) % n

			case key == sdl.K_DOWN:
				sel = (sel + 1) % n

			case key == sdl.K_RETURN || key == sdl.K_KP_ENTER || key == sdl.K_SPACE:
				return sel, nil

			default:
				// Digit keys 1–9 select items 0–8 directly; 0 selects item 9.
				var idx int
				var matched bool
				if key >= sdl.K_1 && key <= sdl.K_9 {
					idx = int(key-sdl.K_1)
					matched = true
				} else if key == sdl.K_0 && n >= 10 {
					idx = 9
					matched = true
				}
				if matched && idx < n {
					return idx, nil
				}
			}
		}
	}
}
