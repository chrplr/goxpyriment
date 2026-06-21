// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package triggers

import (
	"context"
	"errors"
	"testing"
	"time"
)

// scriptedReadAll returns successive masks from the slice on each call; once
// exhausted it keeps returning the final value. If err is non-nil it is
// returned on the first call instead.
func scriptedReadAll(masks []byte, err error) func() (byte, error) {
	i := 0
	return func() (byte, error) {
		if err != nil {
			return 0, err
		}
		m := masks[i]
		if i < len(masks)-1 {
			i++
		}
		return m, nil
	}
}

func TestReadLineFromMask(t *testing.T) {
	readAll := func() (byte, error) { return 0b00000101, nil }

	for _, tc := range []struct {
		line int
		want byte
	}{{0, 1}, {1, 0}, {2, 1}, {7, 0}} {
		got, err := readLineFromMask("test", readAll, tc.line)
		if err != nil {
			t.Fatalf("line %d: unexpected error: %v", tc.line, err)
		}
		if got != tc.want {
			t.Errorf("line %d: got %d, want %d", tc.line, got, tc.want)
		}
	}
}

func TestReadLineFromMaskOutOfRange(t *testing.T) {
	readAll := func() (byte, error) { return 0xFF, nil }
	for _, line := range []int{-1, 8, 99} {
		if _, err := readLineFromMask("test", readAll, line); err == nil {
			t.Errorf("line %d: expected out-of-range error, got nil", line)
		}
	}
}

func TestReadLineFromMaskReadAllError(t *testing.T) {
	sentinel := errors.New("hw failure")
	readAll := func() (byte, error) { return 0, sentinel }
	_, err := readLineFromMask("test", readAll, 0)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped sentinel, got %v", err)
	}
}

func TestPollWaitForInputDetectsPress(t *testing.T) {
	// Inactive for two polls, then line 2 active.
	readAll := scriptedReadAll([]byte{0, 0, 0b00000100}, nil)
	mask, rt, err := pollWaitForInput(context.Background(), "test", readAll, time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mask != 0b00000100 {
		t.Errorf("mask: got %08b, want 00000100", mask)
	}
	if rt <= 0 {
		t.Errorf("reaction time should be positive, got %v", rt)
	}
}

func TestPollWaitForInputContextCancel(t *testing.T) {
	readAll := func() (byte, error) { return 0, nil } // never active
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_, _, err := pollWaitForInput(ctx, "test", readAll, time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestPollWaitForInputReadError(t *testing.T) {
	sentinel := errors.New("read boom")
	_, _, err := pollWaitForInput(context.Background(), "test", scriptedReadAll(nil, sentinel), time.Millisecond)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped sentinel, got %v", err)
	}
}

func TestPollDrainInputs(t *testing.T) {
	// Active for two polls, then clears.
	readAll := scriptedReadAll([]byte{0xFF, 0x01, 0x00}, nil)
	if err := pollDrainInputs(context.Background(), "test", readAll, time.Millisecond); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPollDrainInputsContextCancel(t *testing.T) {
	readAll := func() (byte, error) { return 0xFF, nil } // never clears
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if err := pollDrainInputs(ctx, "test", readAll, time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

// The Null devices must satisfy the interfaces and the shared helpers' contract.
func TestNullInputDeviceWithHelpers(t *testing.T) {
	var dev NullInputTTLDevice
	if v, err := readLineFromMask("null", dev.ReadAll, 3); err != nil || v != 0 {
		t.Fatalf("readLineFromMask on null: got %d, %v", v, err)
	}
	// Null always reads 0, so WaitForInput must honour ctx cancellation.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if _, _, err := pollWaitForInput(ctx, "null", dev.ReadAll, time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
	if err := pollDrainInputs(context.Background(), "null", dev.ReadAll, time.Millisecond); err != nil {
		t.Fatalf("drain on null: %v", err)
	}
}
