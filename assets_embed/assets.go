// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package assets_embed

import (
	_ "embed"
)

//go:embed Inconsolata.ttf
var InconsolataFont []byte

//go:embed buzzer.wav
var BuzzerWav []byte

//go:embed correct.wav
var CorrectWav []byte

// LogoPNG is the 256×256 goxpyriment mascot logo, suitable for use with
// stimuli.SplashScreen.
//
//go:embed logo.png
var LogoPNG []byte

// IconPNG is the goxpyriment icon displayed in the experiment splash screen.
//
// It is the same image as LogoPNG: logo.png and icon_256.png were byte-identical
// files, so only one is embedded now. Both names are kept because both are part
// of the public API. Treat the contents as read-only — the two variables share
// one backing array.
//
// A binary that references both variables shrinks by ~92 KB (measured); one that
// references only a single variable is unaffected, since the linker already
// dropped the unused embed.
var IconPNG = LogoPNG
