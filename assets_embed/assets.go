// Copyright (2026) Christophe Pallier <christophe@pallier.org>
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
