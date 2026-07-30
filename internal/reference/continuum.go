package reference

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/cwbudde/algo-mixedphase/mixedphase"
)

// The continuum artifact measures the unified group-delay knob of
// [mixedphase.DesignContinuum] on one axis: a requested delay in samples, from
// below the minimum-phase floor, through the prescribed-phase window, to beyond
// maximum phase.
//
// The existing regimes artifact scans the same ground on three different
// parameters — a mix, a budget, a tolerance — because that is what the three
// methods expose. The point of this one is that all of it is reachable from a
// single number, so the axis is shared and the branches can be read against each
// other rather than only against themselves.
//
// Two columns exist to be compared. requested_delay is what the caller asked
// for; mean_group_delay is what the realised filter does. Inside the window they
// agree to the least-squares projection residual, and predicted_delay records
// what the affine delay law says the request should have produced, so the
// residual of that law is itself a committed number rather than a claim in prose.
//
// The fixtures and the operating point are the published ones, so every row is
// comparable with docs/reference-results.csv and docs/reference-phase-regimes.csv
// rather than being a separate experiment sharing target names.
const (
	// ContinuumWindowPoints is the number of samples taken across the reachable
	// window, endpoints included. It matches the seventeen points of the
	// regimes artifact's mix scan so the two can be read side by side.
	ContinuumWindowPoints = 17

	// ContinuumImpulsePoints is the number of window positions at which the
	// impulse response is recorded.
	ContinuumImpulsePoints = 5
)

// ContinuumOutsideFractions are the positions sampled outside the window, as
// fractions of the distance from the window edge to the causal limit. On the
// sub-minimum side that limit is zero delay, and on the super-maximum side it is
// Taps-1.
//
// The ladder stops at three quarters rather than reaching the limit because the
// optimiser's answer stops being a measurement before then: driving the weighted
// mean delay to zero requires the magnitude to collapse, and what is recorded is
// then the iteration budget rather than the trade.
var ContinuumOutsideFractions = []float64{0.25, 0.5, 0.75}

// ContinuumRow is one point of the continuum artifact.
type ContinuumRow struct {
	Target string

	// Regime is the branch [mixedphase.DesignContinuum] took, one of
	// "sub-minimum", "window" or "super-maximum".
	Regime string

	SampleRate int
	Taps       int
	FFTSize    int

	// RequestedDelay is the knob setting, in samples.
	RequestedDelay float64

	// MinimumPhaseDelay is the target's group-delay floor on its own weight
	// band, and with it the lower edge of the reachable window. The upper edge
	// is Taps-1 minus this value.
	MinimumPhaseDelay float64

	// Mix is the prescribed phase mix the affine law inverts the request to. It
	// is meaningful for the window regime only; outside the window no mix
	// reaches the request, which is why those rows exist.
	Mix float64

	// PredictedDelay is the affine law's prediction for that mix. Comparing it
	// with MeanGroupDelay isolates the projection residual.
	PredictedDelay float64

	// Iterations is the optimiser budget spent outside the window, and zero
	// inside it, where the request is met in closed form.
	Iterations int

	RelativeMagnitudeError float64
	RMSMagnitudeErrorDB    float64
	MaxMagnitudeErrorDB    float64
	MeanGroupDelay         float64
	GroupDelayRipple       float64
}

// ContinuumImpulseRow is one coefficient of one continuum design.
type ContinuumImpulseRow struct {
	Target         string
	Regime         string
	RequestedDelay float64
	Taps           int

	SampleIndex      int
	PeakIndex        int
	PeakAlignedIndex int
	Coefficient      float64
	Normalised       float64
}

// ContinuumRows runs the unified knob across both branches of every published
// reference target.
func ContinuumRows() ([]ContinuumRow, error) {
	targets, err := Targets()
	if err != nil {
		return nil, fmt.Errorf("reference: build continuum targets: %w", err)
	}

	rows := make([]ContinuumRow, 0, len(targets)*(ContinuumWindowPoints+6))

	for _, target := range targets {
		floor, err := continuumFloor(target)
		if err != nil {
			return nil, err
		}

		for _, requested := range continuumRequests(floor) {
			row, rowErr := continuumRowAt(target, floor, requested)
			if rowErr != nil {
				return nil, rowErr
			}

			rows = append(rows, row)
		}
	}

	return rows, nil
}

// ContinuumImpulseRows records the impulse response at a few positions along the
// continuum, so that the reversal into maximum phase can be shown rather than
// asserted.
func ContinuumImpulseRows() ([]ContinuumImpulseRow, error) {
	targets, err := Targets()
	if err != nil {
		return nil, fmt.Errorf("reference: build continuum targets: %w", err)
	}

	rows := make(
		[]ContinuumImpulseRow,
		0,
		len(targets)*(ContinuumImpulsePoints+1)*TapCount,
	)

	for _, target := range targets {
		floor, err := continuumFloor(target)
		if err != nil {
			return nil, err
		}

		for _, requested := range continuumImpulseRequests(floor) {
			result, designErr := designContinuumAt(target, requested)
			if designErr != nil {
				return nil, designErr
			}

			rows = append(
				rows,
				continuumImpulse(target, result, requested)...,
			)
		}
	}

	return rows, nil
}

