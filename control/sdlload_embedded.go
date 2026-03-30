// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

//go:build !linux || !arm64

package control

// This file is used on platforms where go-sdl3 ships embedded SDL3 binaries
// (Linux/amd64, macOS, Windows). It extracts the bundled library to a temp
// directory and loads it via sdl.LoadLibrary.
//
// On Linux/arm64 (e.g. Raspberry Pi), sdlload_system.go is used instead,
// loading SDL3, SDL3_ttf, and SDL3_image from the system package manager.

import (
	"github.com/Zyko0/go-sdl3/bin/binimg"
	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/bin/binttf"
)

func loadSDL() interface{ Unload() } { return binsdl.Load() }
func loadTTF() interface{ Unload() } { return binttf.Load() }
func loadIMG() interface{ Unload() } { return binimg.Load() }
