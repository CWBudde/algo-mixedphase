package mixedphase

import (
	"errors"
	"math"
	"slices"
	"testing"
)

// starvedPrototype returns a prototype whose minimum-phase response cannot fit
// the support the split allocates at starvedLength.
//
// This is the input class DesignIterativeAuto exists for: a 257-tap prototype
// with a 0.02 cutoff needs far more than 65 taps, so truncating its
// minimum-phase factor destroys the response and the linear factor has real work
// to do. The lab's own 129-tap fixture is not starved and is used below for the
// opposite case.
func starvedPrototype() []float64 {
	return lowpassPrototype(257, 0.02)
}

const starvedLength = 65

func TestDesignIterativeAutoValidation(t *testing.T) {
	cases := []struct {
		name      string
		prototype []float64
		cfg       AutoIterativeConfig
		want      error
	}{
		{
			name:      "empty prototype",
			prototype: nil,
			want:      ErrEmptyPrototype,
		},
		{
			name:      "non-finite prototype",
			prototype: []float64{1, math.NaN(), 1},
			want:      ErrNonFinitePrototype,
		},
		{
			name:      "negative length",
			prototype: lowpassPrototype(65, 0.1),
			cfg:       AutoIterativeConfig{Length: -1},
			want:      ErrInvalidLength,
		},
		{
			name:      "negative maximum delay",
			prototype: lowpassPrototype(65, 0.1),
			cfg:       AutoIterativeConfig{MaxDelay: -1},
			want:      ErrInvalidDelay,
		},
		{
			name:      "negative coarse step",
			prototype: lowpassPrototype(65, 0.1),
			cfg:       AutoIterativeConfig{CoarseStep: -2},
			want:      ErrInvalidLength,
		},
		{
			name:      "non-finite slack",
			prototype: lowpassPrototype(65, 0.1),
			cfg:       AutoIterativeConfig{RelativeErrorSlack: math.NaN()},
			want:      ErrNonFiniteConfig,
		},
		{
			name:      "negative epsilon reaches the inner design",
			prototype: lowpassPrototype(65, 0.1),
			cfg:       AutoIterativeConfig{Epsilon: -1},
			want:      ErrInvalidEpsilon,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := DesignIterativeAuto(testCase.prototype, testCase.cfg)
			if !errors.Is(err, testCase.want) {
				t.Errorf("error = %v, want %v", err, testCase.want)
			}
		})
	}
}

// TestDesignIterativeAutoSelectsZeroWhenDelayBuysNothing is the guarantee that
// makes this designer safe to reach for.
//
// [DesignIterative] with a hand-picked budget spends latency on a target whose
// minimum-phase factor already fits, and returns a delayed minimum-phase filter
// for it. Here the search must notice and decline, returning bit-for-bit the
// design a zero budget produces rather than something merely close to it.
func TestDesignIterativeAutoSelectsZeroWhenDelayBuysNothing(t *testing.T) {
	prototype := lowpassPrototype(129, 0.08)

	auto, err := DesignIterativeAuto(prototype, AutoIterativeConfig{Length: 129})
	if err != nil {
		t.Fatalf("DesignIterativeAuto() error = %v", err)
	}

	if auto.Delay != 0 {
		t.Errorf("selected delay = %d, want 0 on a target that does not need one", auto.Delay)
	}

	if !slices.Equal(auto.LinearPhasePart, []float64{1}) {
		t.Errorf("linear factor = %v, want a unit impulse", auto.LinearPhasePart)
	}

	base, err := DesignIterative(prototype, IterativeConfig{Length: 129, Delay: 0})
	if err != nil {
		t.Fatalf("DesignIterative() error = %v", err)
	}

	if !slices.Equal(auto.Taps, base.Taps) {
		t.Error("selecting a zero budget did not reproduce the minimum-phase design exactly")
	}
}

// TestDesignIterativeAutoFindsTheStarvedOptimum pins the case the method is for.
//
// Budget: a 257-tap 0.02-cutoff prototype designed into 65 taps on the default
// grid with the default twelve correction passes and default coarse step.
func TestDesignIterativeAutoFindsTheStarvedOptimum(t *testing.T) {
	prototype := starvedPrototype()

	auto, err := DesignIterativeAuto(prototype, AutoIterativeConfig{Length: starvedLength})
	if err != nil {
		t.Fatalf("DesignIterativeAuto() error = %v", err)
	}

	if auto.Delay == 0 {
		t.Fatal("selected a zero budget on a support-starved target")
	}

	if got, want := auto.Delay, 24; got != want {
		t.Errorf("selected delay = %d, want %d", got, want)
	}

	if len(auto.LinearPhasePart) != 2*auto.Delay+1 {
		t.Errorf(
			"linear factor has %d taps, want %d for delay %d",
			len(auto.LinearPhasePart),
			2*auto.Delay+1,
			auto.Delay,
		)
	}

	base, err := DesignIterative(prototype, IterativeConfig{Length: starvedLength, Delay: 0})
	if err != nil {
		t.Fatalf("DesignIterative() error = %v", err)
	}

	improvement := base.Metrics.RMSMagnitudeErrorDB - auto.Metrics.RMSMagnitudeErrorDB
	if improvement < 40 {
		t.Errorf(
			"RMS error improved by only %.3f dB (%.3f -> %.3f); the search has "+
				"stopped finding the delay this target needs",
			improvement,
			base.Metrics.RMSMagnitudeErrorDB,
			auto.Metrics.RMSMagnitudeErrorDB,
		)
	}
}

