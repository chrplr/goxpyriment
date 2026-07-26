// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// VSYNC Blocking Test
//
// Compares FlipTS and PacedFlipTS to determine if the platform/driver blocks on VSYNC
// (double-buffered, blocking Flip) or if it returns immediately (triple/mailbox/non-blocking buffering).
//
// Usage:
//
//	go run main.go
//	go run main.go -w     # windowed mode
package main

import (
	"fmt"
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperimentFromFlags("VSYNC Blocking Test", control.Black, control.White, 24)
	defer exp.End()

	err := exp.Run(func() error {
		nominalDuration := exp.Screen.FrameDuration()
		nominalMs := float64(nominalDuration.Nanoseconds()) / 1e6

		msg := stimuli.NewTextLine(fmt.Sprintf("Testing FlipTS() for 60 frames... (Nominal frame: %.2f ms)", nominalMs), 0, 0, control.White)
		
		// 1. Test FlipTS
		var flipIntervals []float64
		var prevTS uint64

		for i := 0; i < 60; i++ {
			if err := exp.Screen.Clear(); err != nil {
				return err
			}
			if err := msg.Draw(exp.Screen); err != nil {
				return err
			}
			ts, err := exp.Screen.FlipTS()
			if err != nil {
				return err
			}
			if i > 5 { // Discard warm-up frames
				flipIntervals = append(flipIntervals, float64(ts-prevTS)/1e6)
			}
			prevTS = ts
		}
		_ = msg.Unload()

		// 2. Test PacedFlipTS
		msg2 := stimuli.NewTextLine("Testing PacedFlipTS() for 60 frames...", 0, 0, control.White)
		var pacedIntervals []float64
		prevTS = 0

		for i := 0; i < 60; i++ {
			if err := exp.Screen.Clear(); err != nil {
				return err
			}
			if err := msg2.Draw(exp.Screen); err != nil {
				return err
			}
			ts, err := exp.Screen.PacedFlipTS()
			if err != nil {
				return err
			}
			if i > 5 { // Discard warm-up frames
				pacedIntervals = append(pacedIntervals, float64(ts-prevTS)/1e6)
			}
			prevTS = ts
		}
		_ = msg2.Unload()

		// 3. Compute Averages
		var sumFlip float64
		for _, v := range flipIntervals {
			sumFlip += v
		}
		avgFlip := sumFlip / float64(len(flipIntervals))

		var sumPaced float64
		for _, v := range pacedIntervals {
			sumPaced += v
		}
		avgPaced := sumPaced / float64(len(pacedIntervals))

		// 4. Determine Verdict
		var verdict string
		var recommendation string
		if avgFlip < nominalMs*0.7 {
			verdict = "NON-BLOCKING (Triple/mailbox buffering or driver compositor active)."
			recommendation = "You MUST use PacedFlip() or PacedFlipTS() inside dynamic animation loops.\nUsing Flip() or FlipTS() natively will cause frame swallowing."
		} else {
			verdict = "BLOCKING (Double-buffered VSYNC behaves normally)."
			recommendation = "Both FlipTS() and PacedFlipTS() are safe and performant.\nPacedFlip() will behave identically to Flip() with negligible overhead."
		}

		resultText := fmt.Sprintf(
			"RESULTS:\n\n"+
				"Nominal Frame Duration : %6.2f ms\n"+
				"Average FlipTS()       : %6.2f ms\n"+
				"Average PacedFlipTS()  : %6.2f ms\n\n"+
				"VSYNC Behavior         : %s\n\n"+
				"Recommendation         :\n%s\n\n"+
				"Press any key to exit.",
			nominalMs, avgFlip, avgPaced, verdict, recommendation,
		)

		log.Printf("Nominal Frame Duration: %.2f ms", nominalMs)
		log.Printf("Average FlipTS()      : %.2f ms", avgFlip)
		log.Printf("Average PacedFlipTS() : %.2f ms", avgPaced)
		log.Printf("Verdict: %s", verdict)

		tb := stimuli.NewTextBox(resultText, 800, control.Origin(), control.White)
		if err := exp.Show(tb); err != nil {
			return err
		}

		_, err := exp.Keyboard.Wait()
		return err
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
