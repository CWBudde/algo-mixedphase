package reference

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/cwbudde/algo-mixedphase/mixedphase"
)

// The regimes artifact measures the two things a delay budget can be spent on,
// on either side of the group-delay floor a magnitude request implies.
//
// A causal filter realising magnitude M factors as H = H_min * A with A
// all-pass. All-pass group delay is non-negative at every frequency, so no
// realisation of M has less group delay than its minimum-phase one: that is the
// floor. Everything above it is the all-pass factor, and the choice of that
// factor is the only phase freedom a fixed magnitude leaves.
//
// Two families measure the two sides:
//
//   - "continuum" walks [mixedphase.DesignPhaseInterpolation] across the whole
//     phase continuum, minimum phase through linear phase to maximum phase. This
//     is the surplus above the floor being spent on shaping, and it is the
//     control an equaliser or room correction wants to expose.
//   - "factorisation" walks the alternating factorisation of
//     [mixedphase.DesignIterative] over the same delay range, so its answer to
//     the same question can be read against the continuum's on one axis. It is
//     the contrast that gives the other two families their meaning.
//   - "floor-probe" relaxes the magnitude tolerance of
//     [mixedphase.DesignLowGroupDelay] step by step. That optimiser wants the
//     least group delay it can get, so widening its tolerance asks directly what
//     a magnitude concession buys below the floor.
//
// Unlike the length sweep this artifact deliberately reuses the published
// fixtures and the published operating point, so every row here is comparable
// row-for-row with docs/reference-results.csv rather than being a separate
// experiment that happens to share target names.
const (
	// ContinuumMixStep is the spacing of the phase continuum scan. The range is
	// [0, MaximumPhaseMix], so this gives seventeen points per target.
	ContinuumMixStep = 0.125

	// MaximumPhaseMix is the mix at which the prescribed phase reaches maximum
	// phase. It mirrors the bound the design entry points enforce.
	MaximumPhaseMix = 2.0

	// FactorisationDelayStep is the stride of the factorisation scan over its
	// admissible budgets, [0, (TapCount-1)/2]. It is chosen so that the scan has
	// as many points as the continuum and shares its endpoints.
	FactorisationDelayStep = 4
)

// FloorProbeTolerancesDB are the magnitude tolerances the floor probe walks, in
// dB. The ladder ends at the published LowDelayToleranceDB of 2 dB so that the
// published low-group-delay row is its last point.
//
// It ends there because wider tolerances stop measuring the trade. At 4 dB the
// optimiser has not converged and its answer depends on the iteration budget
// rather than on the tolerance: the low-pass reports a mean group delay of
// 33.08, 59.82 or 2.48 samples under the published, the package-default and a
// generous budget respectively. At 8 dB the constraint is loose enough to admit
// a spectral null, and the group-delay statistic is undefined at such a bin:
// every budget returns the same -3560 samples of mean delay with the maximum
// magnitude error saturated at its 60 dB clamp. Both are pinned by
// TestLooseToleranceLeavesTheMeasurableRegime, so the ladder's endpoint is a
// recorded limit rather than a choice of presentation.
var FloorProbeTolerancesDB = []float64{0.25, 0.5, 1, 2}

// RegimeRow is one point of the phase-regimes artifact.
type RegimeRow struct {
	Target string

	// Regime is "continuum", "factorisation" or "floor-probe".
	Regime string

	SampleRate int
	Taps       int
	FFTSize    int

	// Delay is the requested budget of the factorisation family and is not
	// meaningful for the other two, whose delay is an outcome rather than an
	// input.
	Delay int

	// Mix is the prescribed phase mix for the continuum family and is not
	// meaningful for the floor probe, whose phase is free.
	Mix float64

	// ToleranceDB is the permitted magnitude deviation for the floor probe and
	// is not meaningful for the continuum family, which prescribes a complex
	// response rather than a magnitude band.
	ToleranceDB float64

	RelativeMagnitudeError float64
	RMSMagnitudeErrorDB    float64
	MaxMagnitudeErrorDB    float64
	MeanGroupDelay         float64
	GroupDelayRipple       float64

	// ConstraintViolation is the worst remaining magnitude-constraint breach of
	// the floor probe. A non-zero value means the tolerance was too tight for
	// the tap budget, so the row describes an infeasible request rather than a
	// realised trade.
	ConstraintViolation float64
}

