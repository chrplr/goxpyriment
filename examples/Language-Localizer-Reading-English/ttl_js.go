// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build js

package main

import "errors"

// openTTL — browser build. The triggers package is desktop-only (it talks to
// serial ports), so a browser run can never drive a DLP-IO8-G. Asking for one
// is an error rather than a silent no-op: an EEG/MEG run without its triggers
// is worthless, and it must not look like it worked.
func openTTL(spec string) (ttlDevice, string, error) {
	if spec != "" {
		return nullTTL{}, "", errors.New("-dlpio8: hardware triggers are not available in the browser build")
	}
	return nullTTL{}, "", nil
}
