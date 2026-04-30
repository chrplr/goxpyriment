// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package media

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseTimeSpec parses a PsyScope-style time string and returns the total
// duration. The format is one or more comma-separated components of the
// form "<unit>:<value>", where unit is one of:
//
//	ms : milliseconds
//	s  : seconds
//	m  : minutes
//	h  : hours
//
// Whitespace inside the spec is ignored. Empty input is invalid.
//
//	ParseTimeSpec("ms:500")        // 500 ms
//	ParseTimeSpec("s:3,ms:100")    // 3.1 s
//	ParseTimeSpec("h:0,m:1,s:30")  // 1 m 30 s
//
// This helper exists for users translating PsyScope scripts; the native
// API uses time.Duration directly.
func ParseTimeSpec(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("media.ParseTimeSpec: empty spec")
	}
	var total time.Duration
	for _, raw := range strings.Split(s, ",") {
		part := strings.TrimSpace(raw)
		if part == "" {
			return 0, fmt.Errorf("media.ParseTimeSpec: empty component in %q", s)
		}
		colon := strings.IndexByte(part, ':')
		if colon < 0 {
			return 0, fmt.Errorf("media.ParseTimeSpec: missing ':' in component %q", part)
		}
		unit := strings.TrimSpace(part[:colon])
		valueStr := strings.TrimSpace(part[colon+1:])
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return 0, fmt.Errorf("media.ParseTimeSpec: bad number in %q: %w", part, err)
		}
		var multiplier time.Duration
		switch unit {
		case "ms":
			multiplier = time.Millisecond
		case "s":
			multiplier = time.Second
		case "m":
			multiplier = time.Minute
		case "h":
			multiplier = time.Hour
		default:
			return 0, fmt.Errorf("media.ParseTimeSpec: unknown unit %q in %q", unit, s)
		}
		total += time.Duration(value * float64(multiplier))
	}
	return total, nil
}

// ParseFrameSpec parses a PsyScope-style frame string of the form
// "f:<n>" or "frame:<n>" and returns the frame number. Frames are
// 1-based; the value 0 (PsyScope's "before the first frame") is allowed.
//
//	ParseFrameSpec("f:186")     // 186
//	ParseFrameSpec("frame:1")   // 1
//	ParseFrameSpec("f:0")       // 0
func ParseFrameSpec(s string) (int, error) {
	s = strings.TrimSpace(s)
	colon := strings.IndexByte(s, ':')
	if colon < 0 {
		return 0, fmt.Errorf("media.ParseFrameSpec: missing ':' in %q", s)
	}
	unit := strings.TrimSpace(s[:colon])
	if unit != "f" && unit != "frame" {
		return 0, fmt.Errorf("media.ParseFrameSpec: unknown unit %q in %q (want 'f' or 'frame')", unit, s)
	}
	n, err := strconv.Atoi(strings.TrimSpace(s[colon+1:]))
	if err != nil {
		return 0, fmt.Errorf("media.ParseFrameSpec: bad number in %q: %w", s, err)
	}
	if n < 0 {
		return 0, fmt.Errorf("media.ParseFrameSpec: negative frame %d", n)
	}
	return n, nil
}
