package reference

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
)

// csvHeader is the committed quality schema. It carries no timing: wall-clock
// measurements are machine-dependent, and a non-reproducible column would make
// the committed CSV useless as the regression test it is meant to be. Timings
// live in their own artifact, written by [WriteTimingsCSV].
var csvHeader = []string{
	"target",
	"method",
	"sample_rate_hz",
	"taps",
	"fft_size",
	"phase_delay_samples",
	"iterations",
	"relative_magnitude_error",
	"rms_magnitude_error_db",
	"max_magnitude_error_db",
	"mean_group_delay",
	"group_delay_ripple",
	"peak_group_delay",
	"peak_index",
	"energy_centroid",
	"pre_peak_energy_ratio",
	"coefficient_peak",
	"coefficient_range_db",
	"constraint_violation",
}

var responseCSVHeader = []string{
	"target",
	"method",
	"sample_rate_hz",
	"taps",
	"fft_size",
	"phase_delay_samples",
	"frequency_hz",
	"target_magnitude_db",
	"magnitude_db",
	"group_delay_samples",
	"delay_weight",
}

var impulseCSVHeader = []string{
	"target",
	"method",
	"sample_rate_hz",
	"taps",
	"fft_size",
	"phase_delay_samples",
	"sample_index",
	"peak_index",
	"peak_aligned_index",
	"coefficient",
	"normalised_coefficient",
}

// WriteCSV writes rows in the stable format committed under docs.
func WriteCSV(output io.Writer, rows []Row) error {
	writer := csv.NewWriter(output)

	if err := writer.Write(csvHeader); err != nil {
		return fmt.Errorf("reference: write CSV header: %w", err)
	}

	for _, row := range rows {
		record := []string{
			row.Target,
			row.Method,
			strconv.Itoa(row.SampleRate),
			strconv.Itoa(row.Taps),
			strconv.Itoa(row.FFTSize),
			phaseDelay(row),
			strconv.Itoa(row.Iterations),
			formatFloat(row.RelativeMagnitudeError),
			formatFloat(row.RMSMagnitudeErrorDB),
			formatFloat(row.MaxMagnitudeErrorDB),
			formatFloat(row.MeanGroupDelay),
			formatFloat(row.GroupDelayRipple),
			formatFloat(row.PeakGroupDelay),
			strconv.Itoa(row.PeakIndex),
			formatFloat(row.EnergyCentroid),
			formatFloat(row.PrePeakEnergyRatio),
			formatFloat(row.CoefficientPeak),
			formatFloat(row.CoefficientRangeDB),
			formatFloat(row.ConstraintViolation),
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("reference: write CSV row: %w", err)
		}
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		return fmt.Errorf("reference: flush CSV: %w", err)
	}

	return nil
}

// WriteResponseCSV writes the representative realised frequency responses in
// the stable format consumed by the paper.
func WriteResponseCSV(
	output io.Writer,
	rows []FrequencyResponseRow,
) error {
	writer := csv.NewWriter(output)

	if err := writer.Write(responseCSVHeader); err != nil {
		return fmt.Errorf("reference: write response CSV header: %w", err)
	}

	for _, row := range rows {
		record := []string{
			row.Target,
			row.Method,
			strconv.Itoa(row.SampleRate),
			strconv.Itoa(row.Taps),
			strconv.Itoa(row.FFTSize),
			methodDelay(row.Method, row.DelayBudget),
			formatFloat(row.FrequencyHz),
			formatFloat(row.TargetMagnitudeDB),
			formatFloat(row.MagnitudeDB),
			formatFloat(row.GroupDelay),
			formatFloat(row.DelayWeight),
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("reference: write response CSV row: %w", err)
		}
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		return fmt.Errorf("reference: flush response CSV: %w", err)
	}

	return nil
}

// WriteImpulseCSV writes the representative peak-aligned impulse responses in
// the stable format consumed by the paper.
func WriteImpulseCSV(
	output io.Writer,
	rows []ImpulseResponseRow,
) error {
	writer := csv.NewWriter(output)

	if err := writer.Write(impulseCSVHeader); err != nil {
		return fmt.Errorf("reference: write impulse CSV header: %w", err)
	}

	for _, row := range rows {
		record := []string{
			row.Target,
			row.Method,
			strconv.Itoa(row.SampleRate),
			strconv.Itoa(row.Taps),
			strconv.Itoa(row.FFTSize),
			methodDelay(row.Method, row.DelayBudget),
			strconv.Itoa(row.SampleIndex),
			strconv.Itoa(row.PeakIndex),
			strconv.Itoa(row.PeakAlignedIndex),
			formatFloat(row.Coefficient),
			formatFloat(row.NormalisedCoefficient),
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("reference: write impulse CSV row: %w", err)
		}
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		return fmt.Errorf("reference: flush impulse CSV: %w", err)
	}

	return nil
}

// timingsHeader carries provenance on every row rather than in a comment
// block: encoding/csv only skips comments when explicitly configured, and
// Typst's csv() has no comment support at all, so plain columns keep the file
// readable by whatever consumes it next.
var timingsHeader = []string{
	"target",
	"method",
	"machine",
	"go_version",
	"trials",
	"runtime_us",
}

// WriteTimingsCSV writes the wall-clock measurements stripped from the quality
// CSV.
//
// This artifact is deliberately not reproducible: rerunning it on another
// machine, or on the same machine under different load, is expected to change
// every number. It is therefore regenerated on demand rather than by
// `just compare`, and is not part of any diff gate.
func WriteTimingsCSV(
	output io.Writer,
	rows []Row,
	machine, goVersion string,
	trials int,
) error {
	writer := csv.NewWriter(output)

	if err := writer.Write(timingsHeader); err != nil {
		return fmt.Errorf("reference: write timings header: %w", err)
	}

	for _, row := range rows {
		record := []string{
			row.Target,
			row.Method,
			machine,
			goVersion,
			strconv.Itoa(trials),
			strconv.FormatInt(row.Runtime.Microseconds(), 10),
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("reference: write timings row: %w", err)
		}
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		return fmt.Errorf("reference: flush timings CSV: %w", err)
	}

	return nil
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', 9, 64)
}

func phaseDelay(row Row) string {
	return methodDelay(row.Method, row.DelayBudget)
}

func methodDelay(method string, delay int) string {
	if method == "low-group-delay" {
		return ""
	}

	return strconv.Itoa(delay)
}
