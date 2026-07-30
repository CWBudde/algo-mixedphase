package reference

import (
	"fmt"
	"math"
	"math/cmplx"

	algofft "github.com/cwbudde/algo-fft"
)

const coefficientFloor = 1e-12

type responseAnalysis struct {
	meanGroupDelay     float64
	groupDelayRipple   float64
	peakGroupDelay     float64
	coefficientPeak    float64
	coefficientRangeDB float64
}

func analyze(target Target, taps []float64) (responseAnalysis, error) {
	return analyzeOn(target, taps, FFTSize)
}

// analyzeOn is analyze on an explicit grid, which must be the grid
// target.DelayWeight was built for.
func analyzeOn(
	target Target,
	taps []float64,
	gridSize int,
) (responseAnalysis, error) {
	if len(target.DelayWeight) != gridSize/2+1 {
		return responseAnalysis{}, fmt.Errorf(
			"delay weight has %d bins, want %d for a %d-point grid",
			len(target.DelayWeight),
			gridSize/2+1,
			gridSize,
		)
	}

	plan, err := algofft.NewPlan64(gridSize)
	if err != nil {
		return responseAnalysis{}, fmt.Errorf(
			"create analysis FFT plan: %w",
			err,
		)
	}

	input := make([]complex128, gridSize)
	for index, tap := range taps {
		input[index] = complex(tap, 0)
	}

	spectrum := make([]complex128, gridSize)
	if err := plan.Forward(spectrum, input); err != nil {
		return responseAnalysis{}, fmt.Errorf("forward analysis FFT: %w", err)
	}

	delays := make([]float64, len(target.DelayWeight))
	valid := make([]bool, len(target.DelayWeight))
	totalWeight := 0.0
	weightedDelay := 0.0

	for bin, weight := range target.DelayWeight {
		if weight == 0 {
			continue
		}

		omega := 2 * math.Pi * float64(bin) / float64(gridSize)
		slope := complex(0, 0)

		for index, tap := range taps {
			angle := omega * float64(index)
			slope += complex(
				float64(index)*tap*math.Cos(angle),
				-float64(index)*tap*math.Sin(angle),
			)
		}

		denominator := cmplx.Abs(spectrum[bin])

		denominator *= denominator
		if denominator < coefficientFloor {
			continue
		}

		response := spectrum[bin]
		delay := (real(response)*real(slope) +
			imag(response)*imag(slope)) / denominator
		delays[bin] = delay
		valid[bin] = true
		totalWeight += weight
		weightedDelay += weight * delay
	}

	if totalWeight == 0 {
		return responseAnalysis{}, fmt.Errorf(
			"group-delay band has no usable response bins",
		)
	}

	mean := weightedDelay / totalWeight
	squaredRipple := 0.0
	peakDelay := math.Inf(-1)

	for bin, weight := range target.DelayWeight {
		if weight == 0 || !valid[bin] {
			continue
		}

		deviation := delays[bin] - mean
		squaredRipple += weight * deviation * deviation
		peakDelay = max(peakDelay, delays[bin])
	}

	coefficientPeak, coefficientMinimum := 0.0, math.Inf(1)

	for _, tap := range taps {
		absolute := math.Abs(tap)
		coefficientPeak = max(coefficientPeak, absolute)
	}

	threshold := coefficientPeak * coefficientFloor

	for _, tap := range taps {
		absolute := math.Abs(tap)
		if absolute >= threshold {
			coefficientMinimum = min(coefficientMinimum, absolute)
		}
	}

	coefficientRangeDB := 0.0
	if coefficientMinimum > 0 && !math.IsInf(coefficientMinimum, 1) {
		coefficientRangeDB = 20 * math.Log10(
			coefficientPeak/coefficientMinimum,
		)
	}

	return responseAnalysis{
		meanGroupDelay:     mean,
		groupDelayRipple:   math.Sqrt(squaredRipple / totalWeight),
		peakGroupDelay:     peakDelay,
		coefficientPeak:    coefficientPeak,
		coefficientRangeDB: coefficientRangeDB,
	}, nil
}
