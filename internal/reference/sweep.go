package reference

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/cwbudde/algo-mixedphase/mixedphase"
)

// The sweep characterises the alternating factorisation away from the single
// operating point the main comparison uses, and answers a question that
// comparison cannot: what the delay budget is worth as the output length grows.
//
// It is a separate artifact rather than extra rows in docs/reference-results.csv
// because it needs a different fixture. A 257-tap prototype is shorter than a
// 513-tap filter, so at the published fixture length every method would
// reproduce the target exactly and the sweep would measure nothing but rounding.
// The sweep therefore builds the same six analytic curves at
// SweepPrototypeTaps, and each row records the prototype length it used so the
// two artifacts can never be read as one.
const (
	// SweepPrototypeTaps is the fixture length for the sweep. It is long enough
	// that every curve is represented past its own decay at the longest output
	// length swept, so difficulty comes from the output budget rather than from
	// a truncated target.
	SweepPrototypeTaps = 2049

	// SweepTargetFFTSize is the grid the sweep fixtures are built on.
	SweepTargetFFTSize = 16384

	// SweepGridSize is the design and analysis grid for every sweep row. It is
	// held constant across output lengths so that error figures are comparable
	// between them, and it oversamples the longest fixture as the design entry
	// points require.
	SweepGridSize = 8192
)

// SweepLengths are the output tap counts the sweep covers, spanning the range
// from a filter far too short for the steeper targets to one long enough for all
// six.
var SweepLengths = []int{129, 257, 513, 1025}

// SweepDelayPoints is roughly how many delay budgets are sampled per output
// length. The admissible range is [0, (Length-1)/2], which grows with the
// length, so a fixed count means the stride grows too; sweepStride reports it.
// A coarse grid is deliberate — the artifact is for reading the shape of the
// trade-off, and a per-sample scan at 1025 taps would multiply the row count by
// thirty-two for no additional shape.
const SweepDelayPoints = 16

// sweepStride is the delay stride used at one output length.
func sweepStride(length int) int {
	return max((length-1)/2/SweepDelayPoints, 1)
}

// sweepDelays lists the budgets sampled at one output length: the strided scan
// plus the linear-phase endpoint.
//
// The endpoint is appended explicitly because the stride only lands on it when it
// happens to divide the admissible range. It does for every length in
// SweepLengths, all of which are one more than a power of two, but the artifact
// would silently lose its linear-phase endpoint for any length that is not.
func sweepDelays(length int) []int {
	maximum := (length - 1) / 2
	stride := sweepStride(length)

	delays := make([]int, 0, SweepDelayPoints+2)
	for delay := 0; delay <= maximum; delay += stride {
		delays = append(delays, delay)
	}

	if len(delays) == 0 || delays[len(delays)-1] != maximum {
		delays = append(delays, maximum)
	}

	return delays
}

// SweepRow is one point of the length-and-delay sweep.
type SweepRow struct {
	Target string

	// Method is "mixed-phase" for the alternating factorisation at an explicit
	// budget, or "linear-phase" for the equal-latency reference described in
	// [SweepRows].
	Method string

	SampleRate    int
	PrototypeTaps int
	Taps          int
	FFTSize       int
	Delay         int
	Iterations    int

	RelativeMagnitudeError float64
	RMSMagnitudeErrorDB    float64
	MaxMagnitudeErrorDB    float64
	MeanGroupDelay         float64
	GroupDelayRipple       float64

	// MinimumPhaseTaps is the share of the budget left to the minimum-phase
	// factor, Taps-2*Delay. It is the quantity the ceiling constraint bounds,
	// and carrying it saves every reader recomputing it.
	MinimumPhaseTaps int

	// OffCentreLinearEnergy is the fraction of the linear-phase factor's energy
	// away from its centre tap. Zero means the factor collapsed to a unit
	// impulse and the factorisation was inert, so the row describes a delayed
	// minimum-phase filter rather than anything the method shaped.
	OffCentreLinearEnergy float64
}

// SweepRows runs the length-and-delay sweep.
//
// Two families are emitted per target. The mixed-phase family designs Taps
// output taps at each sampled budget, which is the trade-off the delay budget
// actually controls. The linear-phase family is the equal-latency reference: a
// symmetric FIR of 2*Delay+1 taps, whose group delay is exactly Delay and whose
// ripple is therefore zero, but which has only that many taps to spend on the
// magnitude. Reading the two families against each other at equal mean group
// delay is the comparison a latency-bounded application faces, and it is where
// the construction earns its keep — see the package documentation of mixedphase.
func SweepRows() ([]SweepRow, error) {
	targets, err := targetsFor(
		SweepPrototypeTaps,
		SweepTargetFFTSize,
		SweepGridSize,
	)
	if err != nil {
		return nil, fmt.Errorf("reference: build sweep targets: %w", err)
	}

	rows := make([]SweepRow, 0, len(targets)*len(SweepLengths)*2*(SweepDelayPoints+1))

	for _, target := range targets {
		for _, length := range SweepLengths {
			for _, delay := range sweepDelays(length) {
				mixed, mixedErr := sweepMixedRow(target, length, delay)
				if mixedErr != nil {
					return nil, mixedErr
				}

				rows = append(rows, mixed)
			}
		}

		// The linear-phase reference is a function of latency alone, so it is
		// swept once per target over the latencies the mixed family reaches
		// rather than once per output length.
		for _, latency := range sweepLatencies() {
			linear, linearErr := sweepLinearRow(target, latency)
			if linearErr != nil {
				return nil, linearErr
			}

			rows = append(rows, linear)
		}
	}

	return rows, nil
}

