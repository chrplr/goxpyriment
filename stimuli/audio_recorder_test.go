// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFloat32WAV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.wav")
	// 2 frames mono float32 = 8 bytes
	pcm := []byte{0, 0, 0, 0, 0, 0, 128, 63} // 0.0 and 1.0 LE
	if err := WriteFloat32WAV(path, pcm, 48000, 1); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 44+8 {
		t.Fatalf("wav size: got %d want %d", len(b), 44+8)
	}
	if string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		t.Fatalf("bad header %q", b[0:12])
	}
}
