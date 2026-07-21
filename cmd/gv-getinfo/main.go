// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Command gv-getinfo prints the header and index-table summary of a .gv video
// file: frame count, frame size, frame rate, pixel format, and how well the
// frames compressed.
//
// It also validates what it reads — a .gv file whose header disagrees with its
// index table will load but play incorrectly, and that is hard to diagnose from
// a black screen.
//
// Usage:
//
//	gv-getinfo [options] <file.gv> [file.gv ...]
//
// Options:
//
//	-frames   list every frame's offset and compressed size
//	-q        print one summary line per file (for scripting)
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/funatsufumiya/go-gv-video/gvvideo"
)

// gvHeaderBytes is the fixed size of the .gv header: six 4-byte fields.
const gvHeaderBytes = 24

// indexEntryBytes is the size of one index-table entry: two uint64s.
const indexEntryBytes = 16

func main() {
	frames := flag.Bool("frames", false, "list every frame's offset and compressed size")
	quiet := flag.Bool("q", false, "print one summary line per file")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <file.gv> [file.gv ...]\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "Prints frame count, frame size, frame rate and compression stats\n")
		fmt.Fprintf(os.Stderr, "for .gv video files.\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}

	exit := 0
	for i, path := range flag.Args() {
		if !*quiet && i > 0 {
			fmt.Println()
		}
		if err := report(path, *frames, *quiet); err != nil {
			fmt.Fprintf(os.Stderr, "gv-getinfo: %s: %v\n", path, err)
			exit = 1
		}
	}
	os.Exit(exit)
}

// formatName describes the Format field. Only raw RGBA is playable by
// goxpyriment: stimuli/gvvideo_buf.go LZ4-decompresses straight into a texture
// and never DXT-decodes.
func formatName(f uint32) (name string, playable bool) {
	switch f {
	case 0:
		return "raw RGBA", true
	case gvvideo.GVFormatDXT1:
		return "DXT1", false
	case gvvideo.GVFormatDXT3:
		return "DXT3", false
	case gvvideo.GVFormatDXT5:
		return "DXT5", false
	case 7:
		return "BC7", false
	default:
		return fmt.Sprintf("unknown (%d)", f), false
	}
}

func report(path string, listFrames, quiet bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return err
	}
	fileSize := st.Size()

	// Sanity-check the header before letting the loader act on it: it locates
	// the index table by seeking FrameCount*16 back from EOF, so a non-.gv file
	// fails there with an opaque "invalid argument" instead of saying what is
	// actually wrong.
	if err := plausibleHeader(f, fileSize); err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	v, err := gvvideo.LoadGVVideoFromReader(f)
	if err != nil {
		return err
	}
	h := v.Header

	format, playable := formatName(h.Format)
	rawFrame := uint64(h.Width) * uint64(h.Height) * 4
	rawTotal := rawFrame * uint64(h.FrameCount)

	var compressed uint64
	sizes := make([]uint64, 0, len(v.AddressSizeBlocks))
	for _, b := range v.AddressSizeBlocks {
		compressed += b.Size
		sizes = append(sizes, b.Size)
	}

	var duration float64
	if h.FPS > 0 {
		duration = float64(h.FrameCount) / float64(h.FPS)
	}

	if quiet {
		fmt.Printf("%s\t%dx%d\t%d frames\t%.3f fps\t%s\t%s\n",
			path, h.Width, h.Height, h.FrameCount, h.FPS, format, humanBytes(uint64(fileSize)))
		return nil
	}

	fmt.Printf("file        : %s\n", path)
	fmt.Printf("frame size  : %d x %d px  (%s raw per frame)\n", h.Width, h.Height, humanBytes(rawFrame))
	fmt.Printf("frame count : %d\n", h.FrameCount)
	fmt.Printf("frame rate  : %.3f fps", h.FPS)
	if duration > 0 {
		fmt.Printf("   (duration %.2f s)", duration)
	}
	fmt.Println()
	fmt.Printf("pixel format: %s", format)
	if !playable {
		fmt.Printf("   [!] goxpyriment can only play raw RGBA")
	}
	fmt.Println()

	fmt.Printf("file size   : %s\n", humanBytes(uint64(fileSize)))
	if rawTotal > 0 {
		fmt.Printf("frame data  : %s compressed from %s  (%.1fx, %.1f%% of raw)\n",
			humanBytes(compressed), humanBytes(rawTotal),
			float64(rawTotal)/float64(compressed),
			100*float64(compressed)/float64(rawTotal))
	}
	if len(sizes) > 0 {
		sorted := append([]uint64(nil), sizes...)
		slices.Sort(sorted)
		fmt.Printf("frame sizes : min %s, median %s, max %s\n",
			humanBytes(sorted[0]),
			humanBytes(sorted[len(sorted)/2]),
			humanBytes(sorted[len(sorted)-1]))
	}

	for _, w := range validate(h, v.AddressSizeBlocks, fileSize, rawFrame) {
		fmt.Printf("warning     : %s\n", w)
	}

	if listFrames {
		fmt.Println()
		fmt.Printf("%-8s %14s %12s %10s\n", "frame", "offset", "bytes", "of raw")
		for i, b := range v.AddressSizeBlocks {
			pct := 0.0
			if rawFrame > 0 {
				pct = 100 * float64(b.Size) / float64(rawFrame)
			}
			fmt.Printf("%-8d %14d %12d %9.1f%%\n", i, b.Address, b.Size, pct)
		}
	}
	return nil
}

