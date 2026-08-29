//go:build !js

// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package results

import "fmt"

// Finalize writes out both the CSV file and the companion info file at the end
// of a session. On desktop it is equivalent to Save.
func (df *DataFile) Finalize() error {
	if err := df.InfoFile.Finalize(); err != nil {
		return fmt.Errorf("results.DataFile.Finalize: info file: %w", err)
	}
	return df.OutputFile.Finalize()
}
