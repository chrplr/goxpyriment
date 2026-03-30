// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

//go:build linux && arm64

package control

// This file is used on Linux/arm64 (e.g. Raspberry Pi), where go-sdl3 does
// not ship a pre-built embedded SDL3 binary. SDL3, SDL3_ttf, and SDL3_image
// must be installed via the system package manager:
//
//	sudo apt install libsdl3-dev libsdl3-ttf-dev libsdl3-image-dev
//
// On all other platforms, sdlload_embedded.go is used instead.

import (
	"log"

	"github.com/Zyko0/go-sdl3/img"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
)

type sysSDLLoader struct{}
func (sysSDLLoader) Unload() {
	if err := sdl.CloseLibrary(); err != nil {
		log.Printf("warning: sdl.CloseLibrary: %v", err)
	}
}

type sysTTFLoader struct{}
func (sysTTFLoader) Unload() {
	if err := ttf.CloseLibrary(); err != nil {
		log.Printf("warning: ttf.CloseLibrary: %v", err)
	}
}

type sysIMGLoader struct{}
func (sysIMGLoader) Unload() {
	if err := img.CloseLibrary(); err != nil {
		log.Printf("warning: img.CloseLibrary: %v", err)
	}
}

func loadSDL() interface{ Unload() } {
	if err := sdl.LoadLibrary(sdl.Path()); err != nil {
		log.Fatalf("SDL3 system library not found (%s): %v\n"+
			"Install with: sudo apt install libsdl3-dev", sdl.Path(), err)
	}
	return sysSDLLoader{}
}

func loadTTF() interface{ Unload() } {
	if err := ttf.LoadLibrary(ttf.Path()); err != nil {
		log.Fatalf("SDL3_ttf system library not found (%s): %v\n"+
			"Install with: sudo apt install libsdl3-ttf-dev", ttf.Path(), err)
	}
	return sysTTFLoader{}
}

func loadIMG() interface{ Unload() } {
	if err := img.LoadLibrary(img.Path()); err != nil {
		log.Fatalf("SDL3_image system library not found (%s): %v\n"+
			"Install with: sudo apt install libsdl3-image-dev", img.Path(), err)
	}
	return sysIMGLoader{}
}
