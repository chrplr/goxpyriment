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
