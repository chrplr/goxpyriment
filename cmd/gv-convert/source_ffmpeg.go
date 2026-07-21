// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// ffmpegSource decodes anything ffmpeg understands by piping raw RGBA frames
// out of a child process. ffmpeg is looked up on PATH and is not bundled: it
// is a convenience for formats the pure-Go MPEG-1 decoder cannot read.
type ffmpegSource struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
	buf    *bufio.Reader
	w, h   int
	fps    float64
	frame  int
}

// ffprobeInfo is the subset of ffprobe's JSON output we need.
type ffprobeInfo struct {
	Streams []struct {
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		CodecType  string `json:"codec_type"`
		RFrameRate string `json:"r_frame_rate"`
	} `json:"streams"`
}

// probeVideo asks ffprobe for the dimensions and frame rate of the first video
// stream. The dimensions must be known before decoding, because raw RGBA has
// no frame boundaries — we read exactly width*height*4 bytes per frame.
func probeVideo(path string) (w, h int, fps float64, err error) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return 0, 0, 0, fmt.Errorf("ffprobe not found on PATH (install ffmpeg, or convert to MPEG-1 first)")
	}

	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,codec_type,r_frame_rate",
		"-of", "json", path,
	).Output()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	var info ffprobeInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return 0, 0, 0, fmt.Errorf("parsing ffprobe output: %w", err)
	}
	for _, s := range info.Streams {
		if s.CodecType != "video" || s.Width == 0 || s.Height == 0 {
			continue
		}
		return s.Width, s.Height, parseFrameRate(s.RFrameRate), nil
	}
	return 0, 0, 0, fmt.Errorf("no video stream found in %s", path)
}

// parseFrameRate converts ffprobe's "30000/1001" rational form to a float.
// Returns 0 when unparseable, letting the caller fall back to a default.
func parseFrameRate(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	n, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0
	}
	if !ok {
		return n
	}
	d, err := strconv.ParseFloat(den, 64)
	if err != nil || d == 0 {
		return 0
	}
	return n / d
}

// openFFmpeg starts ffmpeg decoding path into raw RGBA on stdout. A non-zero
// fps resamples the output to that rate; 0 keeps the source rate.
func openFFmpeg(path string, fps float64) (*ffmpegSource, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("ffmpeg not found on PATH (install ffmpeg, or convert to MPEG-1 first)")
	}

	w, h, srcFPS, err := probeVideo(path)
	if err != nil {
		return nil, err
	}
	if fps <= 0 {
		fps = srcFPS
	}

	args := []string{"-v", "error", "-i", path}
	if fps > 0 {
		args = append(args, "-r", strconv.FormatFloat(fps, 'f', -1, 64))
	}
	args = append(args, "-f", "rawvideo", "-pix_fmt", "rgba", "-")

	cmd := exec.Command("ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	// Surface ffmpeg's diagnostics instead of swallowing them; -v error keeps
	// this quiet unless something is actually wrong.
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting ffmpeg: %w", err)
	}

	return &ffmpegSource{
		cmd:    cmd,
		stdout: stdout,
		// One frame of read-ahead; frames are large and read in full blocks.
		buf: bufio.NewReaderSize(stdout, w*h*4),
		w:   w,
		h:   h,
		fps: fps,
	}, nil
}

func (s *ffmpegSource) Size() (int, int) { return s.w, s.h }
func (s *ffmpegSource) FPS() float64     { return s.fps }
func (s *ffmpegSource) Describe() string { return "ffmpeg (external)" }

func (s *ffmpegSource) NextFrame() ([]byte, error) {
	frame := make([]byte, s.w*s.h*4)
	if _, err := io.ReadFull(s.buf, frame); err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		// A partial frame means ffmpeg died or the stream is truncated; either
		// way the output would be corrupt, so do not treat it as a clean end.
		if err == io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("truncated frame %d from ffmpeg", s.frame)
		}
		return nil, err
	}
	s.frame++
	return frame, nil
}

func (s *ffmpegSource) Close() error {
	s.stdout.Close()
	// The pipe is closed early when the caller stops reading, so ffmpeg exiting
	// non-zero here is expected and not worth reporting.
	_ = s.cmd.Wait()
	return nil
}
