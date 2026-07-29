package mixedphase

import (
	"errors"
	"math"
	"testing"
)

func nonFinitePrototype(bad float64) []float64 {
	prototype := lowpassPrototype(33, 0.2)
	prototype[7] = bad

	return prototype
}

// TestEntryPointsRejectNonFinitePrototypes pins the behaviour that used to be
// the package's quietest failure: a single NaN tap poisoned the scale-relative
// magnitude floor, every later comparison against it was false, and each design
// returned a full slice of NaN taps with a nil error.
func TestEntryPointsRejectNonFinitePrototypes(t *testing.T) {
	bad := []struct {
		name  string
		value float64
	}{
		{name: "NaN", value: math.NaN()},
		{name: "positive infinity", value: math.Inf(1)},
		{name: "negative infinity", value: math.Inf(-1)},
	}

	designs := []struct {
		name string
		call func([]float64) error
	}{
		{
			name: "MinimumPhase",
			call: func(p []float64) error { _, err := MinimumPhase(p, 0); return err },
		},
		{
			name: "MinimumPhaseWith",
			call: func(p []float64) error {
				_, err := MinimumPhaseWith(p, MinimumPhaseConfig{})

				return err
			},
		},
		{
			name: "DesignIterative",
			call: func(p []float64) error {
				_, err := DesignIterative(p, IterativeConfig{Delay: 4})

				return err
			},
		},
		{
			name: "DesignPhaseInterpolation",
			call: func(p []float64) error {
				_, err := DesignPhaseInterpolation(p, PhaseInterpolationConfig{Mix: 0.5})

				return err
			},
		},
		{
			name: "DesignComplexLeastSquares",
			call: func(p []float64) error {
				_, err := DesignComplexLeastSquares(p, ComplexLeastSquaresConfig{Mix: 0.5})

				return err
			},
		},
		{
			name: "DesignLowGroupDelay",
			call: func(p []float64) error {
				_, err := DesignLowGroupDelay(p, LowGroupDelayConfig{ToleranceDB: 1})

				return err
			},
		},
		{
			name: "Analyze reference",
			call: func(p []float64) error {
				_, err := Analyze(p, lowpassPrototype(33, 0.2), 0)

				return err
			},
		},
		{
			name: "Analyze candidate",
			call: func(p []float64) error {
				_, err := Analyze(lowpassPrototype(33, 0.2), p, 0)

				return err
			},
		},
	}

	for _, design := range designs {
		for _, value := range bad {
			t.Run(design.name+"/"+value.name, func(t *testing.T) {
				err := design.call(nonFinitePrototype(value.value))
				if !errors.Is(err, ErrNonFinitePrototype) {
					t.Fatalf("error = %v, want ErrNonFinitePrototype", err)
				}
			})
		}
	}
}

// TestEntryPointsRejectNaNConfiguration covers the fields guarded only by a
// range comparison. NaN is neither less than nor greater than any bound, so it
// passed every such check untouched.
func TestEntryPointsRejectNaNConfiguration(t *testing.T) {
	prototype := lowpassPrototype(33, 0.2)

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "interpolation mix",
			call: func() error {
				_, err := DesignPhaseInterpolation(
					prototype,
					PhaseInterpolationConfig{Mix: math.NaN()},
				)

				return err
			},
		},
		{
			name: "least squares mix",
			call: func() error {
				_, err := DesignComplexLeastSquares(
					prototype,
					ComplexLeastSquaresConfig{Mix: math.NaN()},
				)

				return err
			},
		},
		{
			name: "iterative epsilon",
			call: func() error {
				_, err := DesignIterative(
					prototype,
					IterativeConfig{Delay: 4, Epsilon: math.NaN()},
				)

				return err
			},
		},
		{
			name: "iterative window alpha",
			call: func() error {
				_, err := DesignIterative(
					prototype,
					IterativeConfig{Delay: 4, WindowAlpha: math.NaN()},
				)

				return err
			},
		},
		{
			name: "iterative tolerance",
			call: func() error {
				_, err := DesignIterative(
					prototype,
					IterativeConfig{Delay: 4, ToleranceDB: math.NaN()},
				)

				return err
			},
		},
		{
			name: "low delay tolerance",
			call: func() error {
				_, err := DesignLowGroupDelay(
					prototype,
					LowGroupDelayConfig{ToleranceDB: math.NaN()},
				)

				return err
			},
		},
		{
			name: "minimum phase epsilon",
			call: func() error {
				_, err := MinimumPhaseWith(
					prototype,
					MinimumPhaseConfig{Epsilon: math.NaN()},
				)

				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, ErrNonFiniteConfig) {
				t.Fatalf("error = %v, want ErrNonFiniteConfig", err)
			}
		})
	}
}

// TestDesignGridMustOversampleTheFilter pins the rule that keeps the reported
// metrics meaningful. On a grid equal to the filter length the projection is
// exact at every grid point by construction, so the error columns read zero
// however the filter behaves between the points.
func TestDesignGridMustOversampleTheFilter(t *testing.T) {
	prototype := lowpassPrototype(33, 0.2)

	tests := []struct {
		name    string
		fftSize int
		wantErr bool
	}{
		{name: "equal to the prototype", fftSize: 33, wantErr: true},
		{name: "below twice the prototype", fftSize: 64, wantErr: true},
		{name: "odd", fftSize: 67, wantErr: true},
		{name: "exactly twice", fftSize: 66, wantErr: false},
		{name: "generous", fftSize: 512, wantErr: false},
		{name: "default", fftSize: 0, wantErr: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DesignPhaseInterpolation(
				prototype,
				PhaseInterpolationConfig{Mix: 0.5, FFTSize: test.fftSize},
			)

			switch {
			case test.wantErr && !errors.Is(err, ErrInvalidLength):
				t.Fatalf("error = %v, want ErrInvalidLength", err)
			case !test.wantErr && err != nil:
				t.Fatalf("unexpected error = %v", err)
			}
		})
	}
}
