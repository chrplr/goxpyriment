// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// TextInput represents a text input box for user input.
//
// Embeds BaseVisual for position management and lifecycle no-ops.
type TextInput struct {
	BaseVisual      // Position, GetPosition, SetPosition, Preload, Unload
	Message         string
	BoxWidth        float32
	BackgroundColor sdl.Color
	FrameColor      sdl.Color
	TextColor       sdl.Color
	UserText        string
}

// NewTextInput creates a new TextInput.
func NewTextInput(message string, position sdl.FPoint, boxWidth float32, bgColor, frameColor, textColor sdl.Color) *TextInput {
	return &TextInput{
		BaseVisual:      BaseVisual{Position: position},
		Message:         message,
		BoxWidth:        boxWidth,
		BackgroundColor: bgColor,
		FrameColor:      frameColor,
		TextColor:       textColor,
	}
}

// Get displays the input box and waits for user input.
func (ti *TextInput) Get(screen *apparatus.Screen, keyboard *apparatus.Keyboard) (string, error) {
	runes := []rune(ti.UserText)

	// Start SDL text input
	if err := screen.Window.StartTextInput(); err != nil {
		return "", err
	}
	defer screen.Window.StopTextInput()

	for {
		ti.UserText = string(runes)
		if err := ti.Present(screen, true, true); err != nil {
			return "", err
		}

		var event sdl.Event
		if sdl.WaitEvent(&event) == nil {
			switch event.Type {
			case sdl.EVENT_QUIT:
				return "", sdl.EndLoop
			case sdl.EVENT_KEY_DOWN:
				key := event.KeyboardEvent().Key
				if key == sdl.K_ESCAPE {
					return "", sdl.EndLoop
				}
				if key == sdl.K_RETURN || key == sdl.K_KP_ENTER {
					return string(runes), nil
				}
				if key == sdl.K_BACKSPACE {
					if len(runes) > 0 {
						runes = runes[:len(runes)-1]
					}
				}
			case sdl.EVENT_TEXT_INPUT:
				newText := event.TextInputEvent().Text
				runes = append(runes, []rune(newText)...)
			}
		}
	}
}

func (ti *TextInput) Draw(screen *apparatus.Screen) error {
	// Message
	msg := NewTextBox(ti.Message, int32(ti.BoxWidth), sdl.FPoint{X: ti.Position.X, Y: ti.Position.Y + 60}, ti.TextColor)
	if err := msg.Draw(screen); err != nil {
		return err
	}

	// Input Frame
	frame := NewRectangle(ti.Position.X, ti.Position.Y, ti.BoxWidth, 40, ti.FrameColor)
	if err := frame.Draw(screen); err != nil {
		return err
	}

	// Inner background (smaller than frame)
	inner := NewRectangle(ti.Position.X, ti.Position.Y, ti.BoxWidth-2, 38, ti.BackgroundColor)
	if err := inner.Draw(screen); err != nil {
		return err
	}

	// User Text
	user := NewTextLine(ti.UserText, ti.Position.X, ti.Position.Y, ti.TextColor)
	if err := user.Draw(screen); err != nil {
		return err
	}

	return nil
}

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (ti *TextInput) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(ti, screen, clear, update)
}

// Preload, Unload, GetPosition, SetPosition are all provided by BaseVisual.
