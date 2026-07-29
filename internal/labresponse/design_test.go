package labresponse

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/cwbudde/algo-mixedphase/internal/reference"
)

func TestTargetNamesStartWithTheAdjustableLowpass(t *testing.T) {
	names, err := TargetNames()
	if err != nil {
		t.Fatalf("TargetNames() error = %v", err)
	}

	targets, err := reference.Targets()
	if err != nil {
		t.Fatalf("reference.Targets() error = %v", err)
	}

	want := []string{LowpassTarget}
	for _, target := range targets {
		want = append(want, target.Name)
	}

	if !slices.Equal(names, want) {
		t.Errorf("TargetNames() = %v, want %v", names, want)
	}
}

func TestDesignRejectsUnknownNames(t *testing.T) {
	cases := []struct {
		name    string
		request Request
		want    error
	}{
		{
			name:    "unknown target",
			request: Request{Method: "iterative", Target: "no-such-target"},
			want:    ErrUnknownTarget,
		},
		{
			name:    "unknown method",
			request: Request{Method: "no-such-method", Target: LowpassTarget, Length: 129, Cutoff: 0.08},
			want:    ErrUnknownMethod,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := Design(testCase.request); !errors.Is(err, testCase.want) {
				t.Errorf("Design() error = %v, want %v", err, testCase.want)
			}
		})
	}
}

// TestDesignRejectsAnUnusableLowpass keeps the untrusted boundary closed: the
// adjustable fixture is the only target built from request fields, so it is the
// only one a shared URL can ask for at an impossible size.
func TestDesignRejectsAnUnusableLowpass(t *testing.T) {
	if _, err := Design(Request{
		Method: "iterative",
		Target: LowpassTarget,
		Length: 1,
		Cutoff: 0.08,
	}); err == nil {
		t.Error("Design() with a one-tap low-pass succeeded, want an error")
	}
}

// TestDesignDefaultsToTheAdjustableLowpass pins the compatibility contract: a
// request that names no target — every URL shared before the comparison
// fixtures existed — still designs the low-pass it used to.
func TestDesignDefaultsToTheAdjustableLowpass(t *testing.T) {
	blank, err := Design(Request{Method: "iterative", Length: 129, Cutoff: 0.08, Delay: 8, Iterations: 12})
	if err != nil {
		t.Fatalf("Design() error = %v", err)
	}

	named, err := Design(Request{
		Method:     "iterative",
		Target:     LowpassTarget,
		Length:     129,
		Cutoff:     0.08,
		Delay:      8,
		Iterations: 12,
	})
	if err != nil {
		t.Fatalf("Design() error = %v", err)
	}

	if !slices.Equal(blank.Result.Taps, named.Result.Taps) {
		t.Error("an unnamed target no longer designs the adjustable low-pass")
	}
}

// TestDesignPlotsTheRealisedFilter checks that the curves come from the taps
// rather than the design grid, which is the property the lab exists to show.
func TestDesignPlotsTheRealisedFilter(t *testing.T) {
	design, err := Design(Request{
		Method:     "iterative",
		Target:     reference.DegenerateContrastTarget,
		Length:     reference.TapCount,
		Delay:      reference.DelayBudget,
		Iterations: reference.IterativePasses,
	})
	if err != nil {
		t.Fatalf("Design() error = %v", err)
	}

	if len(design.Realised.MagnitudeDB) != Points {
		t.Fatalf("realised curve has %d points, want %d", len(design.Realised.MagnitudeDB), Points)
	}

	direct := New(design.Result.Taps)
	if !slices.Equal(direct.MagnitudeDB, design.Realised.MagnitudeDB) {
		t.Error("the realised curve was not evaluated from the returned taps")
	}

	prototype := New(mustPrototype(t, reference.DegenerateContrastTarget))
	if !slices.Equal(prototype.MagnitudeDB, design.Prototype.MagnitudeDB) {
		t.Error("the reference curve was not evaluated from the target prototype")
	}
}

