// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Package safeexit runs hardware cleanup on Ctrl-C without making Ctrl-C stop
// working.
//
// # The trap this exists to close
//
// A hardware test wants to drive its lines LOW before it dies, so it catches
// SIGINT and does the cleanup itself:
//
//	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
//	go func() { <-stop; dev.AllLow(); dev.Close(); os.Exit(0) }()
//
// signal.Notify *replaces* the runtime's default die-on-SIGINT. From that call
// onward Ctrl-C goes to the channel and nowhere else, so the program's own
// handler is the only thing that can end it — and that handler's first act is
// an ioctl on the very hardware that may be why the operator is pressing
// Ctrl-C. If the device does not answer, the handler blocks, and the program
// can no longer be stopped from the keyboard at all. SIGTERM is caught the same
// way, so `kill` from another terminal is equally inert, and `timeout 10 …`
// does nothing either.
//
// That is not hypothetical: on 2026-08-21 a `test_parallel_port -blink` run
// against a PCIe LPT card stopped responding to Ctrl-C, and the machine had to
// be powered off at the switch.
//
// # What this does instead
//
// On the first signal it hands SIGINT and SIGTERM straight back to the runtime,
// so a second Ctrl-C kills by the default rule no matter what the cleanup is
// doing. Then it runs the cleanup on its own goroutine with a deadline, and
// exits when the cleanup finishes or the deadline passes, whichever comes
// first. A wedged device can cost you the cleanup; it can no longer cost you
// the process.
package safeexit

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// DefaultTimeout bounds the cleanup. It is generous for the intended work —
// one or two ioctls, a serial write, a file save — and short enough that an
// operator does not conclude the program has hung again.
const DefaultTimeout = 2 * time.Second

// ExitCode is the conventional shell status for death by SIGINT.
const ExitCode = 130

// OnSignal arranges for cleanup to run when SIGINT or SIGTERM arrives, and for
// the process to exit afterwards — within timeout, whether or not cleanup
// returns. Pass 0 for [DefaultTimeout].
//
// It returns immediately; the handler runs on its own goroutine.
//
//	safeexit.OnSignal(0, func() {
//	    _ = dev.AllLow()
//	    _ = dev.Close()
//	})
//
// cleanup must be safe to abandon: it may still be running when the process
// exits. Anything that must reach disk should be flushed by cleanup itself,
// early, rather than after a call that could block on hardware.
func OnSignal(timeout time.Duration, cleanup func()) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-ch
		// First, before touching anything that could block: give the signals
		// back to the runtime. From here a second Ctrl-C, or a plain kill,
		// ends the process by the default rule. This line is the whole point
		// of the package.
		signal.Reset(os.Interrupt, syscall.SIGTERM)
		fmt.Fprintf(os.Stderr, "\n%v — cleaning up (Ctrl-C again to exit at once)\n", sig)

		done := make(chan struct{})
		go func() {
			defer close(done)
			cleanup()
		}()
		select {
		case <-done:
		case <-time.After(timeout):
			fmt.Fprintf(os.Stderr,
				"cleanup did not finish within %v — exiting anyway.\n"+
					"    The device did not answer, so its lines may still be HIGH.\n", timeout)
		}
		os.Exit(ExitCode)
	}()
}
