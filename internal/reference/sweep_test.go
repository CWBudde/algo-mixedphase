package reference

import (
	"bytes"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/cwbudde/algo-mixedphase/mixedphase"
)

// sweepFixture runs the sweep once and shares the result across the tests in
// this file. The sweep costs several seconds, and six tests each rebuilding it
// dominated the package's runtime for no benefit: it is deterministic, and none
// of these tests mutates a row.
var sweepFixture = sync.OnceValues(SweepRows)

func mustSweep(t *testing.T) []SweepRow {
	t.Helper()

	rows, err := sweepFixture()
	if err != nil {
		t.Fatalf("SweepRows() error = %v", err)
	}

	return rows
}

// TestSweepCoversEveryTargetAndLength checks the artifact's shape, so a silently
// truncated sweep cannot pass for a complete one.
func TestSweepCoversEveryTargetAndLength(t *testing.T) {
	rows := mustSweep(t)

	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	seen := map[string]map[int]int{}

	for _, row := range rows {
		if row.SampleRate != SampleRate {
			t.Errorf("%s: sample rate %d", row.Target, row.SampleRate)
		}

		if row.PrototypeTaps != SweepPrototypeTaps {
			t.Errorf(
				"%s: prototype %d taps, want %d; the sweep must not be read "+
					"against the published fixture length",
				row.Target,
				row.PrototypeTaps,
				SweepPrototypeTaps,
			)
		}

		if row.FFTSize != SweepGridSize {
			t.Errorf("%s: grid %d", row.Target, row.FFTSize)
		}

		// The support split is the invariant every row must satisfy.
		if want := row.Taps - 2*row.Delay; row.MinimumPhaseTaps != want {
			t.Errorf(
				"%s %s at %d taps and delay %d: minimum-phase factor has %d "+
					"taps, want %d",
				row.Target, row.Method, row.Taps, row.Delay,
				row.MinimumPhaseTaps, want,
			)
		}

		if row.Delay < 0 || row.Delay > (row.Taps-1)/2 {
			t.Errorf(
				"%s: delay %d outside [0, %d]",
				row.Target, row.Delay, (row.Taps-1)/2,
			)
		}

		if _, ok := seen[row.Target]; !ok {
			seen[row.Target] = map[int]int{}
		}

		if row.Method == "mixed-phase" {
			seen[row.Target][row.Taps]++
		}
	}

	for _, target := range targets {
		lengths, ok := seen[target.Name]
		if !ok {
			t.Errorf("target %q missing from the sweep", target.Name)

			continue
		}

		for _, length := range SweepLengths {
			if lengths[length] == 0 {
				t.Errorf("target %q has no rows at %d taps", target.Name, length)
			}
		}
	}
}

// TestSweepLinearPhaseHasNoRipple pins the reference family's defining property:
// a symmetric FIR has exactly linear phase, so its weighted group-delay
// deviation is zero and its mean delay is its half-length.
func TestSweepLinearPhaseHasNoRipple(t *testing.T) {
	rows := mustSweep(t)
	checked := 0

	for _, row := range rows {
		if row.Method != "linear-phase" {
			continue
		}

		checked++

		if row.GroupDelayRipple > 1e-6 {
			t.Errorf(
				"%s at latency %d: ripple %g, want zero for a symmetric filter",
				row.Target, row.Delay, row.GroupDelayRipple,
			)
		}

		if math.Abs(row.MeanGroupDelay-float64(row.Delay)) > 1e-6 {
			t.Errorf(
				"%s at latency %d: mean group delay %g",
				row.Target, row.Delay, row.MeanGroupDelay,
			)
		}
	}

	if checked == 0 {
		t.Fatal("the sweep contains no linear-phase reference rows")
	}
}

// TestSweepDelayBudgetStopsPayingAsLengthGrows is the sweep's own finding, and
// the reason the delay budget is a property of the output length rather than of
// the available latency.
//
// The budget exists to recover magnitude accuracy when the output support cannot
// host the target's minimum-phase response. As the support grows that need
// disappears, and with it the budget's value. For each target the sweep must show
// the best achievable RMS dB error at a non-zero budget converging on the
// zero-budget error — so that by the longest length there is nothing left for a
// budget to buy.
//
// Budget: SweepPrototypeTaps fixtures over SweepLengths on the SweepGridSize
// grid with IterativePasses correction passes.
func TestSweepDelayBudgetStopsPayingAsLengthGrows(t *testing.T) {
	rows := mustSweep(t)

	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	longest := SweepLengths[len(SweepLengths)-1]

	for _, target := range targets {
		t.Run(target.Name, func(t *testing.T) {
			var previous float64

			for index, length := range SweepLengths {
				zero, ok := zeroBudgetRow(rows, target.Name, length)
				if !ok {
					t.Fatalf("no zero-budget row at %d taps", length)
				}

				gain := bestBudgetGainDB(rows, target.Name, length)

				t.Logf(
					"%5d taps: zero budget %10.6f dB, best budget saves %8.4f dB",
					length, zero.RMSMagnitudeErrorDB, gain,
				)

				if index > 0 && gain > previous+1 {
					t.Errorf(
						"at %d taps a budget saves %.4f dB, more than the %.4f dB "+
							"it saved at %d taps; the budget's value should fall "+
							"as the support grows",
						length, gain, previous, SweepLengths[index-1],
					)
				}

				previous = gain

				// By the longest length the budget must have nothing left to buy.
				if length == longest && gain > 1 {
					t.Errorf(
						"at %d taps a budget still saves %.4f dB; the support is "+
							"supposed to be sufficient by here",
						length, gain,
					)
				}
			}
		})
	}
}