// RegimeRows runs both regime families over the published reference targets at
// the published operating point.
func RegimeRows() ([]RegimeRow, error) {
	targets, err := Targets()
	if err != nil {
		return nil, fmt.Errorf("reference: build regime targets: %w", err)
	}

	mixes := continuumMixes()
	delays := factorisationDelays()
	rows := make(
		[]RegimeRow,
		0,
		len(targets)*(len(mixes)+len(delays)+len(FloorProbeTolerancesDB)),
	)

	for _, target := range targets {
		for _, mix := range mixes {
			row, rowErr := continuumRow(target, mix)
			if rowErr != nil {
				return nil, rowErr
			}

			rows = append(rows, row)
		}

		for _, delay := range factorisationDelays() {
			row, rowErr := factorisationRow(target, delay)
			if rowErr != nil {
				return nil, rowErr
			}

			rows = append(rows, row)
		}

		for _, tolerance := range FloorProbeTolerancesDB {
			row, rowErr := floorProbeRow(target, tolerance)
			if rowErr != nil {
				return nil, rowErr
			}

			rows = append(rows, row)
		}
	}

	return rows, nil
}

// continuumMixes lists the prescribed phase mixes the continuum family scans.
func continuumMixes() []float64 {
	count := int(MaximumPhaseMix/ContinuumMixStep) + 1

	mixes := make([]float64, count)
	for i := range mixes {
		mixes[i] = float64(i) * ContinuumMixStep
	}

	return mixes
}

// factorisationDelays lists the budgets the factorisation family scans. The
// endpoints matter: zero is the minimum-phase design and (TapCount-1)/2 spends
// the whole support on the linear factor, which is the linear-phase design.
func factorisationDelays() []int {
	maximum := (TapCount - 1) / 2

	delays := make([]int, 0, maximum/FactorisationDelayStep+1)
	for delay := 0; delay <= maximum; delay += FactorisationDelayStep {
		delays = append(delays, delay)
	}

	return delays
}

func factorisationRow(target Target, delay int) (RegimeRow, error) {
	result, err := mixedphase.DesignIterative(
		target.Prototype,
		mixedphase.IterativeConfig{
			Length:     TapCount,
			Delay:      delay,
			Iterations: IterativePasses,
			FFTSize:    FFTSize,
		},
	)
	if err != nil {
		return RegimeRow{}, fmt.Errorf(
			"reference: factorisation %s at delay %d: %w",
			target.Name,
			delay,
			err,
		)
	}

	row, err := regimeRowFrom(target, "factorisation", result)
	if err != nil {
		return RegimeRow{}, err
	}

	row.Delay = delay

	return row, nil
}

func continuumRow(target Target, mix float64) (RegimeRow, error) {
	result, err := mixedphase.DesignPhaseInterpolation(
		target.Prototype,
		mixedphase.PhaseInterpolationConfig{
			Length:  TapCount,
			Mix:     mix,
			FFTSize: FFTSize,
		},
	)
	if err != nil {
		return RegimeRow{}, fmt.Errorf(
			"reference: continuum %s at mix %g: %w",
			target.Name,
			mix,
			err,
		)
	}

	row, err := regimeRowFrom(target, "continuum", result)
	if err != nil {
		return RegimeRow{}, err
	}

	row.Mix = mix

	return row, nil
}

