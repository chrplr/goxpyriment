// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

// SDL3, SDL3_ttf, and SDL3_image are loaded from precompiled binaries bundled
// inside go-sdl3. This works on Linux/amd64, Linux/arm64, macOS, and Windows.
// The library is extracted to a temp directory at runtime and loaded via dlopen.

import (
	"github.com/Zyko0/go-sdl3/bin/binimg"
	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/bin/binttf"
)

func loadSDL() interface{ Unload() } { return binsdl.Load() }
func loadTTF() interface{ Unload() } { return binttf.Load() }
func loadIMG() interface{ Unload() } { return binimg.Load() }