// bestBudgetGainDB is the largest RMS dB error a non-zero budget saves against
// the zero-budget design at one target and length, or zero if none saves any.
func bestBudgetGainDB(rows []SweepRow, target string, taps int) float64 {
	zero, ok := zeroBudgetRow(rows, target, taps)
	if !ok {
		return 0
	}

	best := 0.0

	for _, row := range rows {
		if row.Method != "mixed-phase" ||
			row.Target != target ||
			row.Taps != taps ||
			row.Delay == 0 {
			continue
		}

		best = max(best, zero.RMSMagnitudeErrorDB-row.RMSMagnitudeErrorDB)
	}

	return best
}

// TestSweepLinearPhaseNeedsFarMoreLatency pins the numbers quoted in the
// mixedphase package documentation.
//
// Budget: SweepPrototypeTaps fixtures designed into 1025 output taps with a zero
// budget on the SweepGridSize grid, against the linear-phase family sampled every
// sweepStride(1025) samples of latency. Each factor is therefore a lower bound to
// within one stride.
func TestSweepLinearPhaseNeedsFarMoreLatency(t *testing.T) {
	rows := mustSweep(t)

	cases := []struct {
		target      string
		wantLatency float64
		wantFactor  float64
	}{
		{"low-pass", 5.89, 16},
		{"parametric-eq", 6.68, 19},
		{"crossover", 10.46, 18},
		{"deep-notch", 9.94, 22},
		{"steep-crossover", 52.39, 7},
	}

	for _, testCase := range cases {
		t.Run(testCase.target, func(t *testing.T) {
			mixed, ok := zeroBudgetRow(rows, testCase.target, 1025)
			if !ok {
				t.Fatal("no zero-budget row at 1025 taps")
			}

			if math.Abs(mixed.MeanGroupDelay-testCase.wantLatency) > 0.05 {
				t.Errorf(
					"latency %.4f samples, want %.2f",
					mixed.MeanGroupDelay,
					testCase.wantLatency,
				)
			}

			matched, found := smallestMatchingLinear(
				rows,
				testCase.target,
				mixed.RMSMagnitudeErrorDB,
			)
			if !found {
				t.Fatalf(
					"no linear-phase latency matches %.6f dB",
					mixed.RMSMagnitudeErrorDB,
				)
			}

			factor := matched.MeanGroupDelay / mixed.MeanGroupDelay
			if factor < testCase.wantFactor {
				t.Errorf(
					"linear phase matched %.6f dB at %.2f samples, only %.2fx "+
						"the mixed design's %.2f; want at least %.0fx",
					matched.RMSMagnitudeErrorDB,
					matched.MeanGroupDelay,
					factor,
					mixed.MeanGroupDelay,
					testCase.wantFactor,
				)
			}
		})
	}

	// room-correction is the strongest case and has no factor, because the
	// linear-phase family never matches it at any sampled latency.
	t.Run("room-correction", func(t *testing.T) {
		mixed, ok := zeroBudgetRow(rows, "room-correction", 1025)
		if !ok {
			t.Fatal("no zero-budget row at 1025 taps")
		}

		if _, found := smallestMatchingLinear(
			rows,
			"room-correction",
			mixed.RMSMagnitudeErrorDB,
		); found {
			t.Error(
				"a linear-phase design now matches room-correction; the " +
					"documentation says none does at any sampled latency",
			)
		}
	})
}

func zeroBudgetRow(rows []SweepRow, target string, taps int) (SweepRow, bool) {
	for _, row := range rows {
		if row.Method == "mixed-phase" &&
			row.Target == target &&
			row.Taps == taps &&
			row.Delay == 0 {
			return row, true
		}
	}

	return SweepRow{}, false
}

