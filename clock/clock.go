// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

// Package clock provides timing helpers: Wait, GetTime, and a Clock for
// measuring durations and sleeping until a target offset.
package clock

import "time"

// Wait blocks for the given number of milliseconds.
func Wait(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

var startTime = time.Now()

// GetTime returns the time in milliseconds since the program started (relative to first use of this package).
func GetTime() int64 {
	return time.Since(startTime).Milliseconds()
}

// GetTimeNS returns nanoseconds elapsed since the package's shared zero point.
//
// NOTE: uses time.Since() (Go monotonic clock), not sdl.TicksNS().
// These two clocks have different origins; do not subtract GetTimeNS() from
// SDL event timestamps (Screen.FlipNS, WaitKeysEventRT) to compute reaction
// times. Use the SDL-based functions for RT measurement.
func GetTimeNS() int64 {
	return time.Since(startTime).Nanoseconds()
}

// Clock provides a simple timing abstraction relative to a start reference.
// It can be used to measure durations and to sleep until a target offset.
type Clock struct {
	start time.Time
}

// NewClock creates a new Clock whose zero time reference is "now".
func NewClock() *Clock {
	return &Clock{start: time.Now()}
}

// Reset restarts the clock's zero reference to the current time.
func (c *Clock) Reset() {
	c.start = time.Now()
}

// Now returns the time elapsed since the clock's start reference.
func (c *Clock) Now() time.Duration {
	return time.Since(c.start)
}

// NowMillis returns the elapsed time in milliseconds since the clock's start reference.
func (c *Clock) NowMillis() int64 {
	return c.Now().Milliseconds()
}

// NowNanos returns nanoseconds elapsed since the clock's start reference.
//
// NOTE: uses time.Since() (Go monotonic clock), not sdl.TicksNS().
// Do not subtract NowNanos() from SDL event timestamps to compute reaction
// times. Use Screen.FlipNS + Keyboard.WaitKeysEventRT for that purpose.
func (c *Clock) NowNanos() int64 {
	return c.Now().Nanoseconds()
}

// Sleep sleeps for the given duration.
func (c *Clock) Sleep(d time.Duration) {
	time.Sleep(d)
}

// SleepUntil sleeps until the given target offset since the clock's start
// reference has been reached or passed. If the target time is already in the
// past, it returns immediately.
func (c *Clock) SleepUntil(target time.Duration) {
	for {
		now := c.Now()
		if now >= target {
			return
		}
		remaining := target - now
		// Sleep for the remaining duration; OS scheduling will determine
		// the exact wake-up time.
		time.Sleep(remaining)
	}
}