// plausibleHeader reads the 24-byte header and rejects values that cannot
// describe a real .gv file. .gv has no magic number, so this is the only way to
// tell "not a .gv file" from "a corrupt .gv file".
func plausibleHeader(f *os.File, fileSize int64) error {
	if fileSize < gvHeaderBytes {
		return fmt.Errorf("file is %d bytes, too small to be a .gv file", fileSize)
	}
	h, err := gvvideo.ReadHeader(f)
	if err != nil {
		return fmt.Errorf("reading header: %w", err)
	}

	const maxDim = 65536 // far beyond any real display
	switch {
	case h.Width == 0 || h.Height == 0:
		return fmt.Errorf("not a .gv file (header says %dx%d)", h.Width, h.Height)
	case h.Width > maxDim || h.Height > maxDim:
		return fmt.Errorf("not a .gv file (header says %dx%d)", h.Width, h.Height)
	case h.FrameCount == 0:
		return fmt.Errorf("not a .gv file (header says 0 frames)")
	}

	// The index table must fit between the header and EOF.
	if need := gvHeaderBytes + int64(h.FrameCount)*indexEntryBytes; need > fileSize {
		return fmt.Errorf("not a .gv file (header claims %d frames, needing %d bytes, but the file is %d)",
			h.FrameCount, need, fileSize)
	}
	return nil
}

// validate cross-checks the header against the index table. These are the
// inconsistencies that produce a file which loads but plays wrongly.
func validate(h gvvideo.GVHeader, blocks []gvvideo.GVAddressSizeBlock, fileSize int64, rawFrame uint64) []string {
	var warns []string

	if uint64(h.FrameBytes) != rawFrame {
		warns = append(warns, fmt.Sprintf(
			"header FrameBytes is %d but width*height*4 is %d", h.FrameBytes, rawFrame))
	}
	if h.FPS <= 0 {
		warns = append(warns, fmt.Sprintf("frame rate is %v; players will not know the cadence", h.FPS))
	}
	if h.FrameCount == 0 {
		warns = append(warns, "frame count is 0")
		return warns
	}

	// The index table sits at the very end; its size is implied by FrameCount.
	wantIndex := int64(h.FrameCount) * indexEntryBytes
	if fileSize < gvHeaderBytes+wantIndex {
		warns = append(warns, fmt.Sprintf(
			"file is %d bytes, too small for a %d-frame index table", fileSize, h.FrameCount))
		return warns
	}
	indexStart := fileSize - wantIndex

	var prevEnd uint64 = gvHeaderBytes
	for i, b := range blocks {
		if b.Size == 0 {
			warns = append(warns, fmt.Sprintf("frame %d has zero length", i))
			continue
		}
		if b.Address < gvHeaderBytes {
			warns = append(warns, fmt.Sprintf("frame %d starts at %d, inside the header", i, b.Address))
		}
		end := b.Address + b.Size
		if end > uint64(indexStart) {
			warns = append(warns, fmt.Sprintf(
				"frame %d ends at %d, past the start of the index table (%d)", i, end, indexStart))
		}
		if b.Address < prevEnd {
			warns = append(warns, fmt.Sprintf(
				"frame %d starts at %d, overlapping the previous frame (ends %d)", i, b.Address, prevEnd))
		}
		prevEnd = end
		// One report of each kind is enough to make the point.
		if len(warns) > 6 {
			warns = append(warns, "further index problems suppressed")
			break
		}
	}
	return warns
}

// humanBytes formats a byte count in the largest unit that keeps it readable.
func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for n/div >= unit && exp < 3 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}
