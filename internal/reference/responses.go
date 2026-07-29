package reference

import (
	"fmt"
	"math"
	"math/cmplx"

	algofft "github.com/cwbudde/algo-fft"
)

// RepresentativeResponses designs every general method for
// RepresentativeTarget and ImpulseTarget, then samples the realised frequency
// and impulse responses used by the paper.
func RepresentativeResponses() (
	[]FrequencyResponseRow,
	[]ImpulseResponseRow,
	error,
) {
	targets, err := Targets()
	if err != nil {
		return nil, nil, err
	}

	responseTarget, err := findTarget(targets, RepresentativeTarget)
	if err != nil {
		return nil, nil, err
	}

	impulseTarget, err := findTarget(targets, ImpulseTarget)
	if err != nil {
		return nil, nil, err
	}

	plan, err := algofft.NewPlan64(FFTSize)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"reference: create response FFT plan: %w",
			err,
		)
	}

	targetSpectrum, err := realSpectrum(plan, responseTarget.Prototype)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"reference: transform representative target: %w",
			err,
		)
	}

	frequencyRows := make(
		[]FrequencyResponseRow,
		0,
		len(methods())*(FFTSize/2+1),
	)
	impulseRows := make(
		[]ImpulseResponseRow,
		0,
		len(methods())*TapCount,
	)

	for _, method := range methods() {
		result, designErr := method.design(responseTarget)
		if designErr != nil {
			return nil, nil, fmt.Errorf(
				"reference: design representative %s/%s: %w",
				responseTarget.Name,
				method.name,
				designErr,
			)
		}

		spectrum, transformErr := realSpectrum(plan, result.Taps)
		if transformErr != nil {
			return nil, nil, fmt.Errorf(
				"reference: transform representative %s/%s: %w",
				responseTarget.Name,
				method.name,
				transformErr,
			)
		}

		for bin := range FFTSize/2 + 1 {
			frequencyRows = append(frequencyRows, FrequencyResponseRow{
				Target:            responseTarget.Name,
				Method:            method.name,
				SampleRate:        SampleRate,
				Taps:              len(result.Taps),
				FFTSize:           FFTSize,
				DelayBudget:       DelayBudget,
				FrequencyHz:       float64(bin) * SampleRate / FFTSize,
				TargetMagnitudeDB: magnitudeDB(targetSpectrum[bin]),
				MagnitudeDB:       magnitudeDB(spectrum[bin]),
				GroupDelay:        groupDelayAt(result.Taps, spectrum[bin], bin),
				DelayWeight:       responseTarget.DelayWeight[bin],
			})
		}
	}

	for _, method := range methods() {
		result, designErr := method.design(impulseTarget)
		if designErr != nil {
			return nil, nil, fmt.Errorf(
				"reference: design impulse %s/%s: %w",
				impulseTarget.Name,
				method.name,
				designErr,
			)
		}

		peak := 0.0
		for _, coefficient := range result.Taps {
			peak = max(peak, math.Abs(coefficient))
		}

		for sample, coefficient := range result.Taps {
			normalised := 0.0
			if peak > 0 {
				normalised = coefficient / peak
			}

			impulseRows = append(impulseRows, ImpulseResponseRow{
				Target:                impulseTarget.Name,
				Method:                method.name,
				SampleRate:            SampleRate,
				Taps:                  len(result.Taps),
				FFTSize:               FFTSize,
				DelayBudget:           DelayBudget,
				SampleIndex:           sample,
				PeakIndex:             result.Metrics.PeakIndex,
				PeakAlignedIndex:      sample - result.Metrics.PeakIndex,
				Coefficient:           coefficient,
				NormalisedCoefficient: normalised,
			})
		}
	}

	return frequencyRows, impulseRows, nil
}

func findTarget(targets []Target, name string) (Target, error) {
	for _, target := range targets {
		if target.Name == name {
			return target, nil
		}
	}

	return Target{}, fmt.Errorf("reference: target %q is missing", name)
}

func realSpectrum(
	plan *algofft.Plan[complex128],
	values []float64,
) ([]complex128, error) {
	// Zero-padding only makes sense while the signal fits the grid. Truncating
	// silently would misreport the response of exactly the long filters the
	// suite is meant to measure, so refuse instead.
	if len(values) > FFTSize {
		return nil, fmt.Errorf(
			"reference: %d samples do not fit the %d-point grid",
			len(values),
			FFTSize,
		)
	}

	input := make([]complex128, FFTSize)
	for index, value := range values {
		input[index] = complex(value, 0)
	}

	output := make([]complex128, FFTSize)
	if err := plan.Forward(output, input); err != nil {
		return nil, err
	}

	return output, nil
}

func magnitudeDB(value complex128) float64 {
	return 20 * math.Log10(max(cmplx.Abs(value), coefficientFloor))
}

func groupDelayAt(
	taps []float64,
	response complex128,
	bin int,
) float64 {
	omega := 2 * math.Pi * float64(bin) / FFTSize
	slope := complex(0, 0)

	for index, tap := range taps {
		angle := omega * float64(index)
		slope += complex(
			float64(index)*tap*math.Cos(angle),
			-float64(index)*tap*math.Sin(angle),
		)
	}

	denominator := cmplx.Abs(response)
	denominator *= denominator

	if denominator < coefficientFloor {
		return 0
	}

	return (real(response)*real(slope) +
		imag(response)*imag(slope)) / denominator
}
