package mixedphase

import (
	"math"
	"testing"
)

// denseGrid is far enough beyond any default to serve as the converged
// reference these tests measure against.
const denseGrid = 131072

func peakMagnitude(taps []float64) float64 {
	peak := 0.0
	for _, tap := range taps {
		peak = math.Max(peak, math.Abs(tap))
	}

	return peak
}

func worstDeviation(got, want []float64) float64 {
	worst := 0.0
	for i := range want {
		worst = math.Max(worst, math.Abs(got[i]-want[i]))
	}

	return worst
}

// TestMinimumPhaseAliasingBudget states the error the default grid actually
// carries, which AGENTS.md rule 1 requires of any quoted number.
//
// The default is a fixed multiple of the prototype length, not a converged
// choice, so the residual time-domain aliasing does not vanish as prototypes
// grow. These figures are the budget a caller accepts by leaving FFTSize at
// zero; MinimumPhaseConfig.FFTSize is the dial that buys accuracy back.
func TestMinimumPhaseAliasingBudget(t *testing.T) {
	tests := []struct {
		name           string
		prototype      []float64
		wantRelativeAt float64
	}{
		// A two-tap prototype is the worst case in the package: the default
		// grid is the 16-point floor, and the reconstruction is wrong by more
		// than 40% of its own peak.
		{name: "two taps", prototype: []float64{1, -1}, wantRelativeAt: 0.45},
		{name: "9 taps", prototype: lowpassPrototype(9, 0.2), wantRelativeAt: 0.02},
		{name: "33 taps", prototype: lowpassPrototype(33, 0.2), wantRelativeAt: 0.01},
		{name: "129 taps", prototype: lowpassPrototype(129, 0.08), wantRelativeAt: 0.01},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			converged, err := MinimumPhase(test.prototype, denseGrid)
			if err != nil {
				t.Fatalf("MinimumPhase(dense) error = %v", err)
			}

			def, err := MinimumPhase(test.prototype, 0)
			if err != nil {
				t.Fatalf("MinimumPhase(default) error = %v", err)
			}

			relative := worstDeviation(def, converged) / peakMagnitude(converged)
			if relative > test.wantRelativeAt {
				t.Errorf(
					"default-grid deviation = %.4f of peak, above the stated %.4f budget",
					relative,
					test.wantRelativeAt,
				)
			}

			t.Logf("default-grid deviation = %.4f of peak", relative)
		})
	}
}

// TestMinimumPhaseAliasingShrinksWithTheGrid is the property that makes the
// budget above a dial rather than a limit.
func TestMinimumPhaseAliasingShrinksWithTheGrid(t *testing.T) {
	prototype := []float64{1, -1}

	converged, err := MinimumPhase(prototype, denseGrid)
	if err != nil {
		t.Fatalf("MinimumPhase(dense) error = %v", err)
	}

	previous := math.Inf(1)

	for _, size := range []int{64, 256, 1024, 4096, 16384} {
		taps, designErr := MinimumPhase(prototype, size)
		if designErr != nil {
			t.Fatalf("MinimumPhase(%d) error = %v", size, designErr)
		}

		deviation := worstDeviation(taps, converged)
		if deviation >= previous {
			t.Errorf(
				"grid %d deviation %.6g did not improve on the previous %.6g",
				size,
				deviation,
				previous,
			)
		}

		previous = deviation
	}
}

// TestMinimumPhaseInverseStability records that the returned filter is only
// approximately minimum phase, in the terms a caller actually feels.
//
// The reconstruction is minimum phase on the dense grid, but it is then
// truncated to the prototype length, and truncation can push zeros just outside
// the unit circle. The practical consequence is that inverting the result is
// not guaranteed stable: the inverse recursion grows like the largest zero
// magnitude raised to the sample index. A denser grid shrinks the excursion.
func TestMinimumPhaseInverseStability(t *testing.T) {
	prototype := lowpassPrototype(63, 0.1)

	previous := math.Inf(1)

	for _, size := range []int{512, 8192, 131072} {
		taps, err := MinimumPhase(prototype, size)
		if err != nil {
			t.Fatalf("MinimumPhase(%d) error = %v", size, err)
		}

		growth := inverseGrowth(taps, 4000)
		t.Logf("grid %d: inverse response peaks at %.6g", size, growth)

		if growth > previous {
			t.Errorf(
				"grid %d inverse growth %.6g is worse than the coarser grid's %.6g",
				size,
				growth,
				previous,
			)
		}

		previous = growth
	}
}

// inverseGrowth runs the inverse recursion of taps on a unit impulse and
// reports the largest magnitude reached. A genuinely minimum-phase filter keeps
// this bounded; a zero outside the unit circle makes it grow geometrically.
func inverseGrowth(taps []float64, samples int) float64 {
	if taps[0] == 0 {
		return math.Inf(1)
	}

	history := make([]float64, samples)
	largest := 0.0

	for n := range samples {
		accumulated := 0.0
		if n == 0 {
			accumulated = 1
		}

		for k := 1; k < len(taps) && k <= n; k++ {
			accumulated -= taps[k] * history[n-k]
		}

		history[n] = accumulated / taps[0]
		largest = math.Max(largest, math.Abs(history[n]))
	}

	return largest
}
