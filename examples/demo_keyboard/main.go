// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// test_keyboard demonstrates every keyboard input method provided by the
// goxpyriment framework. Run with:
//
//	go run examples/test_keyboard/main.go -w
//
// Each section is self-contained; press SPACE (or the indicated key) to
// advance to the next one. ESC quits at any time.
package main

import (
	"fmt"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// show displays a TextBox with the given message and waits for the user to
// press SPACE before continuing.
func show(exp *control.Experiment, msg string) {
	box := stimuli.NewTextBox(msg+"\n\n[SPACE to continue]", 900, control.FPoint{}, control.White)
	exp.Show(box)
	exp.Keyboard.WaitKey(control.K_SPACE)
}

// display renders a message without waiting.
func display(exp *control.Experiment, msg string) {
	box := stimuli.NewTextBox(msg, 900, control.FPoint{}, control.White)
	exp.Show(box)
}

func main() {
	exp := control.NewExperimentFromFlags("Keyboard Demo", control.Black, control.White, 32)
	defer exp.End()

	err := exp.Run(func() error {

		// ----------------------------------------------------------------
		// 0. Introduction
		// ----------------------------------------------------------------
		show(exp,
			"KEYBOARD INPUT DEMO\n\n"+
				"This demo walks through every keyboard input method in the\n"+
				"goxpyriment framework. Each section shows one technique.\n\n"+
				"ESC quits at any time.")

		// ----------------------------------------------------------------
		// 1. Wait() — block until any key
		// ----------------------------------------------------------------
		display(exp,
			"1. Wait()\n\n"+
				"Blocks until the participant presses any key.\n"+
				"Returns the keycode.\n\n"+
				"Press any key now...")

		key, err := exp.Keyboard.Wait()
		if err != nil {
			return err
		}
		show(exp, fmt.Sprintf("1. Wait() — result\n\nYou pressed: %s (keycode %d)", key.KeyName(), key))

		// ----------------------------------------------------------------
		// 2. WaitKey() — block until a specific key
		// ----------------------------------------------------------------
		show(exp,
			"2. WaitKey()\n\n"+
				"Blocks until one specific key is pressed; ignores all others.\n\n"+
				"Press the F key (other keys are ignored).")

		exp.Keyboard.Clear()
		display(exp, "2. WaitKey()\n\nWaiting for F...")
		if err := exp.Keyboard.WaitKey(control.K_F); err != nil {
			return err
		}
		show(exp, "2. WaitKey() — result\n\nGot F!")

		// ----------------------------------------------------------------
		// 3. WaitKeys() — first of several keys, with timeout
		// ----------------------------------------------------------------
		show(exp,
			"3. WaitKeys()\n\n"+
				"Waits for the first of a set of keys within a time limit.\n"+
				"Returns 0 on timeout.\n\n"+
				"Press F or J within 3 seconds.")

		exp.Keyboard.Clear()
		display(exp, "3. WaitKeys()\n\nWaiting for F or J (3 s timeout)...")
		responseKeys := []control.Keycode{control.K_F, control.K_J}
		key, err = exp.Keyboard.WaitKeys(responseKeys, 3000)
		if err != nil {
			return err
		}
		if key == 0 {
			show(exp, "3. WaitKeys() — result\n\nTimeout — no key pressed within 3 s.")
		} else {
			show(exp, fmt.Sprintf("3. WaitKeys() — result\n\nYou pressed: %s", key.KeyName()))
		}

		// ----------------------------------------------------------------
		// 4. WaitKeysRT() — reaction time from call site (milliseconds)
		// ----------------------------------------------------------------
		show(exp,
			"4. WaitKeysRT()\n\n"+
				"Like WaitKeys but also returns the reaction time (ms) measured\n"+
				"from the moment the call is made — not from stimulus onset.\n\n"+
				"Press F or J as fast as you can.")

		exp.Keyboard.Clear()
		display(exp, "4. WaitKeysRT()\n\nPress F or J...")
		key, rt, err := exp.Keyboard.WaitKeysRT(responseKeys, -1)
		if err != nil {
			return err
		}
		show(exp, fmt.Sprintf("4. WaitKeysRT() — result\n\nKey: %s   RT: %d ms (from call site)", key.KeyName(), rt))

		// ----------------------------------------------------------------
		// 5. GetKeyEventTS() — hardware-precision RT from stimulus onset
		// ----------------------------------------------------------------
		show(exp,
			"5. GetKeyEventTS()\n\n"+
				"Returns the SDL3 hardware event timestamp (nanoseconds).\n"+
				"Subtract it from ShowTS onset for precise stimulus-locked RT.\n\n"+
				"A fixation cross will appear for 1 s, then a GO signal.\n"+
				"Press F or J as fast as you can after GO.")

		exp.Keyboard.Clear()
		fix := stimuli.NewFixCross(30, 3, control.White)
		exp.Show(fix)
		exp.Wait(1000)

		go_stim := stimuli.NewTextLine("GO", 0, 0, control.Green)
		onset, _ := exp.ShowTS(go_stim)

		key, keyTS, err := exp.Keyboard.GetKeyEventTS(responseKeys, 5000)
		if err != nil {
			return err
		}
		if key == 0 {
			show(exp, "5. GetKeyEventTS() — result\n\nTimeout.")
		} else {
			rtNS := int64(keyTS - onset)
			show(exp, fmt.Sprintf(
				"5. GetKeyEventTS() — result\n\n"+
					"Key: %s\n"+
					"RT from GO onset: %d ms (hardware precision)",
				key.KeyName(), rtNS/1_000_000))
		}

		// ----------------------------------------------------------------
		// 6. GetKeyEventsTS() — all events in queue (simultaneous presses)
		// ----------------------------------------------------------------
		show(exp,
			"6. GetKeyEventsTS()\n\n"+
				"Like GetKeyEventTS but returns ALL matching events in the queue,\n"+
				"ordered by hardware timestamp. Useful for bilateral responses.\n\n"+
				"Try pressing F and J simultaneously (within ~50 ms of each other).")

		exp.Keyboard.Clear()
		display(exp, "6. GetKeyEventsTS()\n\nPress F and/or J (try both at once)...")
		events, err := exp.Keyboard.GetKeyEventsTS(responseKeys, 5000)
		if err != nil {
			return err
		}
		if len(events) == 0 {
			show(exp, "6. GetKeyEventsTS() — result\n\nTimeout.")
		} else {
			msg := fmt.Sprintf("6. GetKeyEventsTS() — result\n\n%d event(s) received:\n", len(events))
			for i, ev := range events {
				var lag string
				if i > 0 {
					lagNS := int64(ev.TimestampNS - events[0].TimestampNS)
					lag = fmt.Sprintf("  (+%d ms after first)", lagNS/1_000_000)
				}
				msg += fmt.Sprintf("  [%d] %s%s\n", i+1, ev.Key.KeyName(), lag)
			}
			show(exp, msg)
		}

		// ----------------------------------------------------------------
		// 7. Check() — non-blocking poll
		// ----------------------------------------------------------------
		show(exp,
			"7. Check()\n\n"+
				"Non-blocking: returns immediately with 0 if no key is queued.\n"+
				"Useful inside animation loops.\n\n"+
				"A counter will run for up to 5 s. Press any key to stop it.")

		exp.Keyboard.Clear()
		start := time.Now()
		var pressedKey control.Keycode
		for time.Since(start) < 5*time.Second {
			elapsed := time.Since(start)
			display(exp, fmt.Sprintf("7. Check()\n\nElapsed: %.1f s\n\nPress any key to stop.", elapsed.Seconds()))

			k, err := exp.Keyboard.Check()
			if err != nil {
				return err
			}
			if k != 0 {
				pressedKey = k
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if pressedKey != 0 {
			show(exp, fmt.Sprintf("7. Check() — result\n\nStopped by key: %s", pressedKey.KeyName()))
		} else {
			show(exp, "7. Check() — result\n\nTimer expired (no key pressed).")
		}

		// ----------------------------------------------------------------
		// 8. IsPressed() — instantaneous key-state query
		// ----------------------------------------------------------------
		show(exp,
			"8. IsPressed()\n\n"+
				"Queries whether a key is physically held down right now,\n"+
				"without consuming any event queue entry.\n\n"+
				"Hold the SPACE bar. A counter increments while it is held.\n"+
				"Release SPACE to continue.")

		exp.Keyboard.Clear()
		holdCount := 0
		for {
			// PollEvents keeps the window responsive and handles ESC/quit.
			state := exp.PollEvents(nil)
			if state.QuitRequested {
				return control.EndLoop
			}

			held := exp.Keyboard.IsPressed(control.K_SPACE)
			if held {
				holdCount++
				display(exp, fmt.Sprintf("8. IsPressed()\n\nSPACE held — samples: %d\n\n(release to continue)", holdCount))
			} else if holdCount > 0 {
				// Key was held and now released
				break
			} else {
				display(exp, "8. IsPressed()\n\nHold the SPACE bar...")
			}
			time.Sleep(50 * time.Millisecond)
		}
		show(exp, fmt.Sprintf("8. IsPressed() — result\n\nDetected %d samples while SPACE was held (~%d ms at 50 ms poll rate).",
			holdCount, holdCount*50))

		// ----------------------------------------------------------------
		// 9. WaitKeyReleaseTS() — keypress duration
		// ----------------------------------------------------------------
		show(exp,
			"9. WaitKeyReleaseTS()\n\n"+
				"Waits for KEY_UP and returns its SDL3 hardware timestamp.\n"+
				"Combined with GetKeyEventTS, gives nanosecond-precision\n"+
				"keypress duration.\n\n"+
				"Press and HOLD the F key, then release it.")

		exp.Keyboard.Clear()
		display(exp, "9. WaitKeyReleaseTS()\n\nPress and hold F, then release...")
		pressKey, downTS, err := exp.Keyboard.GetKeyEventTS([]control.Keycode{control.K_F}, -1)
		if err != nil {
			return err
		}
		display(exp, "9. WaitKeyReleaseTS()\n\nF is down — release it now...")
		upTS, err := exp.Keyboard.WaitKeyReleaseTS(pressKey, -1)
		if err != nil {
			return err
		}
		durationMS := int64(upTS-downTS) / 1_000_000
		show(exp, fmt.Sprintf(
			"9. WaitKeyReleaseTS() — result\n\n"+
				"Key: %s\n"+
				"Press duration: %d ms (hardware precision)",
			pressKey.KeyName(), durationMS))

		// ----------------------------------------------------------------
		// Done
		// ----------------------------------------------------------------
		show(exp,
			"All keyboard input methods demonstrated.\n\n"+
				"Summary:\n"+
				"  Wait()            — any key, blocking\n"+
				"  WaitKey()         — specific key, blocking\n"+
				"  WaitKeys()        — first of set, with timeout\n"+
				"  WaitKeysRT()      — + RT from call site (ms)\n"+
				"  GetKeyEventTS()   — + hardware timestamp (ns), RT from onset\n"+
				"  GetKeyEventsTS()  — all events in queue\n"+
				"  Check()           — non-blocking poll\n"+
				"  IsPressed()       — instantaneous held-key state\n"+
				"  WaitKeyReleaseTS()— wait for KEY_UP, measure duration\n"+
				"  Clear()           — drain event queue before trial")

		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
