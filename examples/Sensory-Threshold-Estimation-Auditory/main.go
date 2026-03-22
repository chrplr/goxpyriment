// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
// Adaptive Auditory Threshold Estimation
//
// Measures pure-tone hearing thresholds using a 1-up/2-down staircase with
// a 2-Interval Forced Choice (2-IFC) paradigm. Staircases for all tested
// frequencies are interleaved — the frequency is chosen at random on each
// trial to prevent pitch anticipation.
//
// Staircase rule (1-up / 2-down):
//   - 1 miss  → increase intensity by one step  (louder)
//   - 2 consecutive hits → decrease by one step (quieter)
//   Phase 1 (first 2 reversals): 4 dB steps
//   Phase 2 (after 2 reversals): 2 dB steps
//   Termination: 8 reversals per frequency
//   Threshold: mean intensity at the last 4 reversals
//
// Usage:
//
//	go run main.go [-s <id>] [-freqs 250,500,1000,2000,4000] [-d]
//
// Flags:
//
//	-s      int     Subject ID (default 0).
//	-freqs  string  Comma-separated list of frequencies in Hz
//	                (default "50,250,500,1000,2000,4000,8000").
//	-start  float   Starting level in dBFS, e.g. -20 (default -20).
//	-d              Development mode: windowed 1024×768.
//
// AUDIO SAFETY: Start at a comfortable system volume. The initial level is
// -20 dBFS (10 % of digital full-scale). Raise the volume only if tones are
// inaudible at that level.

package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
	"strings"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/staircase"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ─── Tone generation ────────────────────────────────────────────────────────

// generateToneData builds a mono 32-bit float PCM buffer for a pure sine tone
// with linear fade-in and fade-out ramps to avoid clicks.
func generateToneData(freqHz, dBFS float64, durationMs, rampMs int) []byte {
	const sampleRate = 44100
	nSamples := sampleRate * durationMs / 1000
	rampN := sampleRate * rampMs / 1000
	amplitude := float32(math.Pow(10.0, dBFS/20.0))

	data := make([]byte, nSamples*4)
	for i := 0; i < nSamples; i++ {
		v := float32(math.Sin(2*math.Pi*freqHz*float64(i)/sampleRate)) * amplitude
		// Fade-in.
		if i < rampN {
			v *= float32(i) / float32(rampN)
		}
		// Fade-out.
		if i >= nSamples-rampN {
			v *= float32(nSamples-1-i) / float32(rampN)
		}
		bits := math.Float32bits(v)
		data[i*4+0] = byte(bits)
		data[i*4+1] = byte(bits >> 8)
		data[i*4+2] = byte(bits >> 16)
		data[i*4+3] = byte(bits >> 24)
	}
	return data
}

// ─── Tone player ────────────────────────────────────────────────────────────

// tonePlayer wraps a single SDL audio stream that is reused across trials.
type tonePlayer struct {
	stream *sdl.AudioStream
}

func newTonePlayer(device sdl.AudioDeviceID) (*tonePlayer, error) {
	spec := &sdl.AudioSpec{
		Format:   sdl.AUDIO_F32LE,
		Channels: 1,
		Freq:     44100,
	}
	stream, err := sdl.CreateAudioStream(spec, spec)
	if err != nil {
		return nil, fmt.Errorf("create audio stream: %w", err)
	}
	if err := device.BindAudioStream(stream); err != nil {
		stream.Destroy()
		return nil, fmt.Errorf("bind audio stream: %w", err)
	}
	return &tonePlayer{stream: stream}, nil
}

// play queues a 500 ms tone with 50 ms ramps at the given frequency and level.
func (p *tonePlayer) play(freqHz, dBFS float64) error {
	data := generateToneData(freqHz, dBFS, 500, 50)
	_ = p.stream.Clear()
	return p.stream.PutData(data)
}

// stop drains any queued audio immediately (use after clock.Wait).
func (p *tonePlayer) stop() { _ = p.stream.Clear() }

func (p *tonePlayer) destroy() { p.stream.Destroy() }

// ─── Visual helpers ─────────────────────────────────────────────────────────

