// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package apparatus

import (
	"bytes"
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

	// RIFF chunks are word-aligned: a chunk of odd size is followed by one pad
	// byte, which the parent's size counts and the chunk's own size does not.
	// Only 8-bit audio with an odd frame count can produce an odd data chunk.
	// Without the pad the cue and LIST chunks started one byte off, and editors
	// ignored or misread the markers.
	dataPad := dataSize % 2

	riffSize := 36 + dataSize + dataPad // 4("WAVE") + 24(fmt chunk) + 8+dataSize(data chunk)
	if len(cues) > 0 {
		riffSize += 8 + cueDataSize  // "cue " header(8) + data
		riffSize += 8 + listDataSize // "LIST" header(8) + data
	}

	// The header and the trailing chunks are assembled in memory (a
	// bytes.Buffer write cannot fail), so the file sees three writes, each of
	// which is checked.
	le := binary.LittleEndian
	var head bytes.Buffer
	head.WriteString("RIFF")
	head.Write(le.AppendUint32(nil, uint32(riffSize)))
	head.WriteString("WAVE")
	// fmt chunk
	head.WriteString("fmt ")
	head.Write(le.AppendUint32(nil, 16)) // chunk size
	head.Write(le.AppendUint16(nil, audioFmt))
	head.Write(le.AppendUint16(nil, uint16(channels)))
	head.Write(le.AppendUint32(nil, uint32(sampleRate)))
	head.Write(le.AppendUint32(nil, uint32(byteRate)))
	head.Write(le.AppendUint16(nil, uint16(blockAlign)))
	head.Write(le.AppendUint16(nil, uint16(bitsPerSample)))
	// data chunk header; the samples follow it directly
	head.WriteString("data")
	head.Write(le.AppendUint32(nil, uint32(dataSize)))

	var tail bytes.Buffer
	if dataPad != 0 {
		tail.WriteByte(0)
	}
	if len(cues) > 0 {
		// cue chunk — sample positions of named markers
		tail.WriteString("cue ")
		tail.Write(le.AppendUint32(nil, uint32(cueDataSize)))
		tail.Write(le.AppendUint32(nil, uint32(len(cues))))
		for i, c := range cues {
			tail.Write(le.AppendUint32(nil, uint32(i+1)))    // dwName — unique cue ID
			tail.Write(le.AppendUint32(nil, c.SampleOffset)) // dwPosition
			tail.WriteString("data")                         // fccChunk — references the data chunk
			tail.Write(le.AppendUint32(nil, 0))              // dwChunkStart
			tail.Write(le.AppendUint32(nil, 0))              // dwBlockStart
			tail.Write(le.AppendUint32(nil, c.SampleOffset)) // dwSampleOffset
		}
		// LIST/adtl chunk — human-readable label for each cue point
		tail.WriteString("LIST")
		tail.Write(le.AppendUint32(nil, uint32(listDataSize)))
		tail.WriteString("adtl")
		for _, l := range labls {
			tail.WriteString("labl")
			tail.Write(le.AppendUint32(nil, uint32(4+len(l.label)))) // ckSize = cueID(4) + label bytes
			tail.Write(le.AppendUint32(nil, l.id))
			tail.Write(l.label)
		}
	}

	f, err := os.CreateTemp(filepath.Dir(path), "goxpyriment-wav-*.wav")
	if err != nil {
		return fmt.Errorf("WriteWAV: create temp: %w", err)
	}
	tmpPath := f.Name()
	fail := func(stage string, err error) error {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("WriteWAV: %s: %w", stage, err)
	}

	for _, part := range [][]byte{head.Bytes(), data, tail.Bytes()} {
		if _, err := f.Write(part); err != nil {
			return fail("write", err)
		}
	}
	// A write that only fails when the data reaches the disk (disk full, a
	// network filesystem) is reported by Sync or Close. Both used to be
	// ignored, so a truncated recording was renamed into place and reported as
	// saved.
	if err := f.Sync(); err != nil {
		return fail("sync", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("WriteWAV: close: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
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
