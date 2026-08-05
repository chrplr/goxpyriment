// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

//go:build js

package apparatus

// glRendererString is not available in the browser: purego does not apply on
// js, and the WebGL context is owned by Emscripten rather than reachable
// through SDL_GL_GetProcAddress. The desktop implementation lives in
// glrenderer_notjs.go.
func glRendererString() string { return "" }
