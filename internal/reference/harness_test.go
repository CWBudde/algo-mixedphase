package reference

import (
	"bytes"
	"encoding/csv"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	algofft "github.com/cwbudde/algo-fft"
	"github.com/cwbudde/algo-mixedphase/mixedphase"
)

// errWriter fails every write so the CSV writers' flush paths are exercised.
type errWriter struct{}

var errWriterFailed = errors.New("write failed")

func (errWriter) Write([]byte) (int, error) { return 0, errWriterFailed }

func TestWriteTimingsCSVCarriesProvenanceOnEveryRow(t *testing.T) {
	rows := []Row{
		{Target: "low-pass", Method: "budde-iterative", Runtime: 1500 * time.Microsecond},
		{Target: "crossover", Method: "low-group-delay", Runtime: 2 * time.Millisecond},
	}

	var buffer bytes.Buffer
	if err := WriteTimingsCSV(&buffer, rows, "test-machine", "go1.25.0", 7); err != nil {
		t.Fatalf("WriteTimingsCSV: %v", err)
	}

	records, err := csv.NewReader(&buffer).ReadAll()
	if err != nil {
		t.Fatalf("parse timings CSV: %v", err)
	}

	if len(records) != len(rows)+1 {
		t.Fatalf("record count = %d, want %d", len(records), len(rows)+1)
	}

	for column, want := range timingsHeader {
		if records[0][column] != want {
			t.Errorf("header[%d] = %q, want %q", column, records[0][column], want)
		}
	}

	wantMicroseconds := []string{"1500", "2000"}

	for index, record := range records[1:] {
		if len(record) != len(timingsHeader) {
			t.Fatalf("row %d has %d columns, want %d",
				index, len(record), len(timingsHeader))
		}

		if record[0] != rows[index].Target || record[1] != rows[index].Method {
			t.Errorf("row %d identity = %q/%q, want %q/%q",
				index, record[0], record[1], rows[index].Target, rows[index].Method)
		}

		// Provenance is what makes this artifact interpretable despite being
		// machine-dependent, so assert it rather than the runtime value.
		if record[2] != "test-machine" || record[3] != "go1.25.0" {
			t.Errorf("row %d provenance = %q/%q, want test-machine/go1.25.0",
				index, record[2], record[3])
		}

		if record[4] != "7" {
			t.Errorf("row %d trials = %q, want 7", index, record[4])
		}

		if record[5] != wantMicroseconds[index] {
			t.Errorf("row %d runtime_us = %q, want %q",
				index, record[5], wantMicroseconds[index])
		}
	}
}

func TestWriteTimingsCSVNeverLeaksIntoTheGatedArtifacts(t *testing.T) {
	// The quality CSV must not carry a runtime column: it is diff-gated, and a
	// machine-dependent number in it would make `just compare-check` fail on
	// every machine but one.
	for _, column := range csvHeader {
		if strings.Contains(column, "runtime") || strings.Contains(column, "_us") {
			t.Fatalf("quality CSV header contains timing column %q", column)
		}
	}
}

func TestCSVWritersReportWriteFailures(t *testing.T) {
	tests := []struct {
		name  string
		write func() error
	}{
		{
			name:  "quality",
			write: func() error { return WriteCSV(errWriter{}, []Row{{Target: "t"}}) },
		},
		{
			name: "response",
			write: func() error {
				return WriteResponseCSV(errWriter{}, []FrequencyResponseRow{{Target: "t"}})
			},
		},
		{
			name: "impulse",
			write: func() error {
				return WriteImpulseCSV(errWriter{}, []ImpulseResponseRow{{Target: "t"}})
			},
		},
		{
			name: "timings",
			write: func() error {
				return WriteTimingsCSV(errWriter{}, []Row{{Target: "t"}}, "m", "g", 1)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.write(); err == nil {
				t.Fatal("write to a failing writer returned nil error")
			}
		})
	}
}

