package results

import (
	"runtime/debug"
	"testing"
)

func TestProgramVersion(t *testing.T) {
	vcs := func(modified string) []debug.BuildSetting {
		return []debug.BuildSetting{
			{Key: "vcs", Value: "git"},
			{Key: "vcs.revision", Value: "329e41c4b1f2a8d0e5c6b7a8f9e0d1c2b3a4f5e6"},
			{Key: "vcs.time", Value: "2026-10-25T09:00:00Z"},
			{Key: "vcs.modified", Value: modified},
		}
	}
	for _, tc := range []struct {
		name string
		info debug.BuildInfo
		want string
	}{
		{"tagged", debug.BuildInfo{Main: debug.Module{Path: "example.org/exp", Version: "v0.3.0"}, Settings: vcs("false")},
			"example.org/exp v0.3.0 (commit 329e41c4b1f2, 2026-10-25T09:00:00Z)"},
		// +dirty already says that the tree was modified
		{"pseudo-version, dirty", debug.BuildInfo{Main: debug.Module{Path: "example.org/exp", Version: "v0.3.1-0.20261025090000-329e41c4b1f2+dirty"}, Settings: vcs("true")},
			"example.org/exp v0.3.1-0.20261025090000-329e41c4b1f2+dirty (commit 329e41c4b1f2, 2026-10-25T09:00:00Z)"},
		// a module in a subfolder of the repository, without matching tags
		{"devel, modified", debug.BuildInfo{Main: debug.Module{Path: "example.org/exp", Version: "(devel)"}, Settings: vcs("true")},
			"example.org/exp (devel) (commit 329e41c4b1f2, 2026-10-25T09:00:00Z, modified)"},
		// go run: no VCS information
		{"go run", debug.BuildInfo{Main: debug.Module{Path: "example.org/exp", Version: "(devel)"}},
			"example.org/exp (devel)"},
		{"no module", debug.BuildInfo{Path: "command-line-arguments"},
			"command-line-arguments (devel)"},
		{"nothing", debug.BuildInfo{}, "(unknown) (devel)"},
	} {
		if got := programVersion(&tc.info); got != tc.want {
			t.Errorf("%s: programVersion = %q; want %q", tc.name, got, tc.want)
		}
	}
}
