// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package results

import "runtime/debug"

// Version is the goxpyriment module version, read from the binary's embedded
// build info. It reflects the git tag when the module is built as a versioned
// dependency (e.g. via go install or go get). Returns "(devel)" when built
// directly from source via go.work.
var Version = func() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "(devel)"
	}
	// When goxpyriment is the main module (go run / go.work during development).
	if info.Main.Path == "github.com/chrplr/goxpyriment" && info.Main.Version != "" {
		return info.Main.Version
	}
	// When goxpyriment is a versioned dependency of the experiment binary.
	for _, dep := range info.Deps {
		if dep.Path == "github.com/chrplr/goxpyriment" {
			return dep.Version
		}
	}
	return "(devel)"
}()