func mustPrototype(t *testing.T, name string) []float64 {
	t.Helper()

	targets, err := reference.Targets()
	if err != nil {
		t.Fatalf("reference.Targets() error = %v", err)
	}

	for _, target := range targets {
		if target.Name == name {
			return target.Prototype
		}
	}

	t.Fatalf("reference target %q not found", name)

	return nil
}

// harnessEquivalent maps a published method name onto the lab request that must
// reproduce it. The lab drives the same designers, so agreement here is exact
// rather than approximate; anything less means the lab is quietly configuring a
// different design from the one the paper reports.
var harnessEquivalent = map[string]Request{
	"budde-iterative": {
		Method:     "iterative",
		Delay:      reference.DelayBudget,
		Iterations: reference.IterativePasses,
	},
	"phase-interpolation": {
		Method: "interpolation",
		Delay:  reference.DelayBudget,
	},
	"complex-minimax": {
		Method:     "minimax",
		Delay:      reference.DelayBudget,
		Iterations: reference.MinimaxPasses,
	},
	"minphase-truncation": {
		Method:     "iterative",
		Delay:      0,
		Iterations: reference.IterativePasses,
	},
	"low-group-delay": {
		Method:      "lowdelay",
		ToleranceDB: reference.LowDelayToleranceDB,
		Iterations:  reference.LowDelayIterations,
	},
}

// TestLabReproducesThePublishedComparison is the reason the comparison targets
// are worth importing at all.
//
// A visitor who selects a published target at the published tap and delay budget
// must get the numbers in docs/reference-results.csv, or the lab is a different
// experiment wearing the same labels. Because both paths call the same designers
// with the same weights on the same grid, the agreement is bit-exact and is
// asserted as such.
//
// Every published row is checked, which also makes this the guard that the lab
// still offers every target and every method the comparison publishes.
func TestLabReproducesThePublishedComparison(t *testing.T) {
	rows, err := reference.Run(0)
	if err != nil {
		t.Fatalf("reference.Run() error = %v", err)
	}

	for _, row := range rows {
		equivalent, ok := harnessEquivalent[row.Method]
		if !ok {
			t.Fatalf(
				"published method %q has no lab equivalent; the lab no longer "+
					"covers the comparison it links to",
				row.Method,
			)
		}

		t.Run(row.Target+"/"+row.Method, func(t *testing.T) {
			request := equivalent
			request.Target = row.Target
			request.Length = reference.TapCount

			design, designErr := Design(request)
			if designErr != nil {
				t.Fatalf("Design() error = %v", designErr)
			}

			metrics := design.Result.Metrics
			checks := []struct {
				name      string
				got, want float64
			}{
				{"relative magnitude error", metrics.RelativeMagnitudeError, row.RelativeMagnitudeError},
				{"RMS magnitude error dB", metrics.RMSMagnitudeErrorDB, row.RMSMagnitudeErrorDB},
				{"peak magnitude error dB", metrics.MaxMagnitudeErrorDB, row.MaxMagnitudeErrorDB},
				{"energy centroid", metrics.EnergyCentroid, row.EnergyCentroid},
				{"pre-peak energy ratio", metrics.PrePeakEnergyRatio, row.PrePeakEnergyRatio},
			}

			for _, check := range checks {
				if check.got != check.want {
					t.Errorf(
						"%s = %v, published %v (difference %g)",
						check.name,
						check.got,
						check.want,
						math.Abs(check.got-check.want),
					)
				}
			}

			if metrics.PeakIndex != row.PeakIndex {
				t.Errorf("peak index = %d, published %d", metrics.PeakIndex, row.PeakIndex)
			}

			if design.Result.Iterations != row.Iterations {
				t.Errorf(
					"iterations = %d, published %d",
					design.Result.Iterations,
					row.Iterations,
				)
			}
		})
	}
}
