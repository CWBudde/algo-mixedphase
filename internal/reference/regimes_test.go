package reference

import (
	"bytes"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/cwbudde/algo-mixedphase/mixedphase"
)

// regimeFixture runs both regime families once and shares the result, for the
// same reason sweepFixture does: the floor probe costs seconds and every test
// here only reads.
var regimeFixture = sync.OnceValues(RegimeRows)

func mustRegimes(t *testing.T) []RegimeRow {
	t.Helper()

	rows, err := regimeFixture()
	if err != nil {
		t.Fatalf("RegimeRows() error = %v", err)
	}

	return rows
}

func regimeRowsFor(t *testing.T, target, regime string) []RegimeRow {
	t.Helper()

	var selected []RegimeRow

	for _, row := range mustRegimes(t) {
		if row.Target == target && row.Regime == regime {
			selected = append(selected, row)
		}
	}

	return selected
}

// TestRegimesCoverEveryTargetAndFamily checks the artifact's shape so a
// silently truncated run cannot pass for a complete one.
func TestRegimesCoverEveryTargetAndFamily(t *testing.T) {
	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	wantMixes := len(continuumMixes())

	for _, target := range targets {
		continuum := regimeRowsFor(t, target.Name, "continuum")
		if len(continuum) != wantMixes {
			t.Errorf(
				"%s has %d continuum rows, want %d",
				target.Name,
				len(continuum),
				wantMixes,
			)
		}

		probe := regimeRowsFor(t, target.Name, "floor-probe")
		if len(probe) != len(FloorProbeTolerancesDB) {
			t.Errorf(
				"%s has %d floor-probe rows, want %d",
				target.Name,
				len(probe),
				len(FloorProbeTolerancesDB),
			)
		}
	}
}

// TestZeroDelayDesignSitsOnTheMinimumPhaseFloor is the claim the whole framing
// rests on: the group delay a magnitude request implies is the minimum-phase
// one, and the alternating factorisation at a zero budget realises exactly it.
//
// Without this, the floor figures quoted in the paper would only be a property
// of the split rather than of the target.
func TestZeroDelayDesignSitsOnTheMinimumPhaseFloor(t *testing.T) {
	const tolerance = 1e-9

	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	for _, target := range targets {
		t.Run(target.Name, func(t *testing.T) {
			zeroBudget, err := mixedphase.DesignIterative(
				target.Prototype,
				mixedphase.IterativeConfig{
					Length:     TapCount,
					Delay:      0,
					Iterations: IterativePasses,
					FFTSize:    FFTSize,
				},
			)
			if err != nil {
				t.Fatalf("DesignIterative() error = %v", err)
			}

			// Mix zero prescribes minimum phase directly, without going
			// through the split at all, so agreement between the two is
			// evidence about the target rather than about either method.
			prescribed, err := mixedphase.DesignPhaseInterpolation(
				target.Prototype,
				mixedphase.PhaseInterpolationConfig{
					Length:  TapCount,
					Mix:     0,
					FFTSize: FFTSize,
				},
			)
			if err != nil {
				t.Fatalf("DesignPhaseInterpolation() error = %v", err)
			}

			split, err := analyze(target, zeroBudget.Taps)
			if err != nil {
				t.Fatalf("analyze(split) error = %v", err)
			}

			direct, err := analyze(target, prescribed.Taps)
			if err != nil {
				t.Fatalf("analyze(direct) error = %v", err)
			}

			if math.Abs(split.meanGroupDelay-direct.meanGroupDelay) > tolerance {
				t.Errorf(
					"zero-budget mean group delay = %g, minimum-phase prescription gives %g",
					split.meanGroupDelay,
					direct.meanGroupDelay,
				)
			}
		})
	}
}