// smallestMatchingLinear returns the lowest-latency linear-phase row whose RMS dB
// error is no worse than want.
func smallestMatchingLinear(
	rows []SweepRow,
	target string,
	want float64,
) (SweepRow, bool) {
	// The mixed designs reach errors below the printed precision of the CSV, so
	// the comparison is made at a tolerance rather than exactly; matching to
	// within a thousandth of a decibel is well past audibility.
	const tolerance = 1e-3

	best, found := SweepRow{}, false

	for _, row := range rows {
		if row.Method != "linear-phase" || row.Target != target {
			continue
		}

		if row.RMSMagnitudeErrorDB > max(want, tolerance) {
			continue
		}

		if !found || row.MeanGroupDelay < best.MeanGroupDelay {
			best, found = row, true
		}
	}

	return best, found
}

// TestWriteSweepCSVMatchesItsHeader guards the schema against a column being
// added to one side only.
func TestWriteSweepCSVMatchesItsHeader(t *testing.T) {
	var buffer bytes.Buffer

	if err := WriteSweepCSV(&buffer, []SweepRow{{
		Target: "t", Method: "mixed-phase", SampleRate: SampleRate,
		PrototypeTaps: SweepPrototypeTaps, Taps: 129, FFTSize: SweepGridSize,
	}}); err != nil {
		t.Fatalf("WriteSweepCSV() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buffer.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("wrote %d lines, want a header and one row", len(lines))
	}

	header := strings.Split(lines[0], ",")
	if len(header) != len(sweepCSVHeader) {
		t.Errorf("header has %d columns, want %d", len(header), len(sweepCSVHeader))
	}

	if got := len(strings.Split(lines[1], ",")); got != len(sweepCSVHeader) {
		t.Errorf("row has %d fields, want %d", got, len(sweepCSVHeader))
	}
}

// TestCommittedSweepCSVIsReproducible byte-compares the committed artifact
// against fresh generator output, the same gate the other CSVs carry.
func TestCommittedSweepCSVIsReproducible(t *testing.T) {
	rows := mustSweep(t)

	var buffer bytes.Buffer
	if err := WriteSweepCSV(&buffer, rows); err != nil {
		t.Fatalf("WriteSweepCSV() error = %v", err)
	}

	assertCommittedCSV(t, "reference-delay-sweep.csv", buffer.Bytes())
}

// minimumPhaseSupport reports how many leading taps hold all but tail of the
// energy, which is the support the split's ceiling constraint is measured
// against.
func minimumPhaseSupport(taps []float64, tail float64) int {
	total := 0.0
	for _, tap := range taps {
		total += tap * tap
	}

	if total == 0 {
		return 0
	}

	running := 0.0

	for index, tap := range taps {
		running += tap * tap
		if running >= (1-tail)*total {
			return index + 1
		}
	}

	return len(taps)
}

// TestSweepMinimumPhaseSupportBoundsTheBudget pins the support figures quoted in
// the mixedphase package documentation, and with them the ceiling constraint.
//
// The split leaves the minimum-phase factor Taps-2*Delay coefficients, so a
// budget is admissible on magnitude grounds only while that share still covers
// the factor's own support L_min:
//
//	Delay <= (Taps - L_min) / 2
//
// A target whose L_min exceeds Taps outright has no admissible budget by that
// reading, and its optimum is necessarily a compromise found by search — which is
// exactly steep-crossover below 513 taps.
//
// Budget: the SweepPrototypeTaps fixtures reconstructed on the SweepTargetFFTSize
// grid, with the support taken at 1e-6 of the factor's energy.
func TestSweepMinimumPhaseSupportBoundsTheBudget(t *testing.T) {
	targets, err := targetsFor(
		SweepPrototypeTaps,
		SweepTargetFFTSize,
		SweepGridSize,
	)
	if err != nil {
		t.Fatalf("targetsFor() error = %v", err)
	}

	want := map[string]int{
		"crossover":       52,
		"low-pass":        53,
		"parametric-eq":   116,
		"deep-notch":      129,
		"steep-crossover": 238,
		"room-correction": 995,
	}

	for _, target := range targets {
		t.Run(target.Name, func(t *testing.T) {
			factor, minimumErr := mixedphase.MinimumPhase(
				target.Prototype,
				SweepTargetFFTSize,
			)
			if minimumErr != nil {
				t.Fatalf("MinimumPhase() error = %v", minimumErr)
			}

			support := minimumPhaseSupport(factor, 1e-6)
			if support != want[target.Name] {
				t.Errorf("support = %d taps, want %d", support, want[target.Name])
			}
		})
	}
}

// TestSweepBudgetGainTableMatchesTheDocumentation pins the per-length gain table
// quoted in the mixedphase package documentation, which is the single clearest
// statement of when the delay budget is worth anything.
func TestSweepBudgetGainTableMatchesTheDocumentation(t *testing.T) {
	rows := mustSweep(t)

	// Gains are quoted to two decimals in the documentation, so they are checked
	// to half of that.
	const tolerance = 0.005

	want := map[string]map[int]float64{
		"steep-crossover": {129: 57.19, 257: 23.05, 513: 0, 1025: 0},
		"room-correction": {129: 0, 257: 0.04, 513: 0.03, 1025: 0},
		"low-pass":        {129: 0, 257: 0, 513: 0, 1025: 0},
		"parametric-eq":   {129: 0, 257: 0, 513: 0, 1025: 0},
		"crossover":       {129: 0, 257: 0, 513: 0, 1025: 0},
		"deep-notch":      {129: 0, 257: 0, 513: 0, 1025: 0},
	}

	for target, lengths := range want {
		for _, length := range SweepLengths {
			got := bestBudgetGainDB(rows, target, length)
			if math.Abs(got-lengths[length]) > tolerance {
				t.Errorf(
					"%s at %d taps: a budget saves %.4f dB, documented as %.2f dB",
					target, length, got, lengths[length],
				)
			}
		}
	}
}

func TestTargetsForRejectsUnusableGrids(t *testing.T) {
	cases := []struct {
		name                                 string
		prototypeTaps, targetFFT, weightGrid int
	}{
		{"zero prototype", 0, SweepTargetFFTSize, SweepGridSize},
		{"negative prototype", -1, SweepTargetFFTSize, SweepGridSize},
		{"zero target grid", SweepPrototypeTaps, 0, SweepGridSize},
		{"zero weight grid", SweepPrototypeTaps, SweepTargetFFTSize, 0},
		// A prototype longer than the grid it is inverted from cannot be
		// windowed out of that grid without wrapping onto itself.
		{"prototype longer than its grid", 8193, 4096, SweepGridSize},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := targetsFor(
				testCase.prototypeTaps,
				testCase.targetFFT,
				testCase.weightGrid,
			); err == nil {
				t.Error("targetsFor() accepted an unusable configuration")
			}
		})
	}
}

