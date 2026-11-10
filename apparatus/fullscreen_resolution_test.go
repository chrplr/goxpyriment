// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build !js

package apparatus

import (
	"strings"
	"testing"

	"github.com/Zyko0/go-sdl3/sdl"
)

// offscreenMode starts SDL's offscreen video driver, with the software
// renderer (NewScreen's Vulkan hint has normal priority, so the environment
// wins), and returns the current mode of its display. It skips where SDL
// cannot run.
func offscreenMode(t *testing.T) *sdl.DisplayMode {
	t.Helper()
	t.Setenv("SDL_VIDEO_DRIVER", "offscreen")
	t.Setenv("SDL_RENDER_DRIVER", "software")
	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		t.Skipf("SDL offscreen video unavailable: %v", err)
	}
	t.Cleanup(sdl.Quit)
	cur, err := sdl.GetPrimaryDisplay().CurrentDisplayMode()
	if err != nil || cur == nil {
		t.Skipf("the offscreen display has no current mode: %v", err)
	}
	return cur
}

// withResolution stages a fullscreen policy and resolution for one test.
func withResolution(t *testing.T, p FullscreenPolicy, w, h int) {
	t.Helper()
	oldP, oldW, oldH := fullscreenPolicy, fullscreenResolution.w, fullscreenResolution.h
	SetFullscreenPolicy(p)
	SetFullscreenResolution(w, h)
	t.Cleanup(func() {
		SetFullscreenPolicy(oldP)
		SetFullscreenResolution(oldW, oldH)
	})
}

// A requested resolution makes the automatic policy exclusive, whatever the
// platform would pick otherwise: a mode can only be set there.
func TestFullscreenResolutionMakesAutoExclusive(t *testing.T) {
	withResolution(t, FullscreenAuto, 1920, 1080)
	if got := FullscreenPolicyInEffect(); got != FullscreenExclusive {
		t.Errorf("auto with a resolution: %s, want exclusive", got)
	}
	withResolution(t, FullscreenDesktop, 1920, 1080)
	if got := FullscreenPolicyInEffect(); got != FullscreenDesktop {
		t.Errorf("desktop with a resolution: %s, want desktop (NewScreen then refuses)", got)
	}
}

// A size the display has no mode for is refused, and so is a resolution under
// fullscreen-desktop, which cannot set one: neither silently opens at the
// current mode. That a mode the display has is set and shown can only be
// checked on a real display: the offscreen driver lists no fullscreen mode.
func TestFullscreenResolutionRefusesWhatItCannotSet(t *testing.T) {
	m := offscreenMode(t)
	cases := []struct {
		name string
		p    FullscreenPolicy
		w, h int
		want string
	}{
		{"no such mode", FullscreenAuto, int(m.W) + 7, int(m.H) + 3, "has no"},
		{"desktop policy", FullscreenDesktop, int(m.W), int(m.H), "needs exclusive fullscreen"},
	}
	for _, c := range cases {
		withResolution(t, c.p, c.w, c.h)
		s, err := NewScreen("resolution-test", 0, 0, sdl.Color{A: 255}, true, 0)
		if err == nil {
			s.Renderer.Destroy()
			s.Window.Destroy()
			t.Errorf("%s: NewScreen opened %dx%d, want an error", c.name, s.Width, s.Height)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q, want it to contain %q", c.name, err, c.want)
		}
	}
}
