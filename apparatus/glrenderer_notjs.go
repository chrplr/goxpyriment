// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build !js

package apparatus

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/ebitengine/purego"
)

// glRENDERER is OpenGL's GL_RENDERER enum: the string identifying the GPU and
// driver actually executing the rendering.
const glRENDERER = 0x1F01

// glRendererString returns OpenGL's GL_RENDERER for the current context, e.g.
// "NVIDIA RTX 2000 Ada Generation Laptop GPU/PCIe/SSE2" or "Mesa Intel(R)
// Graphics (MTL)". It returns "" when the answer is not available rather than
// guessing.
//
// This exists because SystemInfo.RendererName is SDL's *backend* name — it says
// "opengl", never which GPU ran it. On a hybrid laptop that distinction matters:
// which of the two GPUs renders depends on PRIME offload environment variables,
// the compositor, and the session type, and it is not otherwise recorded
// anywhere in the data file.
//
// SDL3 exposes no property carrying this, so it is read from OpenGL directly:
// SDL_GL_GetProcAddress resolves glGetString, and purego calls it. purego is
// pure Go (no CGo), and converts the returned null-terminated char* into a Go
// string itself, so no unsafe pointer arithmetic is needed here.
//
// Returns "" when there is no current GL context — which is the normal case for
// the Vulkan and software renderers, not an error.
func glRendererString() (name string) {
	// purego panics on an unsupported signature or a bad function pointer.
	// This is best-effort diagnostic metadata: a failure here must never take
	// down an experiment that is otherwise fine.
	defer func() {
		if recover() != nil {
			name = ""
		}
	}()

	// glGetString requires a current context; without one it returns NULL and
	// some drivers crash instead. The renderer only holds a GL context when the
	// GL backend is in use.
	if ctx, err := sdl.GL_GetCurrentContext(); err != nil || ctx == nil {
		return ""
	}
	proc := sdl.GL_GetProcAddress("glGetString")
	if proc == 0 {
		return ""
	}
	var glGetString func(name uint32) string
	purego.RegisterFunc(&glGetString, uintptr(proc))
	return glGetString(glRENDERER)
}
