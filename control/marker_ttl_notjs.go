// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build !js

package control

import (
	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/triggers"
)

// openTTLSpec opens the TTL device named by a -ttl spec. Desktop only: the
// triggers package drives serial and parallel ports and does not build for
// the browser (see marker_ttl_js.go).
func openTTLSpec(spec string) (apparatus.MarkerOutput, string, error) {
	dev, desc, err := triggers.Open(spec)
	if err != nil {
		return nil, "", err
	}
	return dev, desc, nil
}
