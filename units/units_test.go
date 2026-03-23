// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package units

import (
	"math"
	"testing"
)

// standard lab setup used across most tests:
// 24″ 1920×1080 monitor, 60 cm viewing distance.
func stdMonitor() Monitor { return NewMonitorFromDiagonal(24, 1920, 1080, 60) }

// ── Round-trip consistency ────────────────────────────────────────────────────

func TestDegPxRoundTrip(t *testing.T) {
	m := stdMonitor()
	for _, deg := range []float64{0.1, 0.5, 1.0, 5.0, 10.0, 30.0} {
		got := m.PxToDeg(m.DegToPx(deg))
		if diff := math.Abs(got-deg) / deg; diff > 1e-9 {
			t.Errorf("PxToDeg(DegToPx(%.1f°)) = %.10f°, relative error %.2e", deg, got, diff)
		}
	}
}

func TestCmPxRoundTrip(t *testing.T) {
	m := stdMonitor()
	for _, cm := range []float64{0.1, 1.0, 5.0, 53.0} {
		got := m.PxToCm(m.CmToPx(cm))
		if diff := math.Abs(got - cm); diff > 1e-9 {
			t.Errorf("PxToCm(CmToPx(%.2f cm)) = %.10f cm, error %.2e", cm, got, diff)
		}
	}
}

func TestDegCmRoundTrip(t *testing.T) {
	m := stdMonitor()
	for _, deg := range []float64{0.5, 1.0, 5.0, 20.0} {
		got := m.CmToDeg(m.DegToCm(deg))
		if diff := math.Abs(got-deg) / deg; diff > 1e-9 {
			t.Errorf("CmToDeg(DegToCm(%.1f°)) = %.10f°, relative error %.2e", deg, got, diff)
		}
	}
}

// ── Known values from first principles ───────────────────────────────────────

func TestDegToCm_knownValue(t *testing.T) {
	// At 60 cm, 1° subtends 2·60·tan(0.5°) = 120·tan(π/360).
	m := NewMonitor(53.1, 29.9, 1920, 1080, 60)
	want := 2 * 60.0 * math.Tan(math.Pi/360)
	got := m.DegToCm(1)
	if math.Abs(got-want) > 1e-10 {
		t.Errorf("DegToCm(1°) = %.8f cm, want %.8f cm", got, want)
	}
}

func TestCmToDeg_knownValue(t *testing.T) {
	// At 60 cm, 1 cm subtends 2·atan(0.5/60)·(180/π).
	m := NewMonitor(53.1, 29.9, 1920, 1080, 60)
	want := 2 * math.Atan(0.5/60) * 180 / math.Pi
	got := m.CmToDeg(1)
	if math.Abs(got-want) > 1e-10 {
		t.Errorf("CmToDeg(1 cm) = %.8f°, want %.8f°", got, want)
	}
}

func TestPPD_range(t *testing.T) {
	// A typical lab monitor at 60 cm should give 30–60 px/°.
	m := stdMonitor()
	ppd := m.PPD()
	if ppd < 30 || ppd > 60 {
		t.Errorf("PPD() = %.1f px/°, expected 30–60 for a 24″ screen at 60 cm", ppd)
	}
}

// ── NewMonitorFromDiagonal ────────────────────────────────────────────────────

func TestNewMonitorFromDiagonal_dimensions(t *testing.T) {
	m := NewMonitorFromDiagonal(24, 1920, 1080, 60)

	// Diagonal: sqrt(width² + height²) must equal 24 inches = 60.96 cm.
	diag := math.Sqrt(m.WidthCm*m.WidthCm + m.HeightCm*m.HeightCm)
	wantDiag := 24 * 2.54
	if math.Abs(diag-wantDiag) > 1e-6 {
		t.Errorf("computed diagonal = %.4f cm, want %.4f cm", diag, wantDiag)
	}

	// Aspect ratio must match pixel ratio.
	wantRatio := float64(1920) / float64(1080)
	gotRatio := m.WidthCm / m.HeightCm
	if math.Abs(gotRatio-wantRatio)/wantRatio > 1e-9 {
		t.Errorf("physical aspect ratio = %.6f, want %.6f", gotRatio, wantRatio)
	}
}

// ── Square pixels ─────────────────────────────────────────────────────────────

func TestHasSquarePixels_true(t *testing.T) {
	// 1920×1080 at 16:9 aspect → square pixels when physical ratio matches.
	m := NewMonitorFromDiagonal(24, 1920, 1080, 60)
	if !m.HasSquarePixels() {
		t.Errorf("expected square pixels for a standard 16:9 monitor, PPcmX=%.4f PPcmY=%.4f",
			m.PPcmX(), m.PPcmY())
	}
}

func TestHasSquarePixels_false(t *testing.T) {
	// Deliberately non-square: same physical height/width as 16:9, but wrong pixel count.
	m := NewMonitor(53.0, 29.9, 1920, 1200, 60)
	if m.HasSquarePixels() {
		t.Errorf("expected non-square pixels, PPcmX=%.4f PPcmY=%.4f", m.PPcmX(), m.PPcmY())
	}
}

// ── Validate ──────────────────────────────────────────────────────────────────

func TestValidate(t *testing.T) {
	valid := stdMonitor()
	if err := valid.Validate(); err != nil {
		t.Errorf("valid monitor failed Validate: %v", err)
	}

	cases := []struct {
		name string
		m    Monitor
	}{
		{"zero WidthCm", Monitor{HeightCm: 1, WidthPx: 1, HeightPx: 1, DistanceCm: 1}},
		{"zero DistanceCm", Monitor{WidthCm: 1, HeightCm: 1, WidthPx: 1, HeightPx: 1}},
	}
	for _, tc := range cases {
		if err := tc.m.Validate(); err == nil {
			t.Errorf("case %q: expected error, got nil", tc.name)
		}
	}
}

// ── Monotonicity ──────────────────────────────────────────────────────────────

func TestDegToPx_monotone(t *testing.T) {
	m := stdMonitor()
	prev := m.DegToPx(0.1)
	for _, deg := range []float64{0.5, 1, 2, 5, 10, 30, 60} {
		cur := m.DegToPx(deg)
		if cur <= prev {
			t.Errorf("DegToPx not monotone: DegToPx(%.0f°)=%.2f ≤ previous %.2f", deg, cur, prev)
		}
		prev = cur
	}
}

// ── PPI smoke test ────────────────────────────────────────────────────────────

func TestPPI_range(t *testing.T) {
	m := NewMonitorFromDiagonal(24, 1920, 1080, 60)
	ppi := m.PPI()
	// A 24″ 1080p screen is ~91-92 PPI.
	if math.Abs(ppi-91.79) > 0.5 {
		t.Errorf("PPI() = %.2f, expected ~91.79 for 24″ 1080p", ppi)
	}
}
