// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
package media

import (
	"errors"
	"fmt"
	"io"

	"github.com/funatsufumiya/go-gv-video/gvvideo"
	"github.com/pierrec/lz4/v4"
)

// gvMaxCompressedSize returns the maximum LZ4-compressed frame byte count
// across all frames in gv. Use the result to pre-allocate a reusable scratch
// buffer for gvReadFrameInto, eliminating the per-frame heap allocation that
// gvvideo.ReadFrameCompressedTo performs internally.
func gvMaxCompressedSize(gv *gvvideo.GVVideo) uint64 {
	var max uint64
	for _, b := range gv.AddressSizeBlocks {
		if b.Size > max {
			max = b.Size
		}
	}
	return max
}

// gvReadFrameInto decompresses frame frameID from gv into decompressed, using
// compressed as a scratch buffer for the LZ4-compressed bytes. Neither buffer
// is ever reallocated. compressed must be >= gvMaxCompressedSize(gv) bytes;
// decompressed must be >= Width*Height*4 bytes (i.e. gv.Header.FrameBytes).
func gvReadFrameInto(gv *gvvideo.GVVideo, frameID uint32, compressed, decompressed []byte) error {
	if frameID >= gv.Header.FrameCount {
		return errors.New("end of video")
	}
	block := gv.AddressSizeBlocks[frameID]
	if _, err := gv.Reader.Seek(int64(block.Address), io.SeekStart); err != nil {
		return fmt.Errorf("gvReadFrameInto: seeking to frame %d: %w", frameID, err)
	}
	if uint64(len(compressed)) < block.Size {
		return errors.New("gvReadFrameInto: compressed buffer too small")
	}
	if _, err := io.ReadFull(gv.Reader, compressed[:block.Size]); err != nil {
		return fmt.Errorf("gvReadFrameInto: reading frame %d: %w", frameID, err)
	}
	uncompressedSize := int(gv.Header.Width) * int(gv.Header.Height) * 4
	if len(decompressed) < uncompressedSize {
		return errors.New("gvReadFrameInto: decompressed buffer too small")
	}
	_, err := lz4.UncompressBlock(compressed[:block.Size], decompressed[:uncompressedSize])
	return err
}
