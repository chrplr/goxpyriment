// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package apparatus

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zyko0/go-sdl3/sdl"
)

// walkRIFF checks that the chunk sizes tile the file exactly, with the pad
// byte after an odd chunk, and returns the chunk IDs in order.
func walkRIFF(t *testing.T, b []byte) []string {
	t.Helper()
	if len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		t.Fatalf("not a RIFF/WAVE file")
	}
	if got, want := int(binary.LittleEndian.Uint32(b[4:8])), len(b)-8; got != want {
		t.Errorf("RIFF size = %d, want %d (file length - 8)", got, want)
	}
	var ids []string
	for off := 12; off < len(b); {
		if off+8 > len(b) {
			t.Fatalf("truncated chunk header at %d", off)
		}
		id := string(b[off : off+4])
		size := int(binary.LittleEndian.Uint32(b[off+4 : off+8]))
		ids = append(ids, id)
		off += 8 + size + size%2
		if off > len(b) {
			t.Fatalf("chunk %q at overruns the file", id)
		}
	}
	return ids
}

func TestWriteWAVOddDataIsPaddedBeforeCues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "odd.wav")
	spec := sdl.AudioSpec{Format: sdl.AUDIO_U8, Channels: 1, Freq: 8000}
	data := []byte{1, 2, 3} // odd length: only 8-bit audio can do this
	if err := WriteWAV(path, spec, data, WAVCue{SampleOffset: 1, Label: "onset"}); err != nil {
		t.Fatalf("WriteWAV: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ids := walkRIFF(t, b)
	if want := []string{"fmt ", "data", "cue ", "LIST"}; len(ids) != len(want) ||
		ids[0] != want[0] || ids[1] != want[1] || ids[2] != want[2] || ids[3] != want[3] {
		t.Errorf("chunks = %q, want %q", ids, want)
	}
	// The data chunk's own size is not padded.
	if got := binary.LittleEndian.Uint32(b[40:44]); got != 3 {
		t.Errorf("data chunk size = %d, want 3", got)
	}
}

func TestWriteWAVEvenDataNoCues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "even.wav")
	spec := sdl.AudioSpec{Format: sdl.AUDIO_S16LE, Channels: 2, Freq: 44100}
	if err := WriteWAV(path, spec, make([]byte, 400)); err != nil {
		t.Fatalf("WriteWAV: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 44+400 {
		t.Errorf("file is %d bytes, want %d", len(b), 44+400)
	}
	walkRIFF(t, b)
}

// A failed write must not leave a file at the target path, nor a temp file.
func TestWriteWAVFailureLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "missing-subdir", "x.wav")
	if err := WriteWAV(bad, sdl.AudioSpec{Format: sdl.AUDIO_S16LE, Channels: 1, Freq: 8000}, make([]byte, 4)); err == nil {
		t.Fatal("WriteWAV into a missing directory succeeded")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("left %d entries behind", len(entries))
	}
}
