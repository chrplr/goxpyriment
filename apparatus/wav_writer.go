// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package apparatus

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/Zyko0/go-sdl3/sdl"
)

// WriteWAV saves raw PCM audio data to a WAV file.
//
// spec must describe the format of data: Format (sdl.AUDIO_F32LE or sdl.AUDIO_S16LE
// are the most common), Channels, and Freq (sample rate in Hz).
//
// The file is written atomically using a temporary path; if writing succeeds the
// temp file is renamed to path.
func WriteWAV(path string, spec sdl.AudioSpec, data []byte) error {
	bitsPerSample, audioFmt, err := wavFormat(spec.Format)
	if err != nil {
		return fmt.Errorf("WriteWAV: %w", err)
	}

	channels := int(spec.Channels)
	sampleRate := int(spec.Freq)
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataSize := len(data)
	riffSize := 36 + dataSize // 4 (WAVE) + 24 (fmt chunk) + 8 (data header)

	f, err := os.CreateTemp("", "goxpyriment-wav-*.wav")
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
