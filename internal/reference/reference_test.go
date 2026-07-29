package reference

import (
	"bytes"
	"encoding/csv"
	"errors"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestTargetsShareFixedBudgets(t *testing.T) {
	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	wantNames := []string{
		"low-pass",
		"parametric-eq",
		"crossover",
		"deep-notch",
		"room-correction",
		"steep-crossover",
	}
	if len(targets) != len(wantNames) {
		t.Fatalf("len(Targets()) = %d, want %d", len(targets), len(wantNames))
	}

	for index, target := range targets {
		if target.Name != wantNames[index] {
			t.Errorf(
				"Targets()[%d].Name = %q, want %q",
				index,
				target.Name,
				wantNames[index],
			)
		}

		if len(target.Prototype) != prototypeLength {
			t.Errorf(
				"%s prototype length = %d, want %d",
				target.Name,
				len(target.Prototype),
				prototypeLength,
			)
		}

		if len(target.DelayWeight) != FFTSize/2+1 {
			t.Errorf(
				"%s weight length = %d, want %d",
				target.Name,
				len(target.DelayWeight),
				FFTSize/2+1,
			)
		}

		totalWeight := 0.0

		for _, weight := range target.DelayWeight {
			if weight < 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
				t.Errorf("%s has invalid weight %g", target.Name, weight)
			}

			totalWeight += weight
		}

		if totalWeight == 0 {
			t.Errorf("%s has zero total weight", target.Name)
		}
	}
}

func TestRunCoversEveryMethodAndMetric(t *testing.T) {
	rows, err := Run(0)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	// Taken from the harness rather than restated, so adding a method cannot
	// leave this test asserting a stale count.
	methodCount := len(methods())

	wantRows := len(targets) * methodCount
	if len(rows) != wantRows {
		t.Fatalf("len(Run()) = %d, want %d", len(rows), wantRows)
	}

	// budde-adaptive selects its own budget, so it is the one method whose
	// reported delay is not the suite constant. Everything else must be.
	choosesDelay := map[string]bool{}
	for _, method := range methods() {
		choosesDelay[method.name] = method.choosesDelay
	}

	for _, row := range rows {
		if row.SampleRate != SampleRate ||
			row.Taps != TapCount ||
			row.FFTSize != FFTSize {
			t.Errorf("%s/%s has inconsistent budget: %+v", row.Target, row.Method, row)
		}

		switch {
		case choosesDelay[row.Method]:
			if row.DelayBudget < 0 || row.DelayBudget > (TapCount-1)/2 {
				t.Errorf(
					"%s/%s selected delay %d outside [0, %d]",
					row.Target,
					row.Method,
					row.DelayBudget,
					(TapCount-1)/2,
				)
			}
		case row.DelayBudget != DelayBudget:
			t.Errorf(
				"%s/%s reported delay %d, want the suite budget %d",
				row.Target,
				row.Method,
				row.DelayBudget,
				DelayBudget,
			)
		}

		if row.Runtime != 0 {
			t.Errorf("%s/%s runtime = %v, want zero", row.Target, row.Method, row.Runtime)
		}

		finite := []float64{
			row.RelativeMagnitudeError,
			row.RMSMagnitudeErrorDB,
			row.MaxMagnitudeErrorDB,
			row.MeanGroupDelay,
			row.GroupDelayRipple,
			row.PeakGroupDelay,
			row.EnergyCentroid,
			row.PrePeakEnergyRatio,
			row.CoefficientPeak,
			row.CoefficientRangeDB,
			row.ConstraintViolation,
		}
		for _, value := range finite {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				t.Errorf(
					"%s/%s has non-finite metric %g",
					row.Target,
					row.Method,
					value,
				)
			}
		}

		if row.PrePeakEnergyRatio < 0 || row.PrePeakEnergyRatio > 1 {
			t.Errorf(
				"%s/%s pre-peak ratio = %g",
				row.Target,
				row.Method,
				row.PrePeakEnergyRatio,
			)
		}
	}

	assertCommittedQualityCSV(t, rows)
}

