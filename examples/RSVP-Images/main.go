// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// RSVP-Images replicates the rapid-serial-visual-presentation oddball task of
// Hebart et al. (2023, THINGS-data). Object images are shown one after another
// on a mid-gray background with a central black fixation dot; the participant
// presses SPACE whenever an "oddball" appears. Oddballs are generated at
// runtime by pixelating a few of the natural images.
//
// Timings are NOT hardcoded: they are read from a design CSV passed on the
// command line, with columns
//
//	onset,duration,trial_type,file_path     (onset/duration in seconds)
//
// where trial_type is "exp" (natural image) or "catch" (pixelated oddball).
// Generate the MEG (SOA 1500±200 ms) and fMRI (SOA 4.5 s) designs with the
// companion tool in cmd/gen-design.
//
// The results CSV mirrors the input columns plus a reaction_time column (ms):
// each SPACE press is attributed to the image whose onset most recently
// preceded it; rows with no associated press get "n/a".
//
// Usage (from the repo root — run as a package, since this example has more
// than one .go file):
//
//	go run ./examples/RSVP-Images -w -s 1 examples/RSVP-Images/design_meg.csv
//
// Flags: -w windowed, -d N display index, -s N subject ID, -pixel N oddball
// block size, -maxpx N downscale cap, -dotsize R fixation-dot radius, -box N
// image bounding-box side. Press ESC (or close the window) to abort.
package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"image"
	_ "image/jpeg" // register JPEG decoder for image.Decode
	"image/png"
	"log"
	"os"
	"strconv"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// trial holds one row of the design CSV.
type trial struct {
	onsetSec    float64
	durationSec float64
	trialType   string
	filePath    string
}

func main() {
	pixelSize := flag.Int("pixel", 16, "block size for pixelating catch (oddball) images; larger = coarser")
	maxPx := flag.Int("maxpx", 900, "downscale every image so its largest side is at most this many pixels (caps GPU memory)")
	dotSize := flag.Float64("dotsize", 5, "radius of the central black fixation dot, in pixels")
	boxSize := flag.Float64("box", 600, "bounding-box side in px; images are scaled to fit, preserving aspect ratio")

	exp := control.NewExperimentFromFlags(
		"Hebart-RSVP",
		control.Gray, control.Black, 32,
	)
	defer exp.End()

	// The design CSV path is the first positional argument (after the flags).
	designPath := ""
	if args := flag.Args(); len(args) >= 1 {
		designPath = args[0]
	}
	if designPath == "" {
		log.Fatal("usage: RSVP-Images [-w -s N ...] <design.csv>\n" +
			"generate a design with cmd/gen-design")
	}

	trials, err := loadDesign(designPath)
	if err != nil {
		log.Fatalf("loading design %q: %v", designPath, err)
	}
	if len(trials) == 0 {
		log.Fatalf("design %q contains no trials", designPath)
	}

	// Build the stimuli. Every image is decoded, downscaled to cap GPU texture
	// memory, and (for catch trials) pixelated, then handed to the GPU as an
	// in-memory PNG — no oddball is ever written to disk.
	stims := make([]stimuli.VisualStimulus, len(trials))
	onsetMs := make([]int, len(trials))
	durationMs := make([]int, len(trials))
	for i, t := range trials {
		pic, err := buildPicture(t, *maxPx, *pixelSize)
		if err != nil {
			log.Fatalf("trial %d (%s): %v", i, t.filePath, err)
		}
		// Preload to populate native Width/Height, then scale to fit the box.
		if err := stimuli.PreloadVisualOnScreen(exp.Screen, pic); err != nil {
			log.Fatalf("trial %d (%s): preloading: %v", i, t.filePath, err)
		}
		scale := float32(*boxSize) / max(pic.Width, pic.Height)
		pic.Width *= scale
		pic.Height *= scale

		stims[i] = pic
		onsetMs[i] = int(t.onsetSec*1000 + 0.5)
		durationMs[i] = int(t.durationSec*1000 + 0.5)
	}

	elements, err := stimuli.MakeVisualStream(stims, onsetMs, durationMs)
	if err != nil {
		log.Fatalf("building stream: %v", err)
	}
	// Convert to the heterogeneous stream form so we can pass a per-frame
	// callback (to draw the fixation dot during the ISI).
	generic := make([]stimuli.StreamElement, len(elements))
	for i, el := range elements {
		generic[i] = stimuli.StreamElement{
			Stimulus:    el.Stimulus,
			DurationOn:  el.DurationOn,
			DurationOff: el.DurationOff,
		}
	}

	exp.AddDataVariableNames([]string{"onset", "duration", "trial_type", "file_path", "reaction_time"})

	exp.ShowInstructions(
		"Keep your eyes on the central dot.\n\n" +
			"A fast sequence of object pictures will be shown.\n" +
			"Press SPACE as fast as you can whenever a\n" +
			"PIXELATED (oddball) picture appears.\n\n" +
			"Press SPACE to begin (ESC to abort).")

	// The fixation dot is drawn on every frame so it is visible during the
	// fixation/ISI period (it also rides on top of each image, marking centre).
	dot := stimuli.NewCircle(float32(*dotSize), control.Black)
	drawDot := func(ctx stimuli.FrameContext) error {
		return dot.Draw(ctx.Screen)
	}

	events, timing, err := stimuli.PresentStreamOfStimuliFunc(exp.Screen, generic, 0, 0, drawDot)
	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("presenting stream: %v", err)
	}

	// Score responses: attribute each SPACE press to the image whose onset most
	// recently preceded it. First press per image wins; the rest stay "n/a".
	rt := make([]interface{}, len(trials))
	for i := range rt {
		rt[i] = "n/a"
	}
	for _, ev := range events {
		if ev.Event.Type != sdl.EVENT_KEY_DOWN {
			continue
		}
		if ev.Event.KeyboardEvent().Key != control.K_SPACE {
			continue
		}
		idx := mostRecentOnset(timing, ev.TimestampNS)
		if idx < 0 {
			continue // press before any image onset
		}
		if _, alreadyScored := rt[idx].(int64); alreadyScored {
			continue
		}
		rt[idx] = int64(ev.TimestampNS-timing[idx].OnsetNS) / 1_000_000
	}

	for i, t := range trials {
		exp.Data.Add(t.onsetSec, t.durationSec, t.trialType, t.filePath, rt[i])
	}

	nCatch := 0
	for _, t := range trials {
		if t.trialType == "catch" {
			nCatch++
		}
	}
	log.Printf("presented %d images (%d catch); responses scored to nearest preceding onset", len(trials), nCatch)
}

