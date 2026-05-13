// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Audio_record demonstrates optional microphone capture via SDL3. It records
// for a few seconds after you press SPACE, then writes session_mic.wav next
// to the CSV data file (see OutputDirectory / results path).
package main

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperimentFromFlags("Audio capture demo", control.Black, control.White, 32)
	defer exp.End()

	spec := stimuli.DefaultRecorderSpec()
	rec, err := exp.OpenAudioRecorder(&spec)
	if err != nil {
		log.Fatalf("open recorder: %v", err)
	}

	title := stimuli.NewTextLine("Press SPACE to record ~2 s, ESC to quit.", 0, 0, control.White)

	err = exp.Run(func() error {
		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := title.Draw(exp.Screen); err != nil {
			return err
		}
		if err := exp.Screen.Update(); err != nil {
			return err
		}

		st := exp.PollEvents(nil)
		if st.QuitRequested {
			return control.EndLoop
		}
		if st.LastKey == control.K_ESCAPE {
			return control.EndLoop
		}
		if st.LastKey != control.K_SPACE {
			clock.Wait(20)
			return nil
		}

		if err := rec.Start(); err != nil {
			return err
		}
		title.Text = "Recording…"
		if err := exp.Show(title); err != nil {
			_ = rec.Stop()
			return err
		}
		clock.Wait(2000)
		if err := rec.Stop(); err != nil {
			return err
		}

		pcm, derr := rec.Drain(nil)
		if derr != nil {
			return derr
		}
		out := filepath.Join(exp.Data.Directory, "session_mic.wav")
		outSpec := rec.OutputFormat()
		if err := stimuli.WriteFloat32WAV(out, pcm, int(outSpec.Freq), int(outSpec.Channels)); err != nil {
			return err
		}
		title.Text = fmt.Sprintf("Saved %s (%d bytes). ESC to exit.", out, len(pcm))
		if err := exp.Show(title); err != nil {
			return err
		}
		for {
			st := exp.PollEvents(nil)
			if st.QuitRequested || st.LastKey == control.K_ESCAPE {
				return control.EndLoop
			}
			clock.Wait(30)
		}
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
