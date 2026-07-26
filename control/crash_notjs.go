//go:build !js

// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

// platformHandleCrash is a no-op on desktop: it returns false so an unrecovered
// panic re-propagates from Experiment.Run with its full stack trace, which is
// what a developer running the experiment in a terminal wants. The browser
// build overrides this in crash_js.go to show an error overlay instead.
func (e *Experiment) platformHandleCrash(r any) bool { return false }
