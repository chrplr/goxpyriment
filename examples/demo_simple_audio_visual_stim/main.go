package main

import (
	. "github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

const NTRIALS = 20

func main() {
	exp := NewExperimentFromFlags("Test", Black, White, 32)
	defer exp.End()

	instr := stimuli.NewTextBox("A white rectangle will be displayed periodically,\nPress 'j' as soon as you see it.\n\nPress any key to start\n\n'Esc' to interrupt", 600, FPoint{}, White)

	w, h, _ := exp.Screen.Size()
	rect := stimuli.NewRectangle(0, 0, float32(w)/5.0, float32(h)/5.0, White)

	sound := stimuli.NewTone(440, 120, 0.5)
	sound.PreloadDevice(exp.AudioDevice) // generates the sound

	exp.AddDataVariableNames([]string{"onset", "duration", "response", "rt"})

	exp.Show(instr)
	exp.Keyboard.Wait()

	// Wake the audio sink before the first trial. PipeWire and PulseAudio
	// suspend a device that has been idle for a few seconds — which it has
	// been, all through the instructions screen — and the first tone after
	// that is swallowed while the device resumes. Amplitude 0 = silence, so
	// the warm-up itself is inaudible. The 1.8 s trials keep the sink awake
	// from here on.
	warmup := stimuli.NewTone(440, 100, 0)
	warmup.PreloadDevice(exp.AudioDevice)
	warmup.Play()
	exp.Wait(300)

	for range NTRIALS {
		exp.Keyboard.Clear() // drop anything left over from the previous trial
		onsetNS, _ := exp.ShowTS(rect)

		sound.Play()

		exp.Wait(200)

		exp.Screen.Clear()
		offsetNS, _ := exp.Screen.FlipTS()

		exp.Wait(800)

		key, keyTS, err := exp.Keyboard.GetKeyEventTS(nil, 800)

		if IsEndLoop(err) { // participant pressed Esc, or closed the window
			break
		}

		duration_ms := (offsetNS - onsetNS) / 1_000_000

		// A timeout returns keycode 0 with no timestamp; subtracting the onset
		// from it would underflow, so report the missing response instead.
		response := "n/a"
		rt_ms := int64(-1)
		if key != 0 {
			response = key.KeyName()
			rt_ms = int64(keyTS-onsetNS) / 1_000_000
		}

		exp.Data.Add(onsetNS/1_000_000, duration_ms, response, rt_ms)
	}
}
