// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

// choicegrid.go — visual multiple-choice keyboard response widget.
//
// ChoiceGrid presents a grid of labelled buttons and collects a sequence of
// responses. Each button can be activated by clicking it with the mouse or by
// pressing the matching key (works for single-character labels such as letters
// and digits). BACKSPACE removes the last entry. ENTER or SPACE submits the
// response (only when MaxSelect == 0). If MaxSelect > 0, the response is
// submitted automatically once that many items have been selected.
//
// The Sperling paradigm is the canonical use case: present the 21-consonant
// palette, collect 3 letters for partial report (MaxSelect=3, auto-submit) or
// up to 9 letters for whole report (MaxSelect=0, explicit ENTER).

import (
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// ChoiceGrid is a visual multiple-choice response widget.
//
// Example:
//
//	cg := stimuli.NewChoiceGrid([]string{"B","C","D","F","G"}, 3, "Pick 3 letters:")
//	selections, err := cg.Get(exp.Screen, exp.Keyboard)
type ChoiceGrid struct {
	// Choices are the button labels (e.g. letters, digits, short words).
	Choices []string
	// MaxSelect is the number of items to collect before auto-submitting.
	// 0 means collect until the participant presses ENTER or SPACE.
	MaxSelect int
	// Prompt is displayed above the grid as an instruction line.
	Prompt string

	// Layout parameters — NewChoiceGrid sets sensible defaults.
	ButtonW float32 // button width in pixels
	ButtonH float32 // button height in pixels
	Margin  float32 // gap between buttons in pixels
	Cols    int     // columns; 0 = auto (roughly square grid, max 7)

	// Colors
	ButtonColor   sdl.Color // button fill
	TextColor     sdl.Color // button label text
	ResponseColor sdl.Color // response line and prompt text
}

// cgButton holds the pre-computed geometry for a single button.
type cgButton struct {
	label  string
	cx, cy float32  // center in screen-center (center-origin) coordinates
	bounds sdl.FRect // in SDL top-left coordinates, used for mouse hit-testing
}

// NewChoiceGrid creates a ChoiceGrid with sensible visual defaults.
//
//   - choices: the button labels (typically uppercase letters or digits).
//   - maxSelect: 0 = participant presses ENTER/SPACE to submit; >0 = auto-submit
//     after that many selections.
//   - prompt: instruction line shown at the top of the screen.
func NewChoiceGrid(choices []string, maxSelect int, prompt string) *ChoiceGrid {
	return &ChoiceGrid{
		Choices:       choices,
		MaxSelect:     maxSelect,
		Prompt:        prompt,
		ButtonW:       80,
		ButtonH:       50,
		Margin:        15,
		Cols:          0,
		ButtonColor:   sdl.Color{R: 200, G: 200, B: 200, A: 255},
		TextColor:     sdl.Color{R: 0, G: 0, B: 0, A: 255},
		ResponseColor: sdl.Color{R: 255, G: 255, B: 255, A: 255},
	}
}

// numCols returns the effective column count.
func (cg *ChoiceGrid) numCols() int {
	if cg.Cols > 0 {
		return cg.Cols
	}
	n := len(cg.Choices)
	cols := int(math.Ceil(math.Sqrt(float64(n))))
	if cols < 1 {
		cols = 1
	}
	if cols > 7 {
		cols = 7
	}
	return cols
}

// buildButtons computes the center position and SDL hit-test bounds for every
// button. Call once per Get invocation (the screen size may have changed).
func (cg *ChoiceGrid) buildButtons(screen *apparatus.Screen) []cgButton {
	cols := cg.numCols()
	n := len(cg.Choices)
	rows := (n + cols - 1) / cols

	w, h, m := cg.ButtonW, cg.ButtonH, cg.Margin
	totalW := float32(cols)*w + float32(cols-1)*m
	totalH := float32(rows)*h + float32(rows-1)*m

	// Place the grid slightly above the screen centre so there is room for
	// the response line and hint below.
	gridCenterY := float32(30)
	startX := -totalW/2 + w/2
	startY := gridCenterY + totalH/2 - h/2

	buttons := make([]cgButton, n)
	for i, label := range cg.Choices {
		r := i / cols
		c := i % cols
		cx := startX + float32(c)*(w+m)
		cy := startY - float32(r)*(h+m)
		sdlX, sdlY := screen.CenterToSDL(cx, cy)
		buttons[i] = cgButton{
			label: label,
			cx:    cx,
			cy:    cy,
			bounds: sdl.FRect{
				X: sdlX - w/2,
				Y: sdlY - h/2,
				W: w,
				H: h,
			},
		}
	}
	return buttons
}

// draw renders the prompt, all buttons, the current response, and a hint.
func (cg *ChoiceGrid) draw(screen *apparatus.Screen, buttons []cgButton, response []string) error {
	H := float32(screen.Height)

	// Prompt near the top of the screen.
	if cg.Prompt != "" {
		p := NewTextLine(cg.Prompt, 0, H/2-60, cg.ResponseColor)
		if err := p.Draw(screen); err != nil {
			return err
		}
	}

	// Buttons.
	for _, b := range buttons {
		rect := NewRectangle(b.cx, b.cy, cg.ButtonW, cg.ButtonH, cg.ButtonColor)
		if err := rect.Draw(screen); err != nil {
			return err
		}
		lbl := NewTextLine(b.label, b.cx, b.cy, cg.TextColor)
		if err := lbl.Draw(screen); err != nil {
			return err
		}
	}

	// Response line.
	respStr := "Response: " + strings.Join(response, " ")
	if len(response) == 0 {
		respStr = "Response: —"
	}
	resp := NewTextLine(respStr, 0, -H/2+100, cg.ResponseColor)
	if err := resp.Draw(screen); err != nil {
		return err
	}

	// Hint line.
	var hint string
	if cg.MaxSelect == 0 {
		hint = "Click or press keys · BACKSPACE to undo · ENTER to confirm"
	} else {
		remaining := cg.MaxSelect - len(response)
		if remaining > 0 {
			hint = fmt.Sprintf("Select %d more  ·  BACKSPACE to undo", remaining)
		}
	}
	if hint != "" {
		h := NewTextLine(hint, 0, -H/2+55, cg.ResponseColor)
		if err := h.Draw(screen); err != nil {
			return err
		}
	}

	return nil
}

// keycodeMatchesLabel returns true when the SDL keycode maps to the label.
// Works for single-character labels (A–Z, 0–9). SDL3 keycodes for letters
// are their lowercase ASCII values (e.g. sdl.K_A == 'a').
func keycodeMatchesLabel(key sdl.Keycode, label string) bool {
	if len(label) != 1 {
		return false
	}
	ch := unicode.ToLower(rune(label[0]))
	return key == sdl.Keycode(ch)
}

// Get displays the choice grid and blocks until the participant submits a
// response.
//
// The response is submitted:
//   - automatically when MaxSelect items have been selected (MaxSelect > 0), or
//   - when the participant presses ENTER or SPACE (MaxSelect == 0).
//
// BACKSPACE removes the last selected item. ESC or window-close returns
// (nil, sdl.EndLoop) so the caller can abort cleanly.
//
// The returned slice preserves selection order. Items may appear more than
// once if the participant selects the same button multiple times.
func (cg *ChoiceGrid) Get(screen *apparatus.Screen, kb *apparatus.Keyboard) ([]string, error) {
	buttons := cg.buildButtons(screen)
	response := []string{}

	for {
		// Render.
		if err := screen.Clear(); err != nil {
			return nil, err
		}
		if err := cg.draw(screen, buttons, response); err != nil {
			return nil, err
		}
		if err := screen.Update(); err != nil {
			return nil, err
		}

		// Block until the next SDL event (no busy-spin needed — there is
		// nothing to animate while waiting for the participant's input).
		var ev sdl.Event
		if sdl.WaitEvent(&ev) != nil {
			continue // SDL error; just try again
		}

		switch ev.Type {
		case sdl.EVENT_QUIT:
			return nil, sdl.EndLoop

		case sdl.EVENT_KEY_DOWN:
			key := ev.KeyboardEvent().Key
			switch {
			case key == sdl.K_ESCAPE:
				return nil, sdl.EndLoop
			case key == sdl.K_BACKSPACE && len(response) > 0:
				response = response[:len(response)-1]
			case (key == sdl.K_RETURN || key == sdl.K_KP_ENTER || key == sdl.K_SPACE) && cg.MaxSelect == 0:
				return response, nil
			default:
				// Match by keyboard key.
				for _, b := range buttons {
					if keycodeMatchesLabel(key, b.label) {
						response = append(response, b.label)
						break
					}
				}
			}

		case sdl.EVENT_MOUSE_BUTTON_DOWN:
			mev := ev.MouseButtonEvent()
			// Convert window-pixel coordinates to the logical renderer space
			// so they match the bounds computed via CenterToSDL.
			lx, ly, err := screen.Renderer.RenderCoordinatesFromWindow(mev.X, mev.Y)
			if err != nil {
				lx, ly = mev.X, mev.Y
			}
			for _, b := range buttons {
				if lx >= b.bounds.X && lx <= b.bounds.X+b.bounds.W &&
					ly >= b.bounds.Y && ly <= b.bounds.Y+b.bounds.H {
					response = append(response, b.label)
					break
				}
			}
		}

		// Auto-submit once MaxSelect items have been collected.
		if cg.MaxSelect > 0 && len(response) >= cg.MaxSelect {
			// One final render to show the complete response before returning.
			screen.Clear()         //nolint:errcheck
			cg.draw(screen, buttons, response) //nolint:errcheck
			screen.Update()        //nolint:errcheck
			return response, nil
		}
	}
}
