// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package misc

import "time"

// Wait blocks for the given number of milliseconds.
func Wait(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

// GetTime returns the current time in milliseconds since the program started (approximately).
var startTime = time.Now()

func GetTime() int64 {
	return time.Since(startTime).Milliseconds()
}
