// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Psychological Refractory Period (PRP) paradigm.
//
// Two tasks overlap in time with a varying Stimulus Onset Asynchrony (SOA).
// Task 1 (auditory): classify a tone as Low (400 Hz → 'S') or High (900 Hz → 'D').
// Task 2 (visual): classify a letter as 'A' (→ 'K') or 'B' (→ 'L').
//
// The PRP effect predicts that RT2 increases as SOA decreases.
//
// Usage:
//
//	go run main.go [-s <id>] [-d]
package main

import (
	"fmt"
	"log"
	"math"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ─── Experiment parameters ───────────────────────────────────────────────────

const (
	// Task 1 response keys.
	KeyLow  = control.K_S // 'S' → low tone (400 Hz)
	KeyHigh = control.K_D // 'D' → high tone (900 Hz)

	// Task 2 response keys.
	KeyA = control.K_K // 'K' → letter 'A'
	KeyB = control.K_L // 'L' → letter 'B'

	ToneDurationMs = 200 // tone duration in ms
	ToneRampMs     = 20  // linear fade-in/out ramp
	FreqLow        = 400.0
	FreqHigh       = 900.0

	FixationMs = 500  // fixation cross duration
	TimeoutMs  = 3000 // response window timeout (from S2 onset)
	ITIMs      = 1000 // inter-trial interval

	PracticeTrials = 16 // 1 rep × 16 conditions
	TrialsPerBlock = 48 // 3 reps × 16 conditions
	NMainBlocks    = 3
)

var SOALevels = []int{50, 150, 400, 800} // ms

// ─── Tone generation ────────────────────────────────────────────────────────

func generateTone(freqHz float64, durationMs, rampMs int) []byte {
	const sampleRate = 44100
	nSamples := sampleRate * durationMs / 1000
	rampN := sampleRate * rampMs / 1000

	data := make([]byte, nSamples*4)
	for i := 0; i < nSamples; i++ {
		v := float32(math.Sin(2 * math.Pi * freqHz * float64(i) / sampleRate))
		if i < rampN {
			v *= float32(i) / float32(rampN)
		}
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

// ─── Tone player ─────────────────────────────────────────────────────────────

type tonePlayer struct {
	stream   *sdl.AudioStream
	toneLow  []byte
	toneHigh []byte
}

func newTonePlayer(device sdl.AudioDeviceID) (*tonePlayer, error) {
	spec := &sdl.AudioSpec{Format: sdl.AUDIO_F32LE, Channels: 1, Freq: 44100}
	stream, err := sdl.CreateAudioStream(spec, spec)
	if err != nil {
		return nil, fmt.Errorf("create audio stream: %w", err)
	}
	if err := device.BindAudioStream(stream); err != nil {
		stream.Destroy()
		return nil, fmt.Errorf("bind audio stream: %w", err)
	}
	return &tonePlayer{
		stream:   stream,
		toneLow:  generateTone(FreqLow, ToneDurationMs, ToneRampMs),
		toneHigh: generateTone(FreqHigh, ToneDurationMs, ToneRampMs),
	}, nil
}

func (p *tonePlayer) play(high bool) (int64, error) {
	data := p.toneLow
	if high {
		data = p.toneHigh
	}
	_ = p.stream.Clear()
	err := p.stream.PutData(data)
	t0 := clock.GetTime()
	return t0, err
}

func (p *tonePlayer) destroy() { p.stream.Destroy() }

// ─── Trial definition ────────────────────────────────────────────────────────

type trialDef struct {
	soaMs    int
	highTone bool // true = 900 Hz, false = 400 Hz
	letterA  bool // true = 'A', false = 'B'
}

// buildTrialList returns a balanced, shuffled slice of trial definitions.
// reps: number of repetitions of the full 16-condition set.
func buildTrialList(reps int) []trialDef {
	var trials []trialDef
	for r := 0; r < reps; r++ {
		for _, soa := range SOALevels {
			for _, high := range []bool{false, true} {
				for _, letA := range []bool{false, true} {
					trials = append(trials, trialDef{soaMs: soa, highTone: high, letterA: letA})
				}
			}
		}
	}
	design.ShuffleList(trials)
	return trials
}

// ─── Single trial ────────────────────────────────────────────────────────────

type trialResult struct {
	trialNum int
	soaMs    int
	s1Type   string // "Low" or "High"
	s2Type   string // "A" or "B"
	rt1Ms    int64  // RT for Task 1, -1 if no response
	rt2Ms    int64  // RT for Task 2 (relative to S2 onset), -1 if no response
	acc1     bool
	acc2     bool
	orderErr bool // Task 2 key pressed before Task 1 key
	timeout  bool
}

func runTrial(exp *control.Experiment, player *tonePlayer,
	t trialDef, trialNum int,
	fixCross *stimuli.FixCross,
	stimA, stimB *stimuli.TextLine,
	showFeedback bool) trialResult {

	res := trialResult{
		trialNum: trialNum,
		soaMs:    t.soaMs,
		rt1Ms:    -1,
		rt2Ms:    -1,
	}
	if t.highTone {
		res.s1Type = "High"
	} else {
		res.s1Type = "Low"
	}
	if t.letterA {
		res.s2Type = "A"
	} else {
		res.s2Type = "B"
	}

	// 1. Fixation.
	exp.Show(fixCross)
	exp.Wait(FixationMs)

	// 2. S1 onset — play tone and record T0.
	t0, err := player.play(t.highTone)
	if err != nil {
		log.Printf("Warning: play tone failed: %v", err)
	}

	// 3. SOA wait, then show S2.
	// During the SOA we also poll for Task 1 responses.
	var r1Time int64 = -1
	var r2Time int64 = -1
	var r1Key control.Keycode
	var r2Key control.Keycode

	// Helper to check if a key is a Task1 or Task2 key.
	isTask1Key := func(k control.Keycode) bool { return k == KeyLow || k == KeyHigh }
	isTask2Key := func(k control.Keycode) bool { return k == KeyA || k == KeyB }

	// Poll during SOA.
	allKeys := []control.Keycode{KeyLow, KeyHigh, KeyA, KeyB}
	soaDeadline := t0 + int64(t.soaMs)
	for r1Time == -1 || r2Time == -1 {
		now := clock.GetTime()
		if now >= soaDeadline {
			break
		}
		k, kRt, _ := exp.Keyboard.WaitKeysRT(allKeys, int(soaDeadline-now))
		if k == 0 {
			break // timeout
		}
		keyTime := now + kRt
		if r1Time == -1 && isTask1Key(k) {
			r1Time = keyTime
			r1Key = k
		}
		if r2Time == -1 && isTask2Key(k) {
			r2Time = keyTime
			r2Key = k
		}
	}

	// 4. S2 onset.
	tSOA := clock.GetTime() // actual S2 onset
	var s2Stim *stimuli.TextLine
	if t.letterA {
		s2Stim = stimA
	} else {
		s2Stim = stimB
	}
	// Draw fixation + letter together.
	_ = exp.Screen.Clear()
	_ = fixCross.Draw(exp.Screen)
	_ = s2Stim.Draw(exp.Screen)
	_ = exp.Screen.Update()

	// 5. Response window — collect any remaining responses until timeout.
	deadline := tSOA + int64(TimeoutMs)
	for r1Time == -1 || r2Time == -1 {
		now := clock.GetTime()
		if now >= deadline {
			break
		}
		k, kRt, _ := exp.Keyboard.WaitKeysRT(allKeys, int(deadline-now))
		if k == 0 {
			break // timeout
		}
		keyTime := now + kRt
		if r1Time == -1 && isTask1Key(k) {
			r1Time = keyTime
			r1Key = k
		}
		if r2Time == -1 && isTask2Key(k) {
			r2Time = keyTime
			r2Key = k
		}
	}

	// Clear S2.
	exp.Show(fixCross)

	// 6. Compute results.
	if r1Time != -1 {
		res.rt1Ms = r1Time - t0
		correctT1 := (t.highTone && r1Key == KeyHigh) || (!t.highTone && r1Key == KeyLow)
		res.acc1 = correctT1
	}
	if r2Time != -1 {
		res.rt2Ms = r2Time - tSOA
		correctT2 := (t.letterA && r2Key == KeyA) || (!t.letterA && r2Key == KeyB)
		res.acc2 = correctT2
	}
	if r1Time == -1 || r2Time == -1 {
		res.timeout = true
	}
	if r1Time != -1 && r2Time != -1 && r2Time < r1Time {
		res.orderErr = true
	}

	// 7. Optional feedback.
	if showFeedback {
		msg := ""
		if res.timeout {
			msg = "Too slow!"
		} else if res.orderErr {
			msg = "Order error: respond to tone first!"
		} else if res.acc1 && res.acc2 {
			msg = "Correct!"
		} else {
			msg = "Error"
		}
		fbColor := control.Green
		if !res.acc1 || !res.acc2 || res.orderErr || res.timeout {
			fbColor = control.Red
		}
		fb := stimuli.NewTextLine(msg, 0, 0, fbColor)
		exp.Show(fb)
		exp.Wait(700)
	}

	// 8. ITI.
	exp.Blank(ITIMs)

	return res
}

// ─── Block runner ─────────────────────────────────────────────────────────────

func runBlock(exp *control.Experiment, player *tonePlayer,
	trials []trialDef, startTrialNum int,
	fixCross *stimuli.FixCross,
	stimA, stimB *stimuli.TextLine,
	showFeedback bool) []trialResult {

	var results []trialResult
	for i, t := range trials {
		r := runTrial(exp, player, t, startTrialNum+i, fixCross, stimA, stimB, showFeedback)
		results = append(results, r)

		rt1Str := fmt.Sprintf("%d", r.rt1Ms)
		if r.rt1Ms == -1 {
			rt1Str = "NA"
		}
		rt2Str := fmt.Sprintf("%d", r.rt2Ms)
		if r.rt2Ms == -1 {
			rt2Str = "NA"
		}
		exp.Data.Add(
			r.trialNum, r.soaMs, r.s1Type, r.s2Type,
			rt1Str, rt2Str, r.acc1, r.acc2, r.orderErr,
		)
		fmt.Printf("Trial %3d  SOA=%3d  S1=%s S2=%s  RT1=%s RT2=%s  acc1=%v acc2=%v  orderErr=%v\n",
			r.trialNum, r.soaMs, r.s1Type, r.s2Type, rt1Str, rt2Str, r.acc1, r.acc2, r.orderErr)
	}
	return results
}

// ─── main ─────────────────────────────────────────────────────────────────────

func main() {
	exp := control.NewExperimentFromFlags("PRP Task", control.Black, control.White, 48)
	defer exp.End()

	if err := exp.SetLogicalSize(1366, 768); err != nil {
		log.Printf("warning: set logical size: %v", err)
	}

	exp.AddDataVariableNames([]string{
		"Trial_Number", "SOA_Value", "S1_Type", "S2_Type",
		"RT1", "RT2", "Accuracy_S1", "Accuracy_S2", "Order_Error",
	})

	runErr := exp.Run(func() error {
		// ── Set up tone player ───────────────────────────────────────────────
		player, err := newTonePlayer(exp.AudioDevice)
		if err != nil {
			return fmt.Errorf("tone player: %w", err)
		}
		defer player.destroy()

		// ── Pre-build stimuli ────────────────────────────────────────────────
		fixCross := stimuli.NewFixCross(30, 3, control.White)
		stimA := stimuli.NewTextLine("A", 0, 0, control.White)
		stimB := stimuli.NewTextLine("B", 0, 0, control.White)

		// ── Instructions ────────────────────────────────────────────────────
		instrText := "Psychological Refractory Period Task\n\n" +
			"You will perform TWO tasks simultaneously:\n\n" +
			"TASK 1 — Tone (respond FIRST):\n" +
			"  Low tone (400 Hz) → press S\n" +
			"  High tone (900 Hz) → press D\n\n" +
			"TASK 2 — Letter:\n" +
			"  Letter A → press K\n" +
			"  Letter B → press L\n\n" +
			"Always respond to the TONE first, then to the letter.\n" +
			"Respond as quickly and accurately as possible.\n\n" +
			"Press SPACE to begin the practice block."
		exp.ShowInstructions(instrText)

		// ── Practice block ───────────────────────────────────────────────────
		practiceTrials := buildTrialList(1) // 16 trials = 1 rep
		practiceTrials = practiceTrials[:PracticeTrials]

		notice := "PRACTICE BLOCK\n\nFeedback will be shown after each trial.\n\nPress SPACE to start."
		exp.ShowInstructions(notice)

		runBlock(exp, player, practiceTrials, 0, fixCross, stimA, stimB, true)

		// ── Main blocks ──────────────────────────────────────────────────────
		trialCounter := 0
		for block := 1; block <= NMainBlocks; block++ {
			blockTrials := buildTrialList(3) // 48 trials = 3 reps × 16 conditions

			blockMsg := fmt.Sprintf(
				"Block %d of %d\n\nNo feedback will be shown.\n\nPress SPACE to begin.",
				block, NMainBlocks)
			exp.ShowInstructions(blockMsg)

			runBlock(exp, player, blockTrials, trialCounter+1, fixCross, stimA, stimB, false)
			trialCounter += len(blockTrials)
		}

		// ── Save & goodbye ───────────────────────────────────────────────────
		_ = exp.Data.Save()

		goodbye := "Experiment complete!\n\nThank you for your participation.\n\nPress SPACE to exit."
		exp.ShowInstructions(goodbye)

		return control.EndLoop
	})

	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatalf("experiment error: %v", runErr)
	}
}