// TestContinuumGroupDelayIsLinearInMix pins the shape of the phase continuum:
// the prescribed mean group delay runs from the target's minimum-phase floor,
// through the linear-phase delay at mix one, to its reflection at maximum
// phase. The endpoints must sum to Length-1 exactly.
func TestContinuumGroupDelayIsLinearInMix(t *testing.T) {
	// The three landmarks are exact: mix zero and mix two are realisable in the
	// support to round-off, and linear phase is symmetric by construction.
	// Interior mixes are prescriptions the finite support only approximates, so
	// the realised delay tracks the straight line to a fraction of a sample
	// rather than exactly. The bound is stated in samples out of the 128 the
	// continuum spans, and the worst measured deviation across all six targets
	// is reported by the test so a regression shows up as a number.
	//
	// The bound is set by parametric-eq at 0.279 samples, roughly four times the
	// next worst: it is the target with the most group-delay structure, so its
	// truncated realisation strays furthest from the prescription. Every other
	// target stays inside 0.075 samples.
	const (
		tolerance     = 1e-6
		lineTolerance = 0.35
	)

	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	linearPhaseDelay := float64(TapCount-1) / 2

	for _, target := range targets {
		t.Run(target.Name, func(t *testing.T) {
			rows := regimeRowsFor(t, target.Name, "continuum")

			var floor, linear, ceiling float64

			for _, row := range rows {
				switch row.Mix {
				case 0:
					floor = row.MeanGroupDelay
				case 1:
					linear = row.MeanGroupDelay
				case MaximumPhaseMix:
					ceiling = row.MeanGroupDelay
				}
			}

			if math.Abs(linear-linearPhaseDelay) > tolerance {
				t.Errorf(
					"mix 1 mean group delay = %g, want the linear-phase delay %g",
					linear,
					linearPhaseDelay,
				)
			}

			if sum := floor + ceiling; math.Abs(sum-float64(TapCount-1)) > tolerance {
				t.Errorf(
					"minimum-phase %g and maximum-phase %g delays sum to %g, want %d",
					floor,
					ceiling,
					sum,
					TapCount-1,
				)
			}

			// Every interior point must lie on the straight line between the
			// two, which is what makes mix a usable latency control.
			worst, worstMix := 0.0, 0.0

			for _, row := range rows {
				want := floor + row.Mix*(linearPhaseDelay-floor)
				if deviation := math.Abs(row.MeanGroupDelay - want); deviation > worst {
					worst, worstMix = deviation, row.Mix
				}
			}

			t.Logf("worst deviation from the prescribed line: %g samples at mix %g", worst, worstMix)

			if worst > lineTolerance {
				t.Errorf(
					"worst deviation from the prescribed line = %g samples at mix %g, want <= %g",
					worst,
					worstMix,
					lineTolerance,
				)
			}
		})
	}
}

// TestContinuumRippleIsSymmetricAndVanishesAtLinearPhase is the contrast with
// the alternating factorisation. Here the delay spent above the floor buys
// group-delay flatness, monotonically, reaching exactly zero at linear phase.
// The factorisation's ripple is fixed by its minimum-phase factor and does not
// move with its budget at all.
func TestContinuumRippleIsSymmetricAndVanishesAtLinearPhase(t *testing.T) {
	const (
		tolerance      = 1e-6
		linearPhaseTol = 1e-9
	)

	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	for _, target := range targets {
		t.Run(target.Name, func(t *testing.T) {
			byMix := map[float64]RegimeRow{}
			for _, row := range regimeRowsFor(t, target.Name, "continuum") {
				byMix[row.Mix] = row
			}

			if got := byMix[1].GroupDelayRipple; got > linearPhaseTol {
				t.Errorf("linear-phase ripple = %g, want <= %g", got, linearPhaseTol)
			}

			for mix, row := range byMix {
				mirror, ok := byMix[MaximumPhaseMix-mix]
				if !ok {
					continue
				}

				if math.Abs(row.GroupDelayRipple-mirror.GroupDelayRipple) > tolerance {
					t.Errorf(
						"mix %g ripple = %g, mirrored mix %g gives %g, want symmetric",
						mix,
						row.GroupDelayRipple,
						MaximumPhaseMix-mix,
						mirror.GroupDelayRipple,
					)
				}

				// Time reversal preserves the magnitude response, so maximum
				// phase costs latency without buying accuracy.
				if math.Abs(row.RMSMagnitudeErrorDB-mirror.RMSMagnitudeErrorDB) >
					tolerance*max(1, math.Abs(row.RMSMagnitudeErrorDB)) {
					t.Errorf(
						"mix %g RMS dB error = %g, mirrored mix %g gives %g, want symmetric",
						mix,
						row.RMSMagnitudeErrorDB,
						MaximumPhaseMix-mix,
						mirror.RMSMagnitudeErrorDB,
					)
				}
			}

			// Ripple must fall monotonically from the minimum-phase end to
			// linear phase; a non-monotone leg would make mix useless as a
			// phase-linearity control.
			previous := math.Inf(1)

			for mix := 0.0; mix <= 1; mix += ContinuumMixStep {
				row, ok := byMix[mix]
				if !ok {
					t.Fatalf("continuum is missing mix %g", mix)
				}

				if row.GroupDelayRipple > previous+tolerance {
					t.Errorf(
						"ripple rose to %g at mix %g, want a monotone fall towards linear phase",
						row.GroupDelayRipple,
						mix,
					)
				}

				previous = row.GroupDelayRipple
			}
		})
	}
}

