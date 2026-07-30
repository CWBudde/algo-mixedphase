package mixedphase_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-mixedphase/mixedphase"
)

func ExampleMinimumPhaseWith() {
	// A symmetric, linear-phase prototype whose energy sits in the middle.
	prototype := []float64{
		0.01, 0.04, 0.12, 0.20, 0.26, 0.20, 0.12, 0.04, 0.01,
	}

	// The discrete Hilbert transform reproduces the target magnitude exactly on
	// the design grid; the real cepstrum recovers it through an exponential.
	taps, err := mixedphase.MinimumPhaseWith(
		prototype,
		mixedphase.MinimumPhaseConfig{Method: mixedphase.MethodHilbert},
	)
	if err != nil {
		panic(err)
	}

	metrics, err := mixedphase.Analyze(prototype, taps, 0)
	if err != nil {
		panic(err)
	}

	// The minimum-phase version front-loads its energy.
	fmt.Println(len(taps))
	fmt.Println(metrics.PeakIndex)
	// Output:
	// 9
	// 2
}

func ExampleDesignComplexLeastSquares() {
	prototype := []float64{
		0.01, 0.04, 0.12, 0.20, 0.26, 0.20, 0.12, 0.04, 0.01,
	}

	// Weight the lower half of the spectrum ten times as heavily, then trade
	// mean-square error for peak error with a few Lawson passes.
	weight := make([]float64, 129)
	for i := range weight {
		weight[i] = 1
		if i < len(weight)/2 {
			weight[i] = 10
		}
	}

	// Mix is deliberately not 0.5: interpolating exactly half way between
	// minimum and linear phase flips half of this prototype's reciprocal zero
	// pairs, so the prescribed response is itself a nine-tap filter and the fit
	// is exact to rounding. Both error norms would then be pure noise and any
	// comparison between them meaningless.
	design := func(passes int) mixedphase.Result {
		result, err := mixedphase.DesignComplexLeastSquares(
			prototype,
			mixedphase.ComplexLeastSquaresConfig{
				Length:            9,
				Mix:               0.25,
				FFTSize:           256,
				Weight:            weight,
				MinimaxIterations: passes,
			},
		)
		if err != nil {
			panic(err)
		}

		return result
	}

	leastSquares := design(0)
	minimax := design(8)

	fmt.Println(len(minimax.Taps))

	// Lawson reweighting lowers the peak error it is aimed at ...
	fmt.Println(minimax.ComplexError.Peak < leastSquares.ComplexError.Peak)

	// ... by levelling the error towards equiripple, so peak approaches RMS.
	fmt.Println(minimax.ComplexError.Peak/minimax.ComplexError.RMS < 1.1)
	fmt.Println(leastSquares.ComplexError.Peak/leastSquares.ComplexError.RMS > 1.5)
	// Output:
	// 9
	// true
	// true
	// true
}

func ExampleDesignLowGroupDelay() {
	prototype := []float64{
		0.01, 0.04, 0.12, 0.20, 0.26, 0.20, 0.12, 0.04, 0.01,
	}

	// Let the magnitude move by up to 2 dB and spend that freedom on delay.
	result, err := mixedphase.DesignLowGroupDelay(
		prototype,
		mixedphase.LowGroupDelayConfig{
			FFTSize:     128,
			ToleranceDB: 2,
		},
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(len(result.Taps))

	// The linear-phase prototype delays the passband by four samples.
	fmt.Println(result.GroupDelay.Mean < 4)
	fmt.Println(result.GroupDelay.ConstraintViolation < 1e-3)
	// Output:
	// 9
	// true
	// true
}

// ExampleDesignPhaseInterpolation walks the phase continuum for one fixed
// magnitude: minimum phase, linear phase, and maximum phase.
func ExampleDesignPhaseInterpolation() {
	prototype := []float64{
		0.01, 0.04, 0.12, 0.20, 0.26, 0.20, 0.12, 0.04, 0.01,
	}

	design := func(mix float64) []float64 {
		result, err := mixedphase.DesignPhaseInterpolation(
			prototype,
			mixedphase.PhaseInterpolationConfig{Mix: mix, FFTSize: 128},
		)
		if err != nil {
			panic(err)
		}

		return result.Taps
	}

	minimum := design(0)
	linear := design(1)
	maximum := design(2)

	// Linear phase centres the peak; the two extremes place it at mirrored
	// positions either side of that centre, early for minimum phase and late
	// for maximum phase.
	fmt.Println(peakIndex(minimum), peakIndex(linear), peakIndex(maximum))

	// The continuum is symmetric about linear phase, so maximum phase is the
	// time reverse of minimum phase and buys no magnitude accuracy for the
	// latency it costs.
	reversed := true

	for i, tap := range maximum {
		if math.Abs(tap-minimum[len(minimum)-1-i]) > 1e-12 {
			reversed = false
		}
	}

	fmt.Println(reversed)
	// Output:
	// 2 4 6
	// true
}

func peakIndex(taps []float64) int {
	best := 0
	for i, tap := range taps {
		if math.Abs(tap) > math.Abs(taps[best]) {
			best = i
		}
	}

	return best
}

func ExampleDesignIterative() {
	// A symmetric FIR prototype; in practice this can come from any linear
	// phase design method.
	prototype := []float64{
		0.01, 0.04, 0.12, 0.20, 0.26, 0.20, 0.12, 0.04, 0.01,
	}

	result, err := mixedphase.DesignIterative(
		prototype,
		mixedphase.IterativeConfig{
			Length: 9,
			Delay:  2,
		},
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(len(result.Taps))
	fmt.Println(len(result.MinimumPhasePart), len(result.LinearPhasePart))
	// Output:
	// 9
	// 5 5
}

// ExampleDesignIterativeAuto shows the delay budget being chosen rather than
// supplied.
//
// This prototype is a nine-tap symmetric FIR designed into nine taps, so it is
// reproduced exactly at both ends of the split: the largest admissible budget
// gives the linear-phase factor the whole filter, and a zero budget gives the
// minimum-phase factor the whole filter. Both reach 0.000 dB, so no budget earns
// its latency and the search declines, returning the zero-delay design with a
// unit-impulse linear factor. That is the property that makes this entry point
// safe to reach for: it spends delay only when the delay buys accuracy.
func ExampleDesignIterativeAuto() {
	prototype := []float64{
		0.01, 0.04, 0.12, 0.20, 0.26, 0.20, 0.12, 0.04, 0.01,
	}

	result, err := mixedphase.DesignIterativeAuto(
		prototype,
		mixedphase.AutoIterativeConfig{Length: 9},
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(result.Delay)
	fmt.Println(len(result.MinimumPhasePart), len(result.LinearPhasePart))
	fmt.Printf("%.3f dB\n", result.Metrics.RMSMagnitudeErrorDB)
	// Output:
	// 0
	// 9 1
	// 0.000 dB
}
