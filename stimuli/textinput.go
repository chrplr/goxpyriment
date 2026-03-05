// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"goxpyriment/io"
	"github.com/Zyko0/go-sdl3/sdl"
)

// TextInput represents a text input box for user input.
type TextInput struct {
	Message         string
	Position        sdl.FPoint
	BoxWidth        float32
	BackgroundColor sdl.Color
	FrameColor      sdl.Color
	TextColor       sdl.Color
	UserText        string
}

// NewTextInput creates a new TextInput.
func NewTextInput(message string, position sdl.FPoint, boxWidth float32, bgColor, frameColor, textColor sdl.Color) *TextInput {
	return &TextInput{
		Message:         message,
		Position:        position,
		BoxWidth:        boxWidth,
		BackgroundColor: bgColor,
		FrameColor:      frameColor,
		TextColor:       textColor,
	}
}

// Get displays the input box and waits for user input.
func (ti *TextInput) Get(screen *io.Screen, keyboard *io.Keyboard) (string, error) {
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

func (ti *TextInput) Draw(screen *io.Screen) error {
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

func (ti *TextInput) Present(screen *io.Screen, clear, update bool) error {
	if clear {
		if err := screen.Clear(); err != nil {
			return err
		}
	}
	if err := ti.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

func (ti *TextInput) Preload() error { return nil }
func (ti *TextInput) Unload() error  { return nil }

func (ti *TextInput) GetPosition() sdl.FPoint {
	return ti.Position
}

func (ti *TextInput) SetPosition(pos sdl.FPoint) {
	ti.Position = pos
}