func TestRunWithTrialsMeasuresRuntimeWithoutChangingResults(t *testing.T) {
	untimed, err := Run(0)
	if err != nil {
		t.Fatalf("Run(0): %v", err)
	}

	timed, err := Run(1)
	if err != nil {
		t.Fatalf("Run(1): %v", err)
	}

	if len(timed) != len(untimed) {
		t.Fatalf("row count with trials = %d, want %d", len(timed), len(untimed))
	}

	for index := range timed {
		if timed[index].Runtime <= 0 {
			t.Errorf("row %d (%s/%s) runtime = %v, want a positive measurement",
				index, timed[index].Target, timed[index].Method, timed[index].Runtime)
		}

		if untimed[index].Runtime != 0 {
			t.Errorf("row %d untimed runtime = %v, want 0",
				index, untimed[index].Runtime)
		}

		// Timing must not perturb the design: the quality columns are what the
		// diff gate compares, and they are produced by the same code path.
		measured := timed[index]
		measured.Runtime = 0

		if measured != untimed[index] {
			t.Errorf("row %d (%s/%s) differs between Run(0) and Run(1)",
				index, measured.Target, measured.Method)
		}
	}
}

func TestFindTargetReportsMissingNames(t *testing.T) {
	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets: %v", err)
	}

	found, err := findTarget(targets, targets[0].Name)
	if err != nil {
		t.Fatalf("findTarget(%q): %v", targets[0].Name, err)
	}

	if found.Name != targets[0].Name {
		t.Errorf("found %q, want %q", found.Name, targets[0].Name)
	}

	if _, err := findTarget(targets, "no-such-target"); err == nil {
		t.Fatal("findTarget on a missing name returned nil error")
	}
}

func TestDisplayNameCoversEveryMethodAndTarget(t *testing.T) {
	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets: %v", err)
	}

	names := make([]string, 0, len(targets)+len(methods()))
	for _, target := range targets {
		names = append(names, target.Name)
	}

	for _, method := range methods() {
		names = append(names, method.name)
	}

	// Distinct slugs must render to distinct labels, otherwise two rows of the
	// published table become indistinguishable. This is what catches a
	// copy-pasted case when a method or target is added.
	seen := make(map[string]string, len(names))

	for _, name := range names {
		display := displayName(name)
		if display == "" {
			t.Errorf("displayName(%q) is empty", name)
		}

		if previous, clash := seen[display]; clash {
			t.Errorf("displayName(%q) and displayName(%q) both render as %q",
				previous, name, display)
		}

		seen[display] = name
	}

	if got := displayName("unregistered"); got != "unregistered" {
		t.Errorf("displayName on an unknown slug = %q, want passthrough", got)
	}
}

func TestRealSpectrumRefusesSignalsLongerThanTheGrid(t *testing.T) {
	plan, err := algofft.NewPlan64(FFTSize)
	if err != nil {
		t.Fatalf("NewPlan64: %v", err)
	}

	if _, err := realSpectrum(plan, make([]float64, FFTSize)); err != nil {
		t.Fatalf("realSpectrum at exactly the grid size: %v", err)
	}

	if _, err := realSpectrum(plan, make([]float64, FFTSize+1)); err == nil {
		t.Fatal("realSpectrum accepted a signal longer than the grid")
	}
}

func TestParseRoomResponseRejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "header only", input: "frequency_hz,response_db\n"},
		{name: "only comments", input: "# nothing here\n"},
		{
			name:  "single row",
			input: "frequency_hz,response_db\n20,1.0\n",
		},
		{
			name:  "non-numeric frequency",
			input: "frequency_hz,response_db\nlow,1.0\n40,2.0\n",
		},
		{
			name:  "non-numeric response",
			input: "frequency_hz,response_db\n20,loud\n40,2.0\n",
		},
		{
			name:  "ragged row",
			input: "frequency_hz,response_db\n20,1.0\n40\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			points, err := parseRoomResponse(test.input)
			if err == nil {
				t.Fatalf("parseRoomResponse(%q) = %v, want error", test.input, points)
			}
		})
	}
}