// continuumFloor measures a target's group-delay floor: the mean group delay of
// its minimum-phase realisation on its own weight band.
//
// It is measured through the same analysis the rest of the artifacts use rather
// than taken from the design, so that the window edges reported here are the same
// quantity as the mean_group_delay column everywhere else.
func continuumFloor(target Target) (float64, error) {
	result, err := mixedphase.DesignPhaseInterpolation(
		target.Prototype,
		mixedphase.PhaseInterpolationConfig{
			Length:  TapCount,
			Mix:     0,
			FFTSize: FFTSize,
		},
	)
	if err != nil {
		return 0, fmt.Errorf(
			"reference: continuum floor %s: %w",
			target.Name,
			err,
		)
	}

	analysis, err := analyze(target, result.Taps)
	if err != nil {
		return 0, fmt.Errorf(
			"reference: analyse continuum floor %s: %w",
			target.Name,
			err,
		)
	}

	return analysis.meanGroupDelay, nil
}

// continuumRequests lists the requested delays sampled for one target: the
// window at even spacing, and a ladder on each side of it.
func continuumRequests(floor float64) []float64 {
	span := float64(TapCount - 1)
	upper := span - floor

	requests := make(
		[]float64,
		0,
		ContinuumWindowPoints+2*len(ContinuumOutsideFractions),
	)

	// The sub-minimum ladder is built first and the super-maximum one is
	// derived from it by reflection, rather than both being computed from their
	// own end. Algebraically the two are the same points either way, but not to
	// the last bit, and the optimiser that serves them is a penalty ladder over
	// a non-convex objective: at the most aggressive request a difference in the
	// final bits of the target delay is enough to change the accepted step count
	// and land in a measurably different local optimum. Reflecting keeps the two
	// branches the same experiment.
	outside := make([]float64, 0, len(ContinuumOutsideFractions))
	for _, fraction := range ContinuumOutsideFractions {
		outside = append(outside, floor*(1-fraction))
	}

	// Ascending order keeps the artifact readable as a sweep.
	for i := len(outside) - 1; i >= 0; i-- {
		requests = append(requests, outside[i])
	}

	for i := range ContinuumWindowPoints {
		fraction := float64(i) / float64(ContinuumWindowPoints-1)
		requests = append(requests, floor+fraction*(upper-floor))
	}

	for _, request := range outside {
		requests = append(requests, span-request)
	}

	return requests
}

// continuumImpulseRequests lists the positions at which the impulse response is
// recorded: one below the floor and an even spread across the window, so that
// minimum, linear and maximum phase are all present.
func continuumImpulseRequests(floor float64) []float64 {
	span := float64(TapCount - 1)
	upper := span - floor

	requests := make([]float64, 0, ContinuumImpulsePoints+1)
	requests = append(requests, floor/2)

	for i := range ContinuumImpulsePoints {
		fraction := float64(i) / float64(ContinuumImpulsePoints-1)
		requests = append(requests, floor+fraction*(upper-floor))
	}

	return requests
}

// designContinuumAt runs the unified knob at the published operating point.
func designContinuumAt(
	target Target,
	requested float64,
) (mixedphase.Result, error) {
	result, err := mixedphase.DesignContinuum(
		target.Prototype,
		mixedphase.ContinuumConfig{
			Length:           TapCount,
			TargetGroupDelay: requested,
			FFTSize:          FFTSize,
			DelayWeight:      target.DelayWeight,
			Iterations:       LowDelayIterations,
			PenaltyStages:    LowDelayPenaltyStages,
		},
	)
	if err != nil {
		return mixedphase.Result{}, fmt.Errorf(
			"reference: continuum %s at %g samples: %w",
			target.Name,
			requested,
			err,
		)
	}

	return result, nil
}

func continuumRowAt(
	target Target,
	floor float64,
	requested float64,
) (ContinuumRow, error) {
	result, err := designContinuumAt(target, requested)
	if err != nil {
		return ContinuumRow{}, err
	}

	analysis, err := analyze(target, result.Taps)
	if err != nil {
		return ContinuumRow{}, fmt.Errorf(
			"reference: analyse continuum %s at %g: %w",
			target.Name,
			requested,
			err,
		)
	}

	centre := float64(TapCount-1) / 2

	row := ContinuumRow{
		Target:                 target.Name,
		Regime:                 result.Regime.String(),
		SampleRate:             SampleRate,
		Taps:                   TapCount,
		FFTSize:                FFTSize,
		RequestedDelay:         requested,
		MinimumPhaseDelay:      floor,
		Iterations:             result.Iterations,
		RelativeMagnitudeError: result.Metrics.RelativeMagnitudeError,
		RMSMagnitudeErrorDB:    result.Metrics.RMSMagnitudeErrorDB,
		MaxMagnitudeErrorDB:    result.Metrics.MaxMagnitudeErrorDB,
		MeanGroupDelay:         analysis.meanGroupDelay,
		GroupDelayRipple:       analysis.groupDelayRipple,
	}

	if result.Regime == mixedphase.RegimeWindow && centre != floor {
		row.Mix = (requested - floor) / (centre - floor)
		row.PredictedDelay = (1-row.Mix)*floor + row.Mix*centre
	}

	return row, nil
}