// drawIFCScreen renders the 2-IFC interval display.
// active: 0 = neither, 1 = interval 1 highlighted, 2 = interval 2 highlighted.
func drawIFCScreen(exp *control.Experiment, active int, info string) error {
	exp.Screen.Clear()

	// Info bar.
	infoLine := stimuli.NewTextLine(info, 0, 270, control.Black)
	if err := infoLine.Draw(exp.Screen); err != nil {
		return err
	}

	// Box colours: active = white, inactive = gray.
	col1, col2 := control.Gray, control.Gray
	if active == 1 {
		col1 = control.White
	}
	if active == 2 {
		col2 = control.White
	}

	box1 := stimuli.NewRectangle(-230, 0, 180, 160, col1)
	box2 := stimuli.NewRectangle(230, 0, 180, 160, col2)
	for _, b := range []*stimuli.Rectangle{box1, box2} {
		if err := b.Draw(exp.Screen); err != nil {
			return err
		}
	}

	lbl1 := stimuli.NewTextLine("1", -230, 0, control.Black)
	lbl2 := stimuli.NewTextLine("2", 230, 0, control.Black)
	for _, l := range []*stimuli.TextLine{lbl1, lbl2} {
		if err := l.Draw(exp.Screen); err != nil {
			return err
		}
	}

	exp.Screen.Update()
	return nil
}

// showFeedback briefly flashes the screen green (correct) or red (wrong).
func showFeedback(exp *control.Experiment, correct bool) error {
	color := control.Red
	if correct {
		color = control.Green
	}
	fb := stimuli.NewRectangle(0, 0, 500, 140, color)
	if err := exp.Show(fb); err != nil {
		return err
	}
	clock.Wait(300)
	return nil
}

// ─── Single 2-IFC trial ─────────────────────────────────────────────────────

// runTrial runs one 2-IFC trial for the given staircase and returns whether
// the response was correct (tone interval identified).
func runTrial(exp *control.Experiment, sc *staircase.UpDown, hz float64, maxReversals int, player *tonePlayer, trialNum int) (bool, error) {
	// Randomly assign tone to interval 1 or 2.
	toneInterval := 1 + rand.Intn(2)

	info := fmt.Sprintf("%.0f Hz  |  level %.1f dBFS  |  reversals %d/%d  |  trial %d",
		hz, sc.Intensity(), sc.NReversals(), maxReversals, trialNum)

	// Interval 1.
	if err := drawIFCScreen(exp, 1, info); err != nil {
		return false, err
	}
	if toneInterval == 1 {
		if err := player.play(hz, sc.Intensity()); err != nil {
			return false, err
		}
	}
	clock.Wait(500)
	player.stop()

	// Gap 400 ms.
	if err := drawIFCScreen(exp, 0, info); err != nil {
		return false, err
	}
	clock.Wait(400)

	// Interval 2.
	if err := drawIFCScreen(exp, 2, info); err != nil {
		return false, err
	}
	if toneInterval == 2 {
		if err := player.play(hz, sc.Intensity()); err != nil {
			return false, err
		}
	}
	clock.Wait(500)
	player.stop()

	// Response prompt.
	exp.Screen.Clear()
	exp.Keyboard.Clear() // discard stale keys before the response prompt appears
	prompt := stimuli.NewTextBox(
		"In which interval did you hear the tone?\nPress  1  or  2.",
		600, control.Point(0, 0), control.Black)
	if err := prompt.Present(exp.Screen, false, true); err != nil {
		return false, err
	}
	if err := prompt.Unload(); err != nil {
		return false, err
	}

	responseKeys := []control.Keycode{control.K_1, control.K_KP_1, control.K_2, control.K_KP_2}
	k, _, err := exp.Keyboard.WaitKeysRT(responseKeys, -1)
	if err != nil {
		return false, err
	}
	var response int
	if k == control.K_1 || k == control.K_KP_1 {
		response = 1
	} else {
		response = 2
	}

	correct := response == toneInterval
	if err := showFeedback(exp, correct); err != nil {
		return false, err
	}
	return correct, nil
}

// ─── Flag helpers ────────────────────────────────────────────────────────────

func parseFreqs(s string) ([]float64, error) {
	parts := strings.Split(s, ",")
	freqs := make([]float64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		f, err := strconv.ParseFloat(p, 64)
		if err != nil || f <= 0 {
			return nil, fmt.Errorf("invalid frequency %q", p)
		}
		freqs = append(freqs, f)
	}
	if len(freqs) == 0 {
		return nil, fmt.Errorf("no frequencies specified")
	}
	return freqs, nil
}

// ─── main ────────────────────────────────────────────────────────────────────

