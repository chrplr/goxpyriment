// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

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

// LogoPNG is the goxpyriment mascot logo, suitable for use with stimuli.SplashScreen.
//
//go:embed logo.png
var LogoPNG []byte

// IconPNG is the 256×256 goxpyriment icon, displayed in the experiment splash screen.
//
//go:embed icon_256.png
var IconPNG []byte