// TestFactorisationHoldsItsRippleWhileTheContinuumDescends is the contrast the
// whole framing turns on. Both families start at the same point, the target's
// minimum-phase design, and both end at linear phase. Between them they answer
// the same question differently: the continuum converts surplus delay into
// group-delay flatness monotonically, while the factorisation holds its ripple
// at the minimum-phase value and reaches flatness only when the budget has
// starved its minimum-phase factor out of existence.
//
// The factorisation's ripple is pinned because its linear factor is symmetric
// and therefore contributes exactly linear phase, so the cascade's deviation is
// its minimum-phase factor's alone.
func TestFactorisationHoldsItsRippleWhileTheContinuumDescends(t *testing.T) {
	const (
		// Fraction of the minimum-phase factor's energy left outside its
		// measured support, matching the ceiling constraint's definition.
		energyTail = 1e-6

		// The ripple must not move by more than this fraction anywhere in the
		// invariance region. It is not zero because the region's edge is
		// approximate: the energy-tail measure of "the factor still fits" is a
		// threshold on a decaying tail, not a hard boundary. deep-notch sets the
		// bound at 3.2% drift by its nominal ceiling; the other four stay under
		// 0.1%. The exact statement, that the cascade's deviation equals its
		// minimum-phase factor's, is pinned separately in adaptive_test.go.
		holdTolerance = 0.05
	)

	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	for _, target := range targets {
		t.Run(target.Name, func(t *testing.T) {
			rows := regimeRowsFor(t, target.Name, "factorisation")

			// The invariance region is exactly the ceiling constraint: the
			// factor keeps its own group delay while it still fits the
			// TapCount-2d taps the split leaves it. Beyond that the budget does
			// move the ripple, but only by destroying the factor, which costs
			// the magnitude the design was for.
			minimum, err := mixedphase.DesignPhaseInterpolation(
				target.Prototype,
				mixedphase.PhaseInterpolationConfig{
					Length:  TapCount,
					Mix:     0,
					FFTSize: FFTSize,
				},
			)
			if err != nil {
				t.Fatalf("DesignPhaseInterpolation() error = %v", err)
			}

			support := minimumPhaseSupport(minimum.Taps, energyTail)
			ceiling := (TapCount - support) / 2

			t.Logf(
				"minimum-phase support %d taps, so the budget ceiling is %d samples",
				support,
				ceiling,
			)

			if ceiling <= 0 {
				t.Skipf(
					"support %d exceeds the %d taps available, so no budget is admissible",
					support,
					TapCount,
				)
			}

			var atZero float64

			for _, row := range rows {
				if row.Delay == 0 {
					atZero = row.GroupDelayRipple
				}
			}

			for _, row := range rows {
				if row.Delay == 0 || row.Delay > ceiling {
					continue
				}

				if math.Abs(row.GroupDelayRipple-atZero) > holdTolerance*atZero {
					t.Errorf(
						"budget %d moved the ripple from %g to %g, want it held within %g relative"+
							" while the factor still fits",
						row.Delay,
						atZero,
						row.GroupDelayRipple,
						holdTolerance,
					)
				}
			}

			// Over the same latency the continuum has already converted part of
			// the deviation into flatness. That is the whole difference between
			// the two families.
			//
			// The comparison is made once, at the ceiling, because the
			// continuum is sampled every eighth of the range and its first
			// interior point can sit above a small matched latency. Reading a
			// budget below that point would compare the factorisation against
			// the continuum's own minimum-phase end.
			floor := minimumPhaseFloor(t, target)
			matched := floor + float64(ceiling)

			continuumRipple := math.Inf(1)

			for _, entry := range regimeRowsFor(t, target.Name, "continuum") {
				if entry.Mix > 0 && entry.Mix <= 1 && entry.MeanGroupDelay <= matched {
					continuumRipple = entry.GroupDelayRipple
				}
			}

			if math.IsInf(continuumRipple, 1) {
				t.Fatalf(
					"the continuum has no sampled point between %g and %g samples,"+
						" so the families cannot be compared at equal latency",
					floor,
					matched,
				)
			}

			t.Logf(
				"at %g samples: factorisation ripple %g, continuum %g",
				matched,
				atZero,
				continuumRipple,
			)

			if continuumRipple >= atZero {
				t.Errorf(
					"at %g samples the continuum ripple is %g, not below the factorisation's %g",
					matched,
					continuumRipple,
					atZero,
				)
			}
		})
	}
}

