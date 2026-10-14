// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build js

package control

import (
	"errors"

	"github.com/chrplr/goxpyriment/apparatus"
)

// openTTLSpec — browser build. A page cannot reach a serial or parallel port,
// and the triggers package does not build for GOOS=js, so asking for a TTL
// device is an error rather than a silent no-op: a run that believes it is
// marking a recording but is not would produce unusable data. The photodiode
// square works in the browser.
func openTTLSpec(spec string) (apparatus.MarkerOutput, string, error) {
	return nil, "", errors.New("hardware triggers are not available in the browser build")
}