// TestAnalyzeOnRejectsAMismatchedWeightGrid closes the one way a caller can
// silently analyse on the wrong grid: the weights are indexed by bin, so a
// weight vector built for another grid would measure the wrong frequencies
// rather than fail.
func TestAnalyzeOnRejectsAMismatchedWeightGrid(t *testing.T) {
	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	if _, err := analyzeOn(targets[0], []float64{1}, SweepGridSize); err == nil {
		t.Error("analyzeOn() accepted weights built for a different grid")
	}
}

func TestOffCentreEnergyOfHandlesDegenerateFactors(t *testing.T) {
	cases := []struct {
		name   string
		factor []float64
		want   float64
	}{
		{"unit impulse", []float64{1}, 0},
		{"empty", nil, 0},
		{"all zero", []float64{0, 0, 0}, 0},
		{"centred impulse", []float64{0, 1, 0}, 0},
		{"half off centre", []float64{1, 1, 0}, 0.5},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := offCentreEnergyOf(testCase.factor); math.Abs(got-testCase.want) > 1e-12 {
				t.Errorf("offCentreEnergyOf() = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestSweepDelaysAlwaysReachTheLinearPhaseEndpoint pins the property the
// artifact's shape depends on. Every length in SweepLengths happens to make the
// stride divide the admissible range, so the explicit endpoint is redundant
// there; a length that does not would otherwise lose its linear-phase row.
func TestSweepDelaysAlwaysReachTheLinearPhaseEndpoint(t *testing.T) {
	lengths := append([]int{}, SweepLengths...)
	// Lengths whose admissible range the stride does not divide, which is the
	// case the explicit endpoint exists for.
	lengths = append(lengths, 131, 201, 999, 3, 1)

	for _, length := range lengths {
		delays := sweepDelays(length)
		maximum := (length - 1) / 2

		if len(delays) == 0 {
			t.Errorf("length %d produced no delays", length)

			continue
		}

		if delays[0] != 0 {
			t.Errorf("length %d starts at delay %d, want 0", length, delays[0])
		}

		if last := delays[len(delays)-1]; last != maximum {
			t.Errorf(
				"length %d ends at delay %d, want the linear-phase endpoint %d",
				length, last, maximum,
			)
		}

		for index, delay := range delays {
			if delay < 0 || delay > maximum {
				t.Errorf("length %d delay %d outside [0, %d]", length, delay, maximum)
			}

			if index > 0 && delay <= delays[index-1] {
				t.Errorf("length %d delays are not increasing at %d", length, index)
			}
		}
	}
}
