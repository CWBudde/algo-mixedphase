package reference

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
)

var csvHeader = []string{
	"target",
	"method",
	"sample_rate_hz",
	"taps",
	"fft_size",
	"phase_delay_samples",
	"iterations",
	"runtime_us",
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
			strconv.FormatInt(row.Runtime.Microseconds(), 10),
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

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', 9, 64)
}

func phaseDelay(row Row) string {
	if row.Method == "low-group-delay" {
		return ""
	}

	return strconv.Itoa(row.DelayBudget)
}