func floorProbeRow(target Target, toleranceDB float64) (RegimeRow, error) {
	result, err := mixedphase.DesignLowGroupDelay(
		target.Prototype,
		mixedphase.LowGroupDelayConfig{
			Length:        TapCount,
			FFTSize:       FFTSize,
			ToleranceDB:   toleranceDB,
			DelayWeight:   target.DelayWeight,
			Iterations:    LowDelayIterations,
			PenaltyStages: LowDelayPenaltyStages,
		},
	)
	if err != nil {
		return RegimeRow{}, fmt.Errorf(
			"reference: floor probe %s at %g dB: %w",
			target.Name,
			toleranceDB,
			err,
		)
	}

	row, err := regimeRowFrom(target, "floor-probe", result)
	if err != nil {
		return RegimeRow{}, err
	}

	row.ToleranceDB = toleranceDB
	row.ConstraintViolation = result.GroupDelay.ConstraintViolation

	return row, nil
}

func regimeRowFrom(
	target Target,
	regime string,
	result mixedphase.Result,
) (RegimeRow, error) {
	analysis, err := analyze(target, result.Taps)
	if err != nil {
		return RegimeRow{}, fmt.Errorf(
			"reference: analyse %s %s: %w",
			target.Name,
			regime,
			err,
		)
	}

	return RegimeRow{
		Target:                 target.Name,
		Regime:                 regime,
		SampleRate:             SampleRate,
		Taps:                   TapCount,
		FFTSize:                FFTSize,
		RelativeMagnitudeError: result.Metrics.RelativeMagnitudeError,
		RMSMagnitudeErrorDB:    result.Metrics.RMSMagnitudeErrorDB,
		MaxMagnitudeErrorDB:    result.Metrics.MaxMagnitudeErrorDB,
		MeanGroupDelay:         analysis.meanGroupDelay,
		GroupDelayRipple:       analysis.groupDelayRipple,
	}, nil
}

var regimesCSVHeader = []string{
	"target",
	"regime",
	"sample_rate_hz",
	"taps",
	"fft_size",
	"phase_mix",
	"phase_delay_samples",
	"tolerance_db",
	"relative_magnitude_error",
	"rms_magnitude_error_db",
	"max_magnitude_error_db",
	"mean_group_delay",
	"group_delay_ripple",
	"constraint_violation",
}

// WriteRegimesCSV writes the phase-regimes artifact in the format committed
// under docs and consumed by the paper.
//
// The phase_mix, phase_delay_samples and tolerance_db columns are each
// meaningful for one family only; the other families leave their cell empty
// rather than writing a zero that would read as a measured value.
func WriteRegimesCSV(output io.Writer, rows []RegimeRow) error {
	writer := csv.NewWriter(output)

	if err := writer.Write(regimesCSVHeader); err != nil {
		return fmt.Errorf("reference: write regimes CSV header: %w", err)
	}

	for _, row := range rows {
		mix := ""
		if row.Regime == "continuum" {
			mix = formatFloat(row.Mix)
		}

		delay := ""
		if row.Regime == "factorisation" {
			delay = strconv.Itoa(row.Delay)
		}

		tolerance, violation := "", ""
		if row.Regime == "floor-probe" {
			tolerance = formatFloat(row.ToleranceDB)
			violation = formatFloat(row.ConstraintViolation)
		}

		record := []string{
			row.Target,
			row.Regime,
			strconv.Itoa(row.SampleRate),
			strconv.Itoa(row.Taps),
			strconv.Itoa(row.FFTSize),
			mix,
			delay,
			tolerance,
			formatFloat(row.RelativeMagnitudeError),
			formatFloat(row.RMSMagnitudeErrorDB),
			formatFloat(row.MaxMagnitudeErrorDB),
			formatFloat(row.MeanGroupDelay),
			formatFloat(row.GroupDelayRipple),
			violation,
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("reference: write regimes CSV row: %w", err)
		}
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		return fmt.Errorf("reference: flush regimes CSV: %w", err)
	}

	return nil
}
