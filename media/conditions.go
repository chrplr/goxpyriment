// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package media

import "time"

// Target identifies what a Movie condition is waiting for. The set is
// sealed: the only constructors are Frame, AtTime, and Done.
//
// Frame(N) targets a specific cumulative 1-based frame index of the
// movie (across loops). Frame(0) fires before any frame has been decoded,
// matching PsyScope's f:0 semantics.
//
// AtTime(d) targets a specific elapsed media time within the movie
// (post-rate-scaling, cumulative across loops). Fires once per
// registration when the movie's effective media time first reaches d.
//
// Done targets end-of-file: the moment after the final frame of the
// final configured loop has been decoded.
type Target interface {
	// targetSentinel is unexported to seal the interface.
	targetSentinel()
}

// Frame is a cumulative 1-based frame target.
type Frame int

// AtTime is an elapsed-media-time target.
type AtTime time.Duration

// Done is the end-of-file target. Use the zero value Done{}.
type Done struct{}

func (Frame) targetSentinel()  {}
func (AtTime) targetSentinel() {}
func (Done) targetSentinel()   {}

// timeCondition is the internal record stored on each Movie. It is
// evaluated either inside the per-frame decode loop (look-ahead) or
// inside NotifyFlipped (display-deferred), depending on deferToDisplay.
type timeCondition struct {
	target         Target
	fn             func(Onset)
	deferToDisplay bool
	fired          bool
}

// matches reports whether the condition should fire for the given
// movie state. decodedFrame is the latest cumulative totalFramesDecoded
// (for look-ahead). displayedFrame is the cumulative currentFrameIndex
// (for display-deferred). mediaTime is the effective elapsed media time
// at this moment. done indicates the movie has reached end-of-file.
func (c *timeCondition) matches(decodedFrame, displayedFrame int, mediaTime time.Duration, done bool) bool {
	if c.fired {
		return false
	}
	switch t := c.target.(type) {
	case Frame:
		if c.deferToDisplay {
			return displayedFrame >= int(t)
		}
		return decodedFrame >= int(t)
	case AtTime:
		return mediaTime >= time.Duration(t)
	case Done:
		return done
	}
	return false
}