func TestWriteCSV(t *testing.T) {
	rows := []Row{{
		Target:                 "fixture",
		Method:                 "method",
		SampleRate:             SampleRate,
		Taps:                   TapCount,
		FFTSize:                FFTSize,
		DelayBudget:            DelayBudget,
		Iterations:             2,
		RelativeMagnitudeError: 0.25,
	}}

	var output bytes.Buffer
	if err := WriteCSV(&output, rows); err != nil {
		t.Fatalf("WriteCSV() error = %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(output.String())).ReadAll()
	if err != nil {
		t.Fatalf("read generated CSV: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("CSV row count = %d, want 2", len(records))
	}

	if len(records[0]) != len(csvHeader) ||
		len(records[1]) != len(csvHeader) {
		t.Fatalf(
			"CSV column counts = %d/%d, want %d",
			len(records[0]),
			len(records[1]),
			len(csvHeader),
		)
	}

	if records[1][0] != "fixture" || records[1][1] != "method" {
		t.Errorf("CSV identity columns = %q/%q", records[1][0], records[1][1])
	}
}

// assertTargetCoverage checks that rows contain exactly the wanted targets,
// each with the expected row count and in the written order.
func assertTargetCoverage[Row any](
	t *testing.T,
	kind string,
	want []string,
	perTarget int,
	rows []Row,
	name func(Row) string,
) {
	t.Helper()

	counts := make(map[string]int, len(want))
	order := make([]string, 0, len(want))

	for _, row := range rows {
		target := name(row)
		if _, seen := counts[target]; !seen {
			order = append(order, target)
		}

		counts[target]++
	}

	if !slices.Equal(order, want) {
		t.Fatalf("%s targets = %v, want %v", kind, order, want)
	}

	for _, target := range want {
		if counts[target] != perTarget {
			t.Errorf(
				"%s rows for %s = %d, want %d",
				kind,
				target,
				counts[target],
				perTarget,
			)
		}
	}
}

func TestRepresentativeResponsesCoverRealisedDesigns(t *testing.T) {
	frequencyRows, impulseRows, err := RepresentativeResponses()
	if err != nil {
		t.Fatalf("RepresentativeResponses() error = %v", err)
	}

	methodCount := len(methods())

	wantFrequencyRows := len(ResponseTargets()) * methodCount * (FFTSize/2 + 1)
	if len(frequencyRows) != wantFrequencyRows {
		t.Fatalf(
			"frequency row count = %d, want %d",
			len(frequencyRows),
			wantFrequencyRows,
		)
	}

	wantImpulseRows := len(ImpulseTargets()) * methodCount * TapCount
	if len(impulseRows) != wantImpulseRows {
		t.Fatalf(
			"impulse row count = %d, want %d",
			len(impulseRows),
			wantImpulseRows,
		)
	}

	// Every published target must be present with its full method set. The
	// contrast target is the whole point of the pair, so a silent regression to
	// one target has to fail here rather than quietly shrink the figures.
	assertTargetCoverage(
		t,
		"frequency",
		ResponseTargets(),
		methodCount*(FFTSize/2+1),
		frequencyRows,
		func(row FrequencyResponseRow) string { return row.Target },
	)
	assertTargetCoverage(
		t,
		"impulse",
		ImpulseTargets(),
		methodCount*TapCount,
		impulseRows,
		func(row ImpulseResponseRow) string { return row.Target },
	)

	weightedRows := 0

	for _, row := range frequencyRows {
		if row.SampleRate != SampleRate ||
			row.Taps != TapCount ||
			row.FFTSize != FFTSize {
			t.Errorf("inconsistent frequency-response budget: %+v", row)
		}

		values := []float64{
			row.FrequencyHz,
			row.TargetMagnitudeDB,
			row.MagnitudeDB,
			row.GroupDelay,
			row.DelayWeight,
		}
		for _, value := range values {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				t.Errorf("%s has non-finite response value %g", row.Method, value)
			}
		}

		if row.DelayWeight > 0 {
			weightedRows++
		}
	}

	if weightedRows == 0 {
		t.Fatal("representative response contains no weighted group-delay bins")
	}

	peaks := 0
	iterativeEnergy := 0.0
	iterativePrePeakEnergy := 0.0
	iterativeSignificantPrePeakSamples := 0

	for _, row := range impulseRows {
		if row.SampleRate != SampleRate ||
			row.Taps != TapCount ||
			row.FFTSize != FFTSize {
			t.Errorf("inconsistent impulse-response budget: %+v", row)
		}

		if math.IsNaN(row.Coefficient) ||
			math.IsInf(row.Coefficient, 0) ||
			math.IsNaN(row.NormalisedCoefficient) ||
			math.IsInf(row.NormalisedCoefficient, 0) {
			t.Errorf("%s has non-finite impulse row: %+v", row.Method, row)
		}

		if row.PeakAlignedIndex == 0 {
			peaks++

			if math.Abs(math.Abs(row.NormalisedCoefficient)-1) > 1e-12 {
				t.Errorf(
					"%s aligned peak = %g, want unit magnitude",
					row.Method,
					row.NormalisedCoefficient,
				)
			}
		}

		// Scoped to ImpulseTarget: the two published targets have very
		// different pre-peak distributions, and pooling them would let either
		// one carry the assertion on its own.
		if row.Method == "budde-iterative" && row.Target == ImpulseTarget {
			energy := row.Coefficient * row.Coefficient

			iterativeEnergy += energy

			if row.PeakAlignedIndex < 0 {
				iterativePrePeakEnergy += energy

				if math.Abs(row.NormalisedCoefficient) >= 0.01 {
					iterativeSignificantPrePeakSamples++
				}
			}
		}
	}

	if want := methodCount * len(ImpulseTargets()); peaks != want {
		t.Errorf("aligned peak count = %d, want %d", peaks, want)
	}

	if ratio := iterativePrePeakEnergy / iterativeEnergy; ratio < 0.1 {
		t.Errorf(
			"alternating %s pre-peak energy ratio = %g, want >= 0.1",
			ImpulseTarget,
			ratio,
		)
	}

	if iterativeSignificantPrePeakSamples < 8 {
		t.Errorf(
			"alternating %s significant pre-peak samples = %d, want >= 8",
			ImpulseTarget,
			iterativeSignificantPrePeakSamples,
		)
	}

	assertCommittedResponseCSVs(t, frequencyRows, impulseRows)
}

func TestMarkdownTable(t *testing.T) {
	rows := []Row{
		{
			Target:                 "parametric-eq",
			Method:                 "budde-iterative",
			Runtime:                1500,
			RelativeMagnitudeError: 0.0125,
			MeanGroupDelay:         4.25,
			PrePeakEnergyRatio:     0.5,
		},
		{
			Target: "parametric-eq",
			Method: "phase-interpolation",
		},
	}

	table := markdownTable(rows)
	wants := []string{
		"| parametric EQ | Budde iterative",
		"|               | phase interpolation",
		"1.25000%",
		"4.25",
		"50.00%",
	}

	for _, want := range wants {
		if !strings.Contains(table, want) {
			t.Errorf("markdownTable() does not contain %q:\n%s", want, table)
		}
	}

	// The table documents quality only. Runtime is set on the fixture above and
	// must still not reach the rendered document, because that document is
	// committed and has to stay reproducible.
	if strings.Contains(table, "ms") {
		t.Errorf("markdownTable() leaked a timing column:\n%s", table)
	}
}

func TestRunRejectsNegativeTrials(t *testing.T) {
	if _, err := Run(-1); !errors.Is(err, ErrInvalidTrials) {
		t.Fatalf("Run(-1) error = %v, want %v", err, ErrInvalidTrials)
	}
}

// TestUpdateMarkdownTableReplacesMarkedRegion covers the writer that rewrites
// the committed method table in docs/MIXED_PHASE_FILTER_DESIGN.md. It edits a
// tracked document in place, so the surrounding prose must survive untouched
// and a document without markers must be refused rather than overwritten.
func TestUpdateMarkdownTableReplacesMarkedRegion(t *testing.T) {
	rows := []Row{{
		Target:                 "parametric-eq",
		Method:                 "budde-iterative",
		RelativeMagnitudeError: 0.0125,
		MeanGroupDelay:         4.25,
		PrePeakEnergyRatio:     0.5,
	}}

	const (
		before = "# Heading\n\nprose before\n\n"
		after  = "\n\nprose after\n"
	)

	path := filepath.Join(t.TempDir(), "document.md")
	original := before + tableStart + "\n\nstale table\n" + tableEnd + after

	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if err := UpdateMarkdownTable(path, rows); err != nil {
		t.Fatalf("UpdateMarkdownTable() error = %v", err)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated document: %v", err)
	}

	got := string(updated)

	if !strings.HasPrefix(got, before) || !strings.HasSuffix(got, after) {
		t.Errorf("surrounding prose was not preserved:\n%s", got)
	}

	if strings.Contains(got, "stale table") {
		t.Errorf("stale table survived the update:\n%s", got)
	}

	if !strings.Contains(got, "Budde iterative") {
		t.Errorf("new table was not written:\n%s", got)
	}

	if strings.Count(got, tableStart) != 1 ||
		strings.Count(got, tableEnd) != 1 {
		t.Errorf("markers were duplicated:\n%s", got)
	}
}

func TestUpdateMarkdownTableRejectsUnmarkedDocument(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "no markers", content: "# Heading\n"},
		{name: "missing end", content: tableStart + "\n"},
		{name: "reversed order", content: tableEnd + "\n" + tableStart + "\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "document.md")
			if err := os.WriteFile(path, []byte(test.content), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}

			if err := UpdateMarkdownTable(path, nil); err == nil {
				t.Fatal("UpdateMarkdownTable() error = nil, want a refusal")
			}

			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read document: %v", err)
			}

			if string(got) != test.content {
				t.Errorf("document was modified despite the error:\n%s", got)
			}
		})
	}
}

func TestUpdateMarkdownTableReportsMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.md")
	if err := UpdateMarkdownTable(path, nil); err == nil {
		t.Fatal("UpdateMarkdownTable() error = nil for a missing file")
	}
}

func assertCommittedResponseCSVs(
	t *testing.T,
	frequencyRows []FrequencyResponseRow,
	impulseRows []ImpulseResponseRow,
) {
	t.Helper()

	var responses bytes.Buffer
	if err := WriteResponseCSV(&responses, frequencyRows); err != nil {
		t.Fatalf("WriteResponseCSV() error = %v", err)
	}

	var impulses bytes.Buffer
	if err := WriteImpulseCSV(&impulses, impulseRows); err != nil {
		t.Fatalf("WriteImpulseCSV() error = %v", err)
	}

	assertCommittedCSV(
		t,
		"reference-response.csv",
		responses.Bytes(),
	)
	assertCommittedCSV(
		t,
		"reference-impulse.csv",
		impulses.Bytes(),
	)
}

func assertCommittedQualityCSV(t *testing.T, rows []Row) {
	t.Helper()

	var generated bytes.Buffer
	if err := WriteCSV(&generated, rows); err != nil {
		t.Fatalf("WriteCSV() error = %v", err)
	}

	assertCommittedCSV(t, "reference-results.csv", generated.Bytes())
}

func assertCommittedCSV(t *testing.T, name string, generated []byte) {
	t.Helper()

	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}

	path := filepath.Join(
		filepath.Dir(source),
		"..",
		"..",
		"docs",
		name,
	)

	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read committed reference CSV: %v", err)
	}

	// A plain byte comparison. The suite carries no timing any more, so there
	// is nothing left to normalise away before comparing.
	if !bytes.Equal(generated, committed) {
		t.Fatal(
			"committed reference data are stale; run `just compare` " +
				"and review the result changes",
		)
	}
}
