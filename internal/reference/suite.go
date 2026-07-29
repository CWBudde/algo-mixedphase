package reference

import (
	"errors"
	"fmt"
	"time"

	"github.com/cwbudde/algo-mixedphase/mixedphase"
)

// ErrInvalidTrials is returned when a negative trial count is requested.
var ErrInvalidTrials = errors.New("reference: trials must not be negative")

type designMethod struct {
	name   string
	design func(Target) (mixedphase.Result, error)

	// choosesDelay marks a design that selects its own linear-phase delay
	// budget rather than honouring the suite's. The reported delay then comes
	// from the design instead of from DelayBudget, which would otherwise put a
	// number in the published table that the design never used.
	choosesDelay bool
}

// reportedDelay is the linear-phase delay the published row should carry.
func (m designMethod) reportedDelay(result mixedphase.Result) int {
	if m.choosesDelay {
		return result.Delay
	}

	return DelayBudget
}

// Run executes every general mixed-phase method against every reference
// target. trials controls runtime measurement: zero executes once without
// timing; a positive value reports the fastest of that many complete designs.
func Run(trials int) ([]Row, error) {
	if trials < 0 {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidTrials, trials)
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
				method,
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
						Iterations: IterativePasses,
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
						MinimaxIterations: MinimaxPasses,
						// The objective is an absolute complex deviation, so an
						// unweighted design spends its whole budget on the
						// passband and lets stopband depth slip. Weighting by
						// the inverse target magnitude is what the package docs
						// prescribe for exactly this case; leaving Weight nil
						// would also make this method coincide with
						// phase-interpolation, which would leave the comparison
						// with three distinct methods rather than four.
						Weight: target.MagnitudeWeight,
					},
				)
			},
		},
		{
			// The baseline the alternating factorisation has to beat to be
			// worth its extra delay. It is DesignIterative with the delay
			// budget removed, so it isolates exactly what the linear factor
			// buys.
			name: "minphase-truncation",
			design: func(target Target) (mixedphase.Result, error) {
				return mixedphase.DesignIterative(
					target.Prototype,
					mixedphase.IterativeConfig{
						Length:     TapCount,
						Delay:      0,
						Iterations: IterativePasses,
						FFTSize:    FFTSize,
					},
				)
			},
		},
		{
			// The alternating factorisation with the delay budget as an output.
			// It is listed beside budde-iterative rather than replacing it
			// because the contrast between the two rows is the evidence: on five
			// of the six targets this one declines the budget and deliberately
			// reproduces minphase-truncation, which is exactly what the fixed
			// budget should have done and did not.
			name:         "budde-adaptive",
			choosesDelay: true,
			design: func(target Target) (mixedphase.Result, error) {
				return mixedphase.DesignIterativeAuto(
					target.Prototype,
					mixedphase.AutoIterativeConfig{
						Length:     TapCount,
						FFTSize:    FFTSize,
						Iterations: IterativePasses,
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
						ToleranceDB:   LowDelayToleranceDB,
						DelayWeight:   target.DelayWeight,
						Iterations:    LowDelayIterations,
						PenaltyStages: LowDelayPenaltyStages,
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
	target string,
	method designMethod,
	result mixedphase.Result,
	runtime time.Duration,
	analysis responseAnalysis,
) Row {
	metrics := result.Metrics

	return Row{
		Target:                 target,
		Method:                 method.name,
		SampleRate:             SampleRate,
		Taps:                   len(result.Taps),
		FFTSize:                FFTSize,
		DelayBudget:            method.reportedDelay(result),
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