// sweepLatencies are the latencies the linear-phase reference is evaluated at.
// They span the mixed family's reachable mean group delays at the longest
// output length.
func sweepLatencies() []int {
	longest := SweepLengths[len(SweepLengths)-1]
	maximum := (longest - 1) / 2
	stride := sweepStride(longest)

	latencies := make([]int, 0, SweepDelayPoints+1)
	for latency := stride; latency <= maximum; latency += stride {
		latencies = append(latencies, latency)
	}

	return latencies
}

func sweepMixedRow(target Target, length, delay int) (SweepRow, error) {
	result, err := mixedphase.DesignIterative(
		target.Prototype,
		mixedphase.IterativeConfig{
			Length:     length,
			Delay:      delay,
			Iterations: IterativePasses,
			FFTSize:    SweepGridSize,
		},
	)
	if err != nil {
		return SweepRow{}, fmt.Errorf(
			"reference: sweep %s at %d taps and delay %d: %w",
			target.Name,
			length,
			delay,
			err,
		)
	}

	return sweepRowFrom(target, "mixed-phase", length, delay, result)
}

func sweepLinearRow(target Target, latency int) (SweepRow, error) {
	// A symmetric FIR of 2*latency+1 taps is what DesignIterative produces when
	// the budget consumes the whole filter, so the reference is the same code
	// path rather than a second implementation that could drift from it.
	length := 2*latency + 1

	result, err := mixedphase.DesignIterative(
		target.Prototype,
		mixedphase.IterativeConfig{
			Length:     length,
			Delay:      latency,
			Iterations: IterativePasses,
			FFTSize:    SweepGridSize,
		},
	)
	if err != nil {
		return SweepRow{}, fmt.Errorf(
			"reference: linear-phase reference for %s at latency %d: %w",
			target.Name,
			latency,
			err,
		)
	}

	return sweepRowFrom(target, "linear-phase", length, latency, result)
}

func sweepRowFrom(
	target Target,
	method string,
	length, delay int,
	result mixedphase.Result,
) (SweepRow, error) {
	analysis, err := analyzeOn(target, result.Taps, SweepGridSize)
	if err != nil {
		return SweepRow{}, fmt.Errorf(
			"reference: analyse %s %s at %d taps and delay %d: %w",
			target.Name,
			method,
			length,
			delay,
			err,
		)
	}

	return SweepRow{
		Target:                 target.Name,
		Method:                 method,
		SampleRate:             SampleRate,
		PrototypeTaps:          SweepPrototypeTaps,
		Taps:                   length,
		FFTSize:                SweepGridSize,
		Delay:                  delay,
		Iterations:             result.Iterations,
		RelativeMagnitudeError: result.Metrics.RelativeMagnitudeError,
		RMSMagnitudeErrorDB:    result.Metrics.RMSMagnitudeErrorDB,
		MaxMagnitudeErrorDB:    result.Metrics.MaxMagnitudeErrorDB,
		MeanGroupDelay:         analysis.meanGroupDelay,
		GroupDelayRipple:       analysis.groupDelayRipple,
		MinimumPhaseTaps:       len(result.MinimumPhasePart),
		OffCentreLinearEnergy:  offCentreEnergyOf(result.LinearPhasePart),
	}, nil
}

// offCentreEnergyOf reports the fraction of a symmetric factor's energy that is
// not in its centre tap, which is zero exactly when the factor is a unit
// impulse and the factorisation was therefore inert.
func offCentreEnergyOf(factor []float64) float64 {
	if len(factor) <= 1 {
		return 0
	}

	total := 0.0
	for _, value := range factor {
		total += value * value
	}

	if total == 0 {
		return 0
	}

	centre := factor[len(factor)/2]

	return 1 - centre*centre/total
}

var sweepCSVHeader = []string{
	"target",
	"method",
	"sample_rate_hz",
	"prototype_taps",
	"taps",
	"fft_size",
	"phase_delay_samples",
	"iterations",
	"relative_magnitude_error",
	"rms_magnitude_error_db",
	"max_magnitude_error_db",
	"mean_group_delay",
	"group_delay_ripple",
	"minimum_phase_taps",
	"off_centre_linear_energy",
}

// WriteSweepCSV writes the length-and-delay sweep in the format committed under
// docs and consumed by the paper.
func WriteSweepCSV(output io.Writer, rows []SweepRow) error {
	writer := csv.NewWriter(output)

	if err := writer.Write(sweepCSVHeader); err != nil {
		return fmt.Errorf("reference: write sweep CSV header: %w", err)
	}

	for _, row := range rows {
		record := []string{
			row.Target,
			row.Method,
			strconv.Itoa(row.SampleRate),
			strconv.Itoa(row.PrototypeTaps),
			strconv.Itoa(row.Taps),
			strconv.Itoa(row.FFTSize),
			strconv.Itoa(row.Delay),
			strconv.Itoa(row.Iterations),
			formatFloat(row.RelativeMagnitudeError),
			formatFloat(row.RMSMagnitudeErrorDB),
			formatFloat(row.MaxMagnitudeErrorDB),
			formatFloat(row.MeanGroupDelay),
			formatFloat(row.GroupDelayRipple),
			strconv.Itoa(row.MinimumPhaseTaps),
			formatFloat(row.OffCentreLinearEnergy),
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("reference: write sweep CSV row: %w", err)
		}
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		return fmt.Errorf("reference: flush sweep CSV: %w", err)
	}

	return nil
}
