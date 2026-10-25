// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package results

import (
	"runtime/debug"
	"strings"
)

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

// ProgramVersion describes the experiment program itself, read from the
// binary's embedded build info: the path and version of its main module, and
// the git commit it was built from, e.g.
//
//	github.com/me/my-experiments v0.3.0 (commit 329e41c4b1f2, 2026-10-25T09:00:00Z)
//
// The version comes from the git tag of the commit (or is a pseudo-version
// derived from the last tag), with a +dirty suffix when the working tree had
// uncommitted changes; ", modified" says the same when the version itself is
// "(devel)". go build records the commit; go run does not, which leaves
// "<path> (devel)".
var ProgramVersion = func() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "(unknown)"
	}
	return programVersion(info)
}()

func programVersion(info *debug.BuildInfo) string {
	path, version := info.Main.Path, info.Main.Version
	if path == "" {
		path = info.Path
	}
	if path == "" {
		path = "(unknown)"
	}
	if version == "" {
		version = "(devel)"
	}
	var revision, vcsTime string
	modified := false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			vcsTime = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if revision == "" {
		return path + " " + version
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	details := []string{"commit " + revision}
	if vcsTime != "" {
		details = append(details, vcsTime)
	}
	if modified && !strings.HasSuffix(version, "+dirty") {
		details = append(details, "modified")
	}
	return path + " " + version + " (" + strings.Join(details, ", ") + ")"
}
