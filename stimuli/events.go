// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
package stimuli

import "github.com/Zyko0/go-sdl3/sdl"

// FirstKeyPress returns the first KEY_DOWN event matching key from a stream's
// event list, along with a found flag. Both Timestamp (Go clock) and
// TimestampNS (SDL3 hardware) are available on the returned UserEvent.
func FirstKeyPress(events []UserEvent, key sdl.Keycode) (UserEvent, bool) {
	for _, ev := range events {
		if ev.Event.Type == sdl.EVENT_KEY_DOWN &&
			ev.Event.KeyboardEvent().Key == key {
			return ev, true
		}
	}
	return UserEvent{}, false
}