func TestParseRoomResponseSkipsCommentsAndKeepsOrder(t *testing.T) {
	input := "# measured on a Tuesday\n" +
		"frequency_hz,response_db\n" +
		"20,-3.5\n" +
		"# a comment in the middle\n" +
		"40,1.25\n" +
		"80,0\n"

	points, err := parseRoomResponse(input)
	if err != nil {
		t.Fatalf("parseRoomResponse: %v", err)
	}

	want := []roomPoint{
		{frequency: 20, response: -3.5},
		{frequency: 40, response: 1.25},
		{frequency: 80, response: 0},
	}

	if len(points) != len(want) {
		t.Fatalf("point count = %d, want %d", len(points), len(want))
	}

	for index, point := range points {
		if point != want[index] {
			t.Errorf("point[%d] = %+v, want %+v", index, point, want[index])
		}
	}
}

func TestMethodDelayBlanksTheUnconstrainedMethod(t *testing.T) {
	// low-group-delay does not honour DelayBudget, so reporting the budget as
	// its delay would be a lie in a published column.
	if got := methodDelay("low-group-delay", DelayBudget); got != "" {
		t.Errorf("methodDelay(low-group-delay) = %q, want empty", got)
	}

	for _, method := range methods() {
		if method.name == "low-group-delay" {
			continue
		}

		got := methodDelay(method.name, DelayBudget)
		if _, err := strconv.Atoi(got); err != nil {
			t.Errorf("methodDelay(%s) = %q, want an integer", method.name, got)
		}
	}
}

// linearFactorOffCentreEnergy reports the fraction of the linear-phase factor's
// energy that sits away from its centre tap.
//
// A value of zero means the factor is a bare unit impulse: the alternating
// correction converged to the identity and the design is a delayed
// minimum-phase filter.
func linearFactorOffCentreEnergy(factor []float64) float64 {
	centre := (len(factor) - 1) / 2

	var offCentre, total float64

	for index, value := range factor {
		energy := value * value
		total += energy

		if index != centre {
			offCentre += energy
		}
	}

	if total == 0 {
		return 0
	}

	return offCentre / total
}

// TestSteepTargetActuallyExercisesTheFactorisation is the guard that keeps the
// reference suite honest.
//
// For a target smooth enough that a minimum-phase filter fits it inside the
// Length-2*Delay taps the split leaves, the residual quotient is unit-magnitude
// and its zero-phase inverse transform is a unit impulse. The alternating
// correction then has nothing to do, and every published number for
// budde-iterative describes a delayed minimum-phase filter rather than anything
// the method shaped. The whole suite was in that state before steep-crossover
// was added.
//
// This test asserts the two halves of that story: the smooth targets do
// degenerate, and steep-crossover does not.
func TestSteepTargetActuallyExercisesTheFactorisation(t *testing.T) {
	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	var steep, smooth int

	for _, target := range targets {
		result, designErr := mixedphase.DesignIterative(
			target.Prototype,
			mixedphase.IterativeConfig{
				Length:     TapCount,
				Delay:      DelayBudget,
				Iterations: iterativePasses,
				FFTSize:    FFTSize,
			},
		)
		if designErr != nil {
			t.Fatalf("%s: DesignIterative() error = %v", target.Name, designErr)
		}

		offCentre := linearFactorOffCentreEnergy(result.LinearPhasePart)
		t.Logf(
			"%-16s off-centre linear energy = %.6f, passes = %d",
			target.Name,
			offCentre,
			result.Iterations,
		)

		if target.Name == "steep-crossover" {
			steep++

			if offCentre < 0.5 {
				t.Errorf(
					"steep-crossover linear factor carries only %.6f of its "+
						"energy off centre; the target no longer starves the "+
						"minimum-phase factor and the suite has stopped "+
						"exercising the alternating correction",
					offCentre,
				)
			}

			if result.Iterations < 2 {
				t.Errorf(
					"steep-crossover converged after %d passes; the correction "+
						"loop is no longer doing work",
					result.Iterations,
				)
			}

			continue
		}

		smooth++

		if offCentre > 0.5 {
			t.Logf(
				"note: %s now also starves the minimum-phase factor (%.6f)",
				target.Name,
				offCentre,
			)
		}
	}

	if steep != 1 {
		t.Fatalf("found %d steep targets, want exactly 1", steep)
	}

	if smooth == 0 {
		t.Fatal("no smooth targets left to contrast against")
	}
}