// TestFloorProbeTradesMagnitudeForDelayBelowTheFloor is the below-floor result.
// Relaxing the magnitude tolerance buys group delay under the minimum-phase
// floor on every target whose magnitude the support can actually realise, and
// the accuracy given up rises with it.
//
// steep-crossover is excluded and asserted separately: at 129 taps its
// magnitude is unrealisable to begin with, so there is no accuracy left to
// trade and the probe cannot get under the floor at all.
func TestFloorProbeTradesMagnitudeForDelayBelowTheFloor(t *testing.T) {
	const starved = "steep-crossover"

	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	for _, target := range targets {
		t.Run(target.Name, func(t *testing.T) {
			rows := regimeRowsFor(t, target.Name, "floor-probe")
			floor := minimumPhaseFloor(t, target)

			widest := rows[len(rows)-1]

			if target.Name == starved {
				if widest.MeanGroupDelay < floor {
					t.Errorf(
						"support-starved target reached %g samples, below its floor %g:"+
							" it was expected to have no accuracy left to trade",
						widest.MeanGroupDelay,
						floor,
					)
				}

				return
			}

			if widest.MeanGroupDelay >= floor {
				t.Errorf(
					"widest tolerance reached %g samples, want below the floor %g",
					widest.MeanGroupDelay,
					floor,
				)
			}

			// Delay must fall and error must rise as the tolerance widens;
			// otherwise the artifact is not a trade curve.
			for i := 1; i < len(rows); i++ {
				if rows[i].MeanGroupDelay >= rows[i-1].MeanGroupDelay {
					t.Errorf(
						"%g dB gives %g samples, not less than %g dB's %g",
						rows[i].ToleranceDB,
						rows[i].MeanGroupDelay,
						rows[i-1].ToleranceDB,
						rows[i-1].MeanGroupDelay,
					)
				}

				if rows[i].RMSMagnitudeErrorDB <= rows[i-1].RMSMagnitudeErrorDB {
					t.Errorf(
						"%g dB gives %g dB error, not more than %g dB's %g",
						rows[i].ToleranceDB,
						rows[i].RMSMagnitudeErrorDB,
						rows[i-1].ToleranceDB,
						rows[i-1].RMSMagnitudeErrorDB,
					)
				}
			}
		})
	}
}

// TestLooseToleranceLeavesTheMeasurableRegime records why
// FloorProbeTolerancesDB stops at 2 dB, so the ladder's endpoint is evidence
// rather than presentation.
//
// At 8 dB the constraint admits a spectral null. Group delay is undefined at
// such a bin, so the statistic collapses to a value with no physical meaning,
// and it does so identically under every iteration budget — which is what
// distinguishes it from an optimiser that merely needed longer.
func TestLooseToleranceLeavesTheMeasurableRegime(t *testing.T) {
	const (
		looseToleranceDB = 8.0
		saturatedErrorDB = 59.0
		impossibleDelay  = -100.0
	)

	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	target := targets[0]

	budgets := []struct {
		name       string
		iterations int
		stages     int
	}{
		{"published", LowDelayIterations, LowDelayPenaltyStages},
		{"package default", 200, 6},
	}

	for _, budget := range budgets {
		t.Run(budget.name, func(t *testing.T) {
			result, err := mixedphase.DesignLowGroupDelay(
				target.Prototype,
				mixedphase.LowGroupDelayConfig{
					Length:        TapCount,
					FFTSize:       FFTSize,
					ToleranceDB:   looseToleranceDB,
					DelayWeight:   target.DelayWeight,
					Iterations:    budget.iterations,
					PenaltyStages: budget.stages,
				},
			)
			if err != nil {
				t.Fatalf("DesignLowGroupDelay() error = %v", err)
			}

			if result.Metrics.MaxMagnitudeErrorDB < saturatedErrorDB {
				t.Errorf(
					"maximum magnitude error = %g dB, expected it saturated above %g",
					result.Metrics.MaxMagnitudeErrorDB,
					saturatedErrorDB,
				)
			}

			analysis, err := analyze(target, result.Taps)
			if err != nil {
				t.Fatalf("analyze() error = %v", err)
			}

			if analysis.meanGroupDelay > impossibleDelay {
				t.Errorf(
					"mean group delay = %g, expected the statistic to have collapsed below %g",
					analysis.meanGroupDelay,
					impossibleDelay,
				)
			}
		})
	}
}