// TestDesignIterativeAutoNeverLosesToMinimumPhase is the invariant the whole
// design rests on: because a zero budget is always evaluated and always
// admissible, no input can make the result worse than minimum-phase truncation
// on the objective.
func TestDesignIterativeAutoNeverLosesToMinimumPhase(t *testing.T) {
	cases := []struct {
		name      string
		prototype []float64
		length    int
	}{
		{"fits its support", lowpassPrototype(129, 0.08), 129},
		{"narrow cutoff", lowpassPrototype(129, 0.02), 129},
		{"wide cutoff", lowpassPrototype(129, 0.4), 129},
		{"support starved", starvedPrototype(), starvedLength},
		{"severely starved", lowpassPrototype(513, 0.01), 65},
		{"prototype length used", lowpassPrototype(65, 0.1), 0},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			auto, err := DesignIterativeAuto(testCase.prototype, AutoIterativeConfig{
				Length: testCase.length,
			})
			if err != nil {
				t.Fatalf("DesignIterativeAuto() error = %v", err)
			}

			base, err := DesignIterative(testCase.prototype, IterativeConfig{
				Length: testCase.length,
				Delay:  0,
			})
			if err != nil {
				t.Fatalf("DesignIterative() error = %v", err)
			}

			if auto.Metrics.RMSMagnitudeErrorDB > base.Metrics.RMSMagnitudeErrorDB {
				t.Errorf(
					"selected %.6f dB at delay %d, worse than %.6f dB at delay 0",
					auto.Metrics.RMSMagnitudeErrorDB,
					auto.Delay,
					base.Metrics.RMSMagnitudeErrorDB,
				)
			}
		})
	}
}

// TestDesignIterativeAutoSlackBoundsPassbandRegression checks the guard actually
// binds. Deep-stopband delays cost linear-magnitude accuracy, and the slack is
// the caller's cap on how much.
func TestDesignIterativeAutoSlackBoundsPassbandRegression(t *testing.T) {
	prototype := starvedPrototype()

	base, err := DesignIterative(prototype, IterativeConfig{Length: starvedLength, Delay: 0})
	if err != nil {
		t.Fatalf("DesignIterative() error = %v", err)
	}

	for _, slack := range []float64{0.1, 0.5, 1, 2, 8} {
		auto, designErr := DesignIterativeAuto(prototype, AutoIterativeConfig{
			Length:             starvedLength,
			RelativeErrorSlack: slack,
		})
		if designErr != nil {
			t.Fatalf("slack %g: DesignIterativeAuto() error = %v", slack, designErr)
		}

		ceiling := (1 + slack) * base.Metrics.RelativeMagnitudeError
		if auto.Metrics.RelativeMagnitudeError > ceiling {
			t.Errorf(
				"slack %g: relative error %g exceeds the ceiling %g",
				slack,
				auto.Metrics.RelativeMagnitudeError,
				ceiling,
			)
		}
	}
}

// TestDesignIterativeAutoExhaustiveSearchIsNeverWorse documents that the default
// scan is a heuristic. A unit coarse step visits every admissible delay, so it
// can only match or beat the strided scan — and on the severely starved fixture
// it does beat it, which is why the doc comment refuses to call the default
// exact.
func TestDesignIterativeAutoExhaustiveSearchIsNeverWorse(t *testing.T) {
	cases := []struct {
		name      string
		prototype []float64
		length    int
	}{
		{"fits its support", lowpassPrototype(129, 0.08), 129},
		{"support starved", starvedPrototype(), starvedLength},
		{"severely starved", lowpassPrototype(513, 0.005), 129},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			strided, err := DesignIterativeAuto(testCase.prototype, AutoIterativeConfig{
				Length: testCase.length,
			})
			if err != nil {
				t.Fatalf("DesignIterativeAuto() error = %v", err)
			}

			exhaustive, err := DesignIterativeAuto(testCase.prototype, AutoIterativeConfig{
				Length:     testCase.length,
				CoarseStep: 1,
			})
			if err != nil {
				t.Fatalf("DesignIterativeAuto() error = %v", err)
			}

			if exhaustive.Metrics.RMSMagnitudeErrorDB >
				strided.Metrics.RMSMagnitudeErrorDB {
				t.Errorf(
					"exhaustive search found %.6f dB at delay %d, worse than the "+
						"strided scan's %.6f dB at delay %d",
					exhaustive.Metrics.RMSMagnitudeErrorDB,
					exhaustive.Delay,
					strided.Metrics.RMSMagnitudeErrorDB,
					strided.Delay,
				)
			}
		})
	}
}

func TestDesignIterativeAutoRespectsMaxDelay(t *testing.T) {
	prototype := starvedPrototype()

	for _, bound := range []int{1, 4, 8, 16} {
		auto, err := DesignIterativeAuto(prototype, AutoIterativeConfig{
			Length:   starvedLength,
			MaxDelay: bound,
		})
		if err != nil {
			t.Fatalf("bound %d: DesignIterativeAuto() error = %v", bound, err)
		}

		if auto.Delay > bound {
			t.Errorf("bound %d: selected delay %d", bound, auto.Delay)
		}
	}

	// A bound above what the split admits must clamp rather than fail.
	auto, err := DesignIterativeAuto(prototype, AutoIterativeConfig{
		Length:   starvedLength,
		MaxDelay: 4096,
	})
	if err != nil {
		t.Fatalf("DesignIterativeAuto() error = %v", err)
	}

	if maximum := (starvedLength - 1) / 2; auto.Delay > maximum {
		t.Errorf("selected delay %d exceeds the %d the split admits", auto.Delay, maximum)
	}
}
