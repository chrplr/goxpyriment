// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package media

import (
	"sync"
	"time"
)

// MasterClock is the shared monotonic media clock used by every Movie in
// a MovieManager. It mirrors the role of CMTimebase in PsyScope Tahoe
// (PSMovieManager.swift): all decoders compute their playback position
// from the same clock reading inside one draw cycle, eliminating
// cross-movie drift by design.
//
// MasterClock supports a freeze/burst mode. Between BeginBurst and
// EndBurst, every call to Now returns the value that was current at
// BeginBurst time. This guarantees that a sequence of script-style
// commands like
//
//	mgr.BeginBurst()
//	movieA.Pause()
//	movieB.Pause()
//	movieC.Pause()
//	mgr.EndBurst()
//
// pauses all three movies at the exact same media time, instead of
// three values microseconds apart. MovieManager.DrawWithoutFlip wraps
// its per-frame body in an internal burst so all movies on a frame
// share one Now value even when no explicit burst is active.
//
// Bursts nest: each BeginBurst increments a freeze counter, each
// EndBurst decrements it; the clock thaws when the counter reaches zero.
// Calling EndBurst more times than BeginBurst is a no-op.
type MasterClock struct {
	mu          sync.Mutex
	epoch       time.Time
	frozenAt    time.Duration
	freezeDepth int
}

// NewMasterClock creates a MasterClock starting at zero now.
func NewMasterClock() *MasterClock {
	return &MasterClock{epoch: time.Now()}
}

// Now returns the elapsed time since the last Reset (or construction),
// or the frozen value when inside a burst.
func (c *MasterClock) Now() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.freezeDepth > 0 {
		return c.frozenAt
	}
	return time.Since(c.epoch)
}

// Reset rewinds the clock to zero. When called inside a burst it also
// resets the frozen value, so Now returns 0 until EndBurst.
func (c *MasterClock) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epoch = time.Now()
	c.frozenAt = 0
}

// BeginBurst freezes Now at its current value. Bursts nest.
func (c *MasterClock) BeginBurst() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.freezeDepth == 0 {
		c.frozenAt = time.Since(c.epoch)
	}
	c.freezeDepth++
}

// EndBurst decrements the burst depth. The clock thaws when the depth
// reaches zero.
func (c *MasterClock) EndBurst() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.freezeDepth == 0 {
		return
	}
	c.freezeDepth--
}

// Frozen reports whether the clock is currently inside a burst.
func (c *MasterClock) Frozen() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.freezeDepth > 0
}