// loadDesign reads a design CSV with header onset,duration,trial_type,file_path.
func loadDesign(path string) ([]trial, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, nil // header only or empty
	}

	trials := make([]trial, 0, len(records)-1)
	for i, rec := range records[1:] { // skip header
		if len(rec) < 4 {
			return nil, &parseError{line: i + 2, msg: "expected 4 columns"}
		}
		onset, err := strconv.ParseFloat(rec[0], 64)
		if err != nil {
			return nil, &parseError{line: i + 2, msg: "bad onset: " + err.Error()}
		}
		duration, err := strconv.ParseFloat(rec[1], 64)
		if err != nil {
			return nil, &parseError{line: i + 2, msg: "bad duration: " + err.Error()}
		}
		trials = append(trials, trial{
			onsetSec:    onset,
			durationSec: duration,
			trialType:   rec[2],
			filePath:    rec[3],
		})
	}
	return trials, nil
}

// buildPicture decodes the trial's image, downscales it to maxPx, pixelates it
// when the trial is a catch (oddball), and returns an in-memory Picture.
func buildPicture(t trial, maxPx, pixelSize int) (*stimuli.Picture, error) {
	f, err := os.Open(t.filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	src, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}

	small := resizeFit(src, maxPx)
	if t.trialType != "exp" {
		small = pixelate(small, pixelSize)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, small); err != nil {
		return nil, err
	}
	return stimuli.NewPictureFromMemory(buf.Bytes(), 0, 0), nil
}

// mostRecentOnset returns the index of the image whose onset is the latest one
// at or before pressNS, or -1 if the press precedes every onset. Onsets are
// monotonically increasing, so a linear scan suffices.
func mostRecentOnset(timing []stimuli.TimingLog, pressNS uint64) int {
	idx := -1
	for i := range timing {
		if timing[i].OnsetNS != 0 && timing[i].OnsetNS <= pressNS {
			idx = i
		} else {
			break
		}
	}
	return idx
}

// parseError reports a problem in a specific design-CSV line.
type parseError struct {
	line int
	msg  string
}

func (e *parseError) Error() string {
	return "line " + strconv.Itoa(e.line) + ": " + e.msg
}
