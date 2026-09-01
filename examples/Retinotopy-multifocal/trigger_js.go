// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build js

package main

import "fmt"

// Browser build: there is no serial or parallel port, and triggers/ does not
// compile for GOOS=js, so this file stands in for trigger_desktop.go. The
// browser version of this example is a demonstration of the stimulus, not a
// recording session.

// TriggerDeviceNames lists the accepted values of -trigger.
const TriggerDeviceNames = "none (hardware triggers are unavailable in the browser)"

// openTrigger accepts only "none" in the browser. Anything else is an error
// rather than a silent no-op: a run that believes it is marking a recording
// but is not would produce unusable data.
func openTrigger(name, device string, line, pulseMs int) (fire func(), closeFn func(), err error) {
	noop := func() {}
	switch name {
	case "", "none":
		return noop, noop, nil
	default:
		return nil, nil, fmt.Errorf("-trigger=%q: hardware triggers are not available in the browser", name)
	}
}