// TestWriteRegimesCSVLeavesInapplicableColumnsEmpty guards the one place this
// artifact could mislead: a zero in the mix column of a floor-probe row would
// read as a measured minimum-phase prescription.
func TestWriteRegimesCSVLeavesInapplicableColumnsEmpty(t *testing.T) {
	var buffer bytes.Buffer

	rows := []RegimeRow{
		{Target: "t", Regime: "continuum", Mix: 0},
		{Target: "t", Regime: "floor-probe", ToleranceDB: 1},
	}

	if err := WriteRegimesCSV(&buffer, rows); err != nil {
		t.Fatalf("WriteRegimesCSV() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buffer.String()), "\n")
	if len(lines) != len(rows)+1 {
		t.Fatalf("wrote %d lines, want %d", len(lines), len(rows)+1)
	}

	if got := len(strings.Split(lines[0], ",")); got != len(regimesCSVHeader) {
		t.Errorf("header has %d fields, want %d", got, len(regimesCSVHeader))
	}

	const (
		mixColumn       = 5
		toleranceColumn = 7
	)

	continuum := strings.Split(lines[1], ",")
	if continuum[mixColumn] == "" {
		t.Error("continuum row has an empty mix column")
	}

	if continuum[toleranceColumn] != "" {
		t.Errorf("continuum row has tolerance %q, want empty", continuum[toleranceColumn])
	}

	probe := strings.Split(lines[2], ",")
	if probe[mixColumn] != "" {
		t.Errorf("floor-probe row has mix %q, want empty", probe[mixColumn])
	}

	if probe[toleranceColumn] == "" {
		t.Error("floor-probe row has an empty tolerance column")
	}
}

// TestRegimeRowBuildersWrapTheirFailures checks that a failure in either family
// is reported with the target and the setting that produced it, rather than
// surfacing as a bare design error a long way from its cause.
func TestRegimeRowBuildersWrapTheirFailures(t *testing.T) {
	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	// An empty prototype is rejected by every design entry point.
	broken := Target{Name: "broken", DelayWeight: targets[0].DelayWeight}

	t.Run("continuum", func(t *testing.T) {
		_, err := continuumRow(broken, 0.5)
		if err == nil {
			t.Fatal("continuumRow() on an empty prototype returned nil error")
		}

		if !strings.Contains(err.Error(), "broken") ||
			!strings.Contains(err.Error(), "0.5") {
			t.Errorf("error %q names neither the target nor the mix", err)
		}
	})

	t.Run("floor probe", func(t *testing.T) {
		_, err := floorProbeRow(broken, 1)
		if err == nil {
			t.Fatal("floorProbeRow() on an empty prototype returned nil error")
		}

		if !strings.Contains(err.Error(), "broken") {
			t.Errorf("error %q does not name the target", err)
		}
	})

	t.Run("analysis", func(t *testing.T) {
		// A delay weight built for another grid makes the analysis, rather than
		// the design, the failing step.
		mismatched := Target{
			Name:        "mismatched",
			Prototype:   targets[0].Prototype,
			DelayWeight: make([]float64, 3),
		}

		_, err := regimeRowFrom(mismatched, "continuum", mixedphase.Result{
			Taps: targets[0].Prototype,
		})
		if err == nil {
			t.Fatal("regimeRowFrom() on a mismatched weight grid returned nil error")
		}

		if !strings.Contains(err.Error(), "mismatched") {
			t.Errorf("error %q does not name the target", err)
		}
	})
}

// TestCommittedRegimesCSVIsReproducible byte-compares the committed artifact
// against fresh generator output, the same gate the other CSVs carry.
func TestCommittedRegimesCSVIsReproducible(t *testing.T) {
	rows := mustRegimes(t)

	var buffer bytes.Buffer
	if err := WriteRegimesCSV(&buffer, rows); err != nil {
		t.Fatalf("WriteRegimesCSV() error = %v", err)
	}

	assertCommittedCSV(t, "reference-phase-regimes.csv", buffer.Bytes())
}

// minimumPhaseFloor reports the target's minimum-phase mean group delay, the
// delay no realisation of its magnitude can beat.
func minimumPhaseFloor(t *testing.T, target Target) float64 {
	t.Helper()

	for _, row := range regimeRowsFor(t, target.Name, "continuum") {
		if row.Mix == 0 {
			return row.MeanGroupDelay
		}
	}

	t.Fatalf("no minimum-phase row for %s", target.Name)

	return 0
}