// continuumImpulse expands one design into per-coefficient rows.
func continuumImpulse(
	target Target,
	result mixedphase.Result,
	requested float64,
) []ContinuumImpulseRow {
	peakIndex, peak := 0, 0.0
	for index, tap := range result.Taps {
		if math.Abs(tap) > peak {
			peakIndex, peak = index, math.Abs(tap)
		}
	}

	rows := make([]ContinuumImpulseRow, 0, len(result.Taps))

	for index, tap := range result.Taps {
		normalised := 0.0
		if peak > 0 {
			normalised = tap / peak
		}

		rows = append(rows, ContinuumImpulseRow{
			Target:           target.Name,
			Regime:           result.Regime.String(),
			RequestedDelay:   requested,
			Taps:             TapCount,
			SampleIndex:      index,
			PeakIndex:        peakIndex,
			PeakAlignedIndex: index - peakIndex,
			Coefficient:      tap,
			Normalised:       normalised,
		})
	}

	return rows
}

var continuumCSVHeader = []string{
	"target",
	"regime",
	"sample_rate_hz",
	"taps",
	"fft_size",
	"requested_delay",
	"minimum_phase_delay",
	"phase_mix",
	"predicted_delay",
	"iterations",
	"relative_magnitude_error",
	"rms_magnitude_error_db",
	"max_magnitude_error_db",
	"mean_group_delay",
	"group_delay_ripple",
}

// WriteContinuumCSV writes the continuum artifact in the format committed under
// docs and consumed by the paper.
//
// The phase_mix and predicted_delay columns are meaningful for the window regime
// only. Outside the window no mix reaches the request and the affine law has
// nothing to predict, so those cells are left empty rather than filled with a
// zero that would read as a measurement.
func WriteContinuumCSV(output io.Writer, rows []ContinuumRow) error {
	writer := csv.NewWriter(output)

	if err := writer.Write(continuumCSVHeader); err != nil {
		return fmt.Errorf("reference: write continuum CSV header: %w", err)
	}

	for _, row := range rows {
		mix, predicted := "", ""
		if row.Regime == "window" {
			mix = formatFloat(row.Mix)
			predicted = formatFloat(row.PredictedDelay)
		}

		record := []string{
			row.Target,
			row.Regime,
			strconv.Itoa(row.SampleRate),
			strconv.Itoa(row.Taps),
			strconv.Itoa(row.FFTSize),
			formatFloat(row.RequestedDelay),
			formatFloat(row.MinimumPhaseDelay),
			mix,
			predicted,
			strconv.Itoa(row.Iterations),
			formatFloat(row.RelativeMagnitudeError),
			formatFloat(row.RMSMagnitudeErrorDB),
			formatFloat(row.MaxMagnitudeErrorDB),
			formatFloat(row.MeanGroupDelay),
			formatFloat(row.GroupDelayRipple),
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("reference: write continuum CSV row: %w", err)
		}
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		return fmt.Errorf("reference: flush continuum CSV: %w", err)
	}

	return nil
}

var continuumImpulseCSVHeader = []string{
	"target",
	"regime",
	"requested_delay",
	"taps",
	"sample_index",
	"peak_index",
	"peak_aligned_index",
	"coefficient",
	"normalised_coefficient",
}

// WriteContinuumImpulseCSV writes the continuum impulse artifact.
func WriteContinuumImpulseCSV(
	output io.Writer,
	rows []ContinuumImpulseRow,
) error {
	writer := csv.NewWriter(output)

	if err := writer.Write(continuumImpulseCSVHeader); err != nil {
		return fmt.Errorf(
			"reference: write continuum impulse CSV header: %w",
			err,
		)
	}

	for _, row := range rows {
		record := []string{
			row.Target,
			row.Regime,
			formatFloat(row.RequestedDelay),
			strconv.Itoa(row.Taps),
			strconv.Itoa(row.SampleIndex),
			strconv.Itoa(row.PeakIndex),
			strconv.Itoa(row.PeakAlignedIndex),
			formatFloat(row.Coefficient),
			formatFloat(row.Normalised),
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf(
				"reference: write continuum impulse CSV row: %w",
				err,
			)
		}
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		return fmt.Errorf("reference: flush continuum impulse CSV: %w", err)
	}

	return nil
}