func main() {
	// register custom flags first (before NewExperimentFromFlags which calls flag.Parse)
	freqsFlag := flag.String("freqs", "50,250,500,1000,2000,4000,8000",
		"Comma-separated list of frequencies in Hz")
	startDB := flag.Float64("start", -20.0, "Starting intensity in dBFS (e.g. -20)")

	exp := control.NewExperimentFromFlags("Auditory Threshold Estimation", control.RGB(128, 128, 128), control.Black, 24)
	defer exp.End()

	if err := exp.SetLogicalSize(1024, 768); err != nil {
		log.Printf("warning: set logical size: %v", err)
	}

	freqs, err := parseFreqs(*freqsFlag)
	if err != nil {
		log.Fatalf("bad -freqs: %v", err)
	}

	exp.AddDataVariableNames([]string{
		"frequency_hz", "trial_number", "current_intensity_db",
		"response_correct", "reversal_occurred", "final_threshold_db",
	})

	runErr := exp.Run(func() error {
		// ── Instructions ────────────────────────────────────────────────────
		freqStrs := make([]string, len(freqs))
		for i, f := range freqs {
			freqStrs[i] = fmt.Sprintf("%.0f Hz", f)
		}
		instrText := fmt.Sprintf(
			"Auditory Threshold Test\n\n"+
				"On each trial you will hear two intervals, separated by a brief gap.\n"+
				"Only ONE interval contains a tone — the other is silence.\n\n"+
				"Press  1  if you heard the tone in interval 1.\n"+
				"Press  2  if you heard the tone in interval 2.\n\n"+
				"Frequencies tested: %s\n\n"+
				"Set your headphone/speaker volume to a comfortable level.\n"+
				"The tones will start at −20 dBFS (moderate level).\n\n"+
				"Press SPACE to begin.", strings.Join(freqStrs, ", "))

		instr := stimuli.NewTextBox(instrText, 900, control.Point(0, 0), control.Black)
		if err := exp.Show(instr); err != nil {
			return err
		}
		if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
			return err
		}
		if err := instr.Unload(); err != nil {
			return err
		}

		// ── Set up tone player ───────────────────────────────────────────────
		player, err := newTonePlayer(exp.AudioDevice)
		if err != nil {
			return fmt.Errorf("tone player: %w", err)
		}
		defer player.destroy()

		// ── Create one staircase per frequency ───────────────────────────────
		const maxReversals = 8
		freqFor := make(map[staircase.Staircase]float64, len(freqs))
		all := make([]staircase.Staircase, len(freqs))
		for i, hz := range freqs {
			sc := staircase.NewUpDown(staircase.UpDownConfig{
				StartIntensity:         *startDB,
				MinIntensity:           -80,
				MaxIntensity:           0,
				StepUp:                 4,
				StepDown:               4,
				NCorrectDown:           2, // 1-up/2-down → ~70.7 % threshold
				Phase2StepUp:           2,
				Phase2StepDown:         2,
				Phase2StartReversal:    2,
				MaxReversals:           maxReversals,
				NReversalsForThreshold: 4,
			})
			all[i] = sc
			freqFor[sc] = hz
		}

		runner := staircase.NewRunner(nil, all...)

		// ── Interleaved staircase loop ───────────────────────────────────────
		trialNum := 0
		for !runner.Done() {
			sc := runner.Next().(*staircase.UpDown)
			trialNum++
			correct, err := runTrial(exp, sc, freqFor[sc], maxReversals, player, trialNum)
			if err != nil {
				return err
			}
			sc.Update(correct)
		}

		// ── Log data ─────────────────────────────────────────────────────────
		globalTrial := 0
		for _, sc := range runner.All() {
			hz := freqFor[sc]
			history := sc.History()
			threshold := sc.Threshold()
			for i, trial := range history {
				globalTrial++
				thrStr := "NA"
				if i == len(history)-1 {
					thrStr = fmt.Sprintf("%.2f", threshold)
				}
				exp.Data.Add(hz, globalTrial, trial.Intensity, trial.Correct, trial.Reversal, thrStr)
			}
		}

		// ── Results summary ──────────────────────────────────────────────────
		lines := []string{"Estimated thresholds:\n"}
		for _, sc := range runner.All() {
			lines = append(lines,
				fmt.Sprintf("  %.0f Hz  →  %.1f dBFS", freqFor[sc], sc.Threshold()))
		}
		lines = append(lines, "\nPress SPACE to exit.")

		summary := stimuli.NewTextBox(
			strings.Join(lines, "\n"), 700, control.Point(0, 0), control.Black)
		if err := exp.Show(summary); err != nil {
			return err
		}
		if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
			return err
		}
		return control.EndLoop
	})

	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatalf("experiment error: %v", runErr)
	}
}
