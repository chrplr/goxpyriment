// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package apparatus

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Zyko0/go-sdl3/sdl"
)

// WAVCue marks a named position within a WAV file. Audio editors that support
// the WAV cue chunk (Audacity, Praat, Adobe Audition, …) display these as
// labelled markers on the waveform.
type WAVCue struct {
	SampleOffset uint32 // zero-based sample index of the marker
	Label        string // human-readable name shown in the audio editor
}

// WriteWAV saves raw PCM audio data to a WAV file.
//
// spec must describe the format of data: Format (sdl.AUDIO_F32LE or sdl.AUDIO_S16LE
// are the most common), Channels, and Freq (sample rate in Hz).
//
// Optional cues are written as a WAV cue chunk followed by an adtl label list,
// so that audio editors display them as named markers on the waveform.
//
// The file is written atomically using a temporary path; if writing succeeds the
// temp file is renamed to path.
func WriteWAV(path string, spec sdl.AudioSpec, data []byte, cues ...WAVCue) error {
	bitsPerSample, audioFmt, err := wavFormat(spec.Format)
	if err != nil {
		return fmt.Errorf("WriteWAV: %w", err)
	}

	channels := int(spec.Channels)
	sampleRate := int(spec.Freq)
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataSize := len(data)

	// Pre-compute sizes for the optional cue and LIST/adtl chunks so we can
	// write the correct RIFF size in the header before the chunks themselves.
	//
	// cue chunk:  "cue "(4) + ckSize(4) + numCuePoints(4) + n×cueRecord(24)
	// LIST chunk: "LIST"(4) + ckSize(4) + "adtl"(4) + n×("labl"(4)+ckSize(4)+cueID(4)+paddedLabel)
	type lablEntry struct {
		id    uint32
		label []byte // null-terminated, padded to even length
	}
	var labls []lablEntry
	cueDataSize := 0
	listDataSize := 0
	if len(cues) > 0 {
		cueDataSize = 4 + 24*len(cues) // numCuePoints(4) + n×24
		listDataSize = 4               // "adtl"
		for i, c := range cues {
			lb := append([]byte(c.Label), 0) // null-terminated
			if len(lb)%2 != 0 {
				lb = append(lb, 0) // pad to even — WAV chunks must be word-aligned
			}
			labls = append(labls, lablEntry{id: uint32(i + 1), label: lb})
			listDataSize += 8 + 4 + len(lb) // "labl"(4) + ckSize(4) + cueID(4) + label
		}
	}

	riffSize := 36 + dataSize // 4("WAVE") + 24(fmt chunk) + 8+dataSize(data chunk)
	if len(cues) > 0 {
		riffSize += 8 + cueDataSize  // "cue " header(8) + data
		riffSize += 8 + listDataSize // "LIST" header(8) + data
	}

	f, err := os.CreateTemp(filepath.Dir(path), "goxpyriment-wav-*.wav")
	if err != nil {
		return fmt.Errorf("WriteWAV: create temp: %w", err)
	}
	tmpPath := f.Name()

	write := func(v any) {
		if err != nil {
			return
		}
		err = binary.Write(f, binary.LittleEndian, v)
	}

	// RIFF header
	f.WriteString("RIFF")
	write(uint32(riffSize))
	f.WriteString("WAVE")

	// fmt chunk
	f.WriteString("fmt ")
	write(uint32(16)) // chunk size
	write(uint16(audioFmt))
	write(uint16(channels))
	write(uint32(sampleRate))
	write(uint32(byteRate))
	write(uint16(blockAlign))
	write(uint16(bitsPerSample))

	// data chunk
	f.WriteString("data")
	write(uint32(dataSize))
	_, writeErr := f.Write(data)
	if err == nil {
		err = writeErr
	}

	// cue chunk — sample positions of named markers
	if len(cues) > 0 {
		f.WriteString("cue ")
		write(uint32(cueDataSize))
		write(uint32(len(cues)))
		for i, c := range cues {
			write(uint32(i + 1))          // dwName — unique cue ID
			write(uint32(c.SampleOffset)) // dwPosition
			f.WriteString("data")         // fccChunk — references the data chunk
			write(uint32(0))              // dwChunkStart
			write(uint32(0))              // dwBlockStart
			write(uint32(c.SampleOffset)) // dwSampleOffset
		}

		// LIST/adtl chunk — human-readable label for each cue point
		f.WriteString("LIST")
		write(uint32(listDataSize))
		f.WriteString("adtl")
		for _, le := range labls {
			f.WriteString("labl")
			write(uint32(4 + len(le.label))) // ckSize = cueID(4) + label bytes
			write(le.id)
			_, wErr := f.Write(le.label)
			if err == nil {
				err = wErr
			}
		}
	}

	f.Close()

	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("WriteWAV: write: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("WriteWAV: rename: %w", err)
	}
	return nil
}

// wavFormat returns (bitsPerSample, wavAudioFormat, error) for a given SDL AudioFormat.
// WAV audio format codes: 1 = PCM, 3 = IEEE float.
func wavFormat(format sdl.AudioFormat) (int, uint16, error) {
	switch format {
	case sdl.AUDIO_F32LE:
		return 32, 3, nil
	case sdl.AUDIO_S16LE:
		return 16, 1, nil
	case sdl.AUDIO_S32LE:
		return 32, 1, nil
	case sdl.AUDIO_U8:
		return 8, 1, nil
	case sdl.AUDIO_S8:
		return 8, 1, nil
	default:
		return 0, 0, fmt.Errorf("unsupported SDL audio format 0x%04X for WAV export", uint32(format))
	}
}
