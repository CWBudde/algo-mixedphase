package reference

import (
	"fmt"
	"time"

	"github.com/cwbudde/algo-mixedphase/mixedphase"
)

type designMethod struct {
	name   string
	design func(Target) (mixedphase.Result, error)
}

// Run executes every general mixed-phase method against every reference
// target. trials controls runtime measurement: zero executes once without
// timing; a positive value reports the fastest of that many complete designs.
func Run(trials int) ([]Row, error) {
	if trials < 0 {
		return nil, fmt.Errorf("reference: trials must not be negative")
	}

	targets, err := Targets()
	if err != nil {
		return nil, err
	}

	methods := methods()
	rows := make([]Row, 0, len(targets)*len(methods))

	for _, target := range targets {
		for _, method := range methods {
			result, runtime, designErr := runDesign(method, target, trials)
			if designErr != nil {
				return nil, fmt.Errorf(
					"reference: design %s/%s: %w",
					target.Name,
					method.name,
					designErr,
				)
			}

			analysis, analyzeErr := analyze(target, result.Taps)
			if analyzeErr != nil {
				return nil, fmt.Errorf(
					"reference: analyze %s/%s: %w",
					target.Name,
					method.name,
					analyzeErr,
				)
			}

			rows = append(rows, rowFromResult(
				target.Name,
				method.name,
				result,
				runtime,
				analysis,
			))
		}
	}

	return rows, nil
}

func methods() []designMethod {
	mix := float64(DelayBudget) / float64((TapCount-1)/2)

	return []designMethod{
		{
			name: "budde-iterative",
			design: func(target Target) (mixedphase.Result, error) {
				return mixedphase.DesignIterative(
					target.Prototype,
					mixedphase.IterativeConfig{
						Length:     TapCount,
						Delay:      DelayBudget,
						Iterations: iterativePasses,
						FFTSize:    FFTSize,
					},
				)
			},
		},
		{
			name: "phase-interpolation",
			design: func(target Target) (mixedphase.Result, error) {
				return mixedphase.DesignPhaseInterpolation(
					target.Prototype,
					mixedphase.PhaseInterpolationConfig{
						Length:  TapCount,
						Mix:     mix,
						FFTSize: FFTSize,
					},
				)
			},
		},
		{
			name: "complex-minimax",
			design: func(target Target) (mixedphase.Result, error) {
				return mixedphase.DesignComplexLeastSquares(
					target.Prototype,
					mixedphase.ComplexLeastSquaresConfig{
						Length:            TapCount,
						Mix:               mix,
						FFTSize:           FFTSize,
						MinimaxIterations: minimaxPasses,
					},
				)
			},
		},
		{
			name: "low-group-delay",
			design: func(target Target) (mixedphase.Result, error) {
				return mixedphase.DesignLowGroupDelay(
					target.Prototype,
					mixedphase.LowGroupDelayConfig{
						Length:        TapCount,
						FFTSize:       FFTSize,
						ToleranceDB:   lowDelayToleranceDB,
						DelayWeight:   target.DelayWeight,
						Iterations:    lowDelayIterations,
						PenaltyStages: lowDelayPenaltyStages,
					},
				)
			},
		},
	}
}

func runDesign(
	method designMethod,
	target Target,
	trials int,
) (mixedphase.Result, time.Duration, error) {
	if trials == 0 {
		result, err := method.design(target)

		return result, 0, err
	}

	durations := make([]time.Duration, trials)

	var result mixedphase.Result

	for trial := range trials {
		start := time.Now()

		current, err := method.design(target)
		if err != nil {
			return mixedphase.Result{}, 0, err
		}

		durations[trial] = time.Since(start)
		if trial == 0 {
			result = current
		}
	}

	best := durations[0]
	for _, duration := range durations[1:] {
		best = min(best, duration)
	}

	return result, best, nil
}

func rowFromResult(
	target, method string,
	result mixedphase.Result,
	runtime time.Duration,
	analysis responseAnalysis,
) Row {
	metrics := result.Metrics

	return Row{
		Target:                 target,
		Method:                 method,
		SampleRate:             SampleRate,
		Taps:                   len(result.Taps),
		FFTSize:                FFTSize,
		DelayBudget:            DelayBudget,
		Iterations:             result.Iterations,
		Runtime:                runtime,
		RelativeMagnitudeError: metrics.RelativeMagnitudeError,
		RMSMagnitudeErrorDB:    metrics.RMSMagnitudeErrorDB,
		MaxMagnitudeErrorDB:    metrics.MaxMagnitudeErrorDB,
		MeanGroupDelay:         analysis.meanGroupDelay,
		GroupDelayRipple:       analysis.groupDelayRipple,
		PeakGroupDelay:         analysis.peakGroupDelay,
		PeakIndex:              metrics.PeakIndex,
		EnergyCentroid:         metrics.EnergyCentroid,
		PrePeakEnergyRatio:     metrics.PrePeakEnergyRatio,
		CoefficientPeak:        analysis.coefficientPeak,
		CoefficientRangeDB:     analysis.coefficientRangeDB,
		ConstraintViolation:    result.GroupDelay.ConstraintViolation,
	}
}
