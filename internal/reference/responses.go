package reference

import (
	"fmt"
	"math"
	"math/cmplx"

	algofft "github.com/cwbudde/algo-fft"
)

// ResponseTargets are the targets whose realised frequency responses are
// published, in written order. DegenerateContrastTarget accompanies the
// representative one because the alternating factorisation is inert on the
// latter: comparing the two is what distinguishes the method from a delayed
// minimum-phase filter.
func ResponseTargets() []string {
	return []string{RepresentativeTarget, DegenerateContrastTarget}
}

// ImpulseTargets are the targets whose peak-aligned impulse responses are
// published, in written order.
func ImpulseTargets() []string {
	return []string{ImpulseTarget, DegenerateContrastTarget}
}

// RepresentativeResponses designs every general method for ResponseTargets and
// ImpulseTargets, then samples the realised frequency and impulse responses
// used by the paper.
func RepresentativeResponses() (
	[]FrequencyResponseRow,
	[]ImpulseResponseRow,
	error,
) {
	targets, err := Targets()
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

	frequencyRows := make(
		[]FrequencyResponseRow,
		0,
		len(ResponseTargets())*len(methods())*(FFTSize/2+1),
	)

	for _, name := range ResponseTargets() {
		target, findErr := findTarget(targets, name)
		if findErr != nil {
			return nil, nil, findErr
		}

		frequencyRows, err = appendFrequencyRows(plan, target, frequencyRows)
		if err != nil {
			return nil, nil, err
		}
	}

	impulseRows := make(
		[]ImpulseResponseRow,
		0,
		len(ImpulseTargets())*len(methods())*TapCount,
	)

	for _, name := range ImpulseTargets() {
		target, findErr := findTarget(targets, name)
		if findErr != nil {
			return nil, nil, findErr
		}

		impulseRows, err = appendImpulseRows(target, impulseRows)
		if err != nil {
			return nil, nil, err
		}
	}

	return frequencyRows, impulseRows, nil
}

func appendFrequencyRows(
	plan *algofft.Plan[complex128],
	target Target,
	rows []FrequencyResponseRow,
) ([]FrequencyResponseRow, error) {
	targetSpectrum, err := realSpectrum(plan, target.Prototype)
	if err != nil {
		return nil, fmt.Errorf(
			"reference: transform representative target %s: %w",
			target.Name,
			err,
		)
	}

	for _, method := range methods() {
		result, designErr := method.design(target)
		if designErr != nil {
			return nil, fmt.Errorf(
				"reference: design representative %s/%s: %w",
				target.Name,
				method.name,
				designErr,
			)
		}

		spectrum, transformErr := realSpectrum(plan, result.Taps)
		if transformErr != nil {
			return nil, fmt.Errorf(
				"reference: transform representative %s/%s: %w",
				target.Name,
				method.name,
				transformErr,
			)
		}

		for bin := range FFTSize/2 + 1 {
			rows = append(rows, FrequencyResponseRow{
				Target:            target.Name,
				Method:            method.name,
				SampleRate:        SampleRate,
				Taps:              len(result.Taps),
				FFTSize:           FFTSize,
				DelayBudget:       method.reportedDelay(result),
				FrequencyHz:       float64(bin) * SampleRate / FFTSize,
				TargetMagnitudeDB: magnitudeDB(targetSpectrum[bin]),
				MagnitudeDB:       magnitudeDB(spectrum[bin]),
				GroupDelay:        groupDelayAt(result.Taps, spectrum[bin], bin),
				DelayWeight:       target.DelayWeight[bin],
			})
		}
	}

	return rows, nil
}

func appendImpulseRows(
	target Target,
	rows []ImpulseResponseRow,
) ([]ImpulseResponseRow, error) {
	for _, method := range methods() {
		result, designErr := method.design(target)
		if designErr != nil {
			return nil, fmt.Errorf(
				"reference: design impulse %s/%s: %w",
				target.Name,
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

			rows = append(rows, ImpulseResponseRow{
				Target:                target.Name,
				Method:                method.name,
				SampleRate:            SampleRate,
				Taps:                  len(result.Taps),
				FFTSize:               FFTSize,
				DelayBudget:           method.reportedDelay(result),
				SampleIndex:           sample,
				PeakIndex:             result.Metrics.PeakIndex,
				PeakAlignedIndex:      sample - result.Metrics.PeakIndex,
				Coefficient:           coefficient,
				NormalisedCoefficient: normalised,
			})
		}
	}

	return rows, nil
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
