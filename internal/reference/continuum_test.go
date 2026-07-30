package reference

import (
	"bytes"
	"math"
	"sync"
	"testing"
)

var (
	continuumOnce sync.Once
	continuumAll  []ContinuumRow
	continuumErr  error

	continuumImpulseOnce sync.Once
	continuumImpulseAll  []ContinuumImpulseRow
	continuumImpulseErr  error
)

// mustContinuumImpulses builds the continuum impulse artifact once for the whole
// test binary, for the same reason as mustContinuum.
func mustContinuumImpulses(t *testing.T) []ContinuumImpulseRow {
	t.Helper()

	continuumImpulseOnce.Do(func() {
		continuumImpulseAll, continuumImpulseErr = ContinuumImpulseRows()
	})

	if continuumImpulseErr != nil {
		t.Fatalf("ContinuumImpulseRows() error = %v", continuumImpulseErr)
	}

	return continuumImpulseAll
}

// mustContinuum builds the continuum artifact once for the whole test binary.
// The out-of-window rows each run a penalty ladder, so regenerating them per
// test would dominate the package's runtime.
func mustContinuum(t *testing.T) []ContinuumRow {
	t.Helper()

	continuumOnce.Do(func() {
		continuumAll, continuumErr = ContinuumRows()
	})

	if continuumErr != nil {
		t.Fatalf("ContinuumRows() error = %v", continuumErr)
	}

	return continuumAll
}

// continuumRowsFor returns the rows of one target, in artifact order.
func continuumRowsFor(t *testing.T, target string) []ContinuumRow {
	t.Helper()

	var out []ContinuumRow

	for _, row := range mustContinuum(t) {
		if row.Target == target {
			out = append(out, row)
		}
	}

	if len(out) == 0 {
		t.Fatalf("no continuum rows for %s", target)
	}

	return out
}

// TestContinuumArtifactCoversBothBranches pins the shape of the artifact: every
// target must exercise all three regimes, or the figures built from it would be
// showing a continuum with a missing end.
func TestContinuumArtifactCoversBothBranches(t *testing.T) {
	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	for _, target := range targets {
		counts := map[string]int{}
		for _, row := range continuumRowsFor(t, target.Name) {
			counts[row.Regime]++
		}

		if counts["window"] != ContinuumWindowPoints {
			t.Errorf(
				"%s has %d window rows, want %d",
				target.Name,
				counts["window"],
				ContinuumWindowPoints,
			)
		}

		for _, regime := range []string{"sub-minimum", "super-maximum"} {
			if counts[regime] != len(ContinuumOutsideFractions) {
				t.Errorf(
					"%s has %d %s rows, want %d",
					target.Name,
					counts[regime],
					regime,
					len(ContinuumOutsideFractions),
				)
			}
		}
	}
}

// TestContinuumRequestIsMetOnEveryRow is the artifact-level statement of what the
// knob promises: the delay a caller asks for is the delay the filter has, on
// both branches and on every published target.
//
// The two branches meet it for different reasons. Inside the window the request
// is inverted in closed form and the miss is the least-squares projection
// residual; outside it the penalty ladder drives the delay constraint, and the
// miss is what the ladder had converged to at the published budget.
func TestContinuumRequestIsMetOnEveryRow(t *testing.T) {
	// The two branches meet the request to different precisions, and the
	// difference is structural rather than incidental: a closed-form inverse
	// inherits the projection residual, while a penalty ladder drives the
	// constraint until it is satisfied. Holding them to one tolerance would hide
	// which of the two is being measured.
	tolerances := map[string]float64{
		"window":        0.3,
		"sub-minimum":   1e-3,
		"super-maximum": 1e-3,
	}

	worst := map[string]float64{}
	worstRow := map[string]ContinuumRow{}

	for _, row := range mustContinuum(t) {
		miss := math.Abs(row.MeanGroupDelay - row.RequestedDelay)
		if miss > worst[row.Regime] {
			worst[row.Regime] = miss
			worstRow[row.Regime] = row
		}
	}

	for regime, tolerance := range tolerances {
		if worst[regime] > tolerance {
			row := worstRow[regime]
			t.Errorf(
				"worst %s delay miss is %.4f samples on %s (requested %.4f, "+
					"achieved %.4f), above the %g-sample tolerance",
				regime,
				worst[regime],
				row.Target,
				row.RequestedDelay,
				row.MeanGroupDelay,
				tolerance,
			)
		}
	}
}

// TestContinuumAffineLawResidualIsSmall pins the identity the in-window regime is
// built on. The law is exact for the prescribed phase, so what this bounds is the
// error the projection onto a finite support introduces — the quantity that
// decides whether a requested delay is reachable in closed form or would need a
// search.
func TestContinuumAffineLawResidualIsSmall(t *testing.T) {
	// Three tenths of a sample across windows up to 127 samples wide, which is
	// a quarter of a percent. The worst case is the parametric EQ near the
	// maximum-phase end, where the minimum phase it interpolates is steepest.
	const tolerance = 0.3

	worst, worstRow := 0.0, ContinuumRow{}
	checked := 0

	for _, row := range mustContinuum(t) {
		if row.Regime != "window" {
			continue
		}

		checked++

		residual := math.Abs(row.MeanGroupDelay - row.PredictedDelay)
		if residual > worst {
			worst, worstRow = residual, row
		}
	}

	if checked == 0 {
		t.Fatal("no window rows to check the affine delay law against")
	}

	if worst > tolerance {
		t.Errorf(
			"worst affine-law residual is %.4f samples on %s at mix %.4f "+
				"(predicted %.4f, achieved %.4f), above the %.2f-sample "+
				"tolerance; the closed-form inverse of the law would no longer "+
				"be an adequate way to meet a request",
			worst,
			worstRow.Target,
			worstRow.Mix,
			worstRow.PredictedDelay,
			worstRow.MeanGroupDelay,
			tolerance,
		)
	}
}

// TestContinuumWindowEdgesAreTheFloorAndItsReflection pins the reachable window
// against the measured floor, and with it the claim that the window is a property
// of the requested magnitude rather than of the tap count.
func TestContinuumWindowEdgesAreTheFloorAndItsReflection(t *testing.T) {
	span := float64(TapCount - 1)

	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	for _, target := range targets {
		rows := continuumRowsFor(t, target.Name)

		var window []ContinuumRow

		for _, row := range rows {
			if row.Regime == "window" {
				window = append(window, row)
			}
		}

		first, last := window[0], window[len(window)-1]

		if math.Abs(first.RequestedDelay-first.MinimumPhaseDelay) > 1e-9 {
			t.Errorf(
				"%s: window starts at %.6f, want the floor %.6f",
				target.Name,
				first.RequestedDelay,
				first.MinimumPhaseDelay,
			)
		}

		reflection := span - last.MinimumPhaseDelay
		if math.Abs(last.RequestedDelay-reflection) > 1e-9 {
			t.Errorf(
				"%s: window ends at %.6f, want the floor's reflection %.6f",
				target.Name,
				last.RequestedDelay,
				reflection,
			)
		}
	}
}

// TestContinuumRippleVanishesAtLinearPhaseOnly pins the one point of the
// continuum where group delay is exactly flat, and the monotone descent towards
// it. This is the axis on which the knob differs from a delay budget, whose
// ripple is fixed by its minimum-phase factor and does not respond at all.
func TestContinuumRippleVanishesAtLinearPhaseOnly(t *testing.T) {
	centre := float64(TapCount-1) / 2

	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	for _, target := range targets {
		previous := math.Inf(1)

		for _, row := range continuumRowsFor(t, target.Name) {
			if row.Regime != "window" {
				continue
			}

			atCentre := math.Abs(row.RequestedDelay-centre) < 1e-9

			switch {
			case atCentre && row.GroupDelayRipple > 1e-9:
				t.Errorf(
					"%s: ripple at linear phase is %g, want zero",
					target.Name,
					row.GroupDelayRipple,
				)
			case !atCentre && row.GroupDelayRipple <= 1e-9:
				t.Errorf(
					"%s: ripple vanishes at %.4f samples, away from linear "+
						"phase at %.4f",
					target.Name,
					row.RequestedDelay,
					centre,
				)
			}

			if row.RequestedDelay <= centre {
				if row.GroupDelayRipple > previous {
					t.Errorf(
						"%s: ripple rises from %g to %g approaching linear "+
							"phase at %.4f samples; the descent must be "+
							"monotone",
						target.Name,
						previous,
						row.GroupDelayRipple,
						row.RequestedDelay,
					)
				}

				previous = row.GroupDelayRipple
			}
		}
	}
}

// TestContinuumWindowIsSymmetricAboutLinearPhase pins the reflection symmetry
// where it is an identity: inside the window the design is a prescribed phase, and
// prescribing the mirrored mix reverses the filter exactly, which cannot change a
// magnitude.
//
// The comparison is absolute as well as relative because the endpoint errors run
// down to 1e-10, where a relative test would only be measuring the last bits of
// two separately projected designs.
func TestContinuumWindowIsSymmetricAboutLinearPhase(t *testing.T) {
	span := float64(TapCount - 1)

	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	for _, target := range targets {
		rows := continuumRowsFor(t, target.Name)

		for _, row := range rows {
			if row.Regime != "window" {
				continue
			}

			mirror, found := findContinuumRow(rows, span-row.RequestedDelay)
			if !found {
				t.Errorf(
					"%s: no row mirrors the window request at %.6f samples",
					target.Name,
					row.RequestedDelay,
				)

				continue
			}

			gap := math.Abs(
				row.RelativeMagnitudeError - mirror.RelativeMagnitudeError,
			)
			if gap > 1e-9 && relativeGap(
				row.RelativeMagnitudeError,
				mirror.RelativeMagnitudeError,
			) > 1e-6 {
				t.Errorf(
					"%s: relative magnitude error at %.4f is %g but %g at its "+
						"reflection %.4f",
					target.Name,
					row.RequestedDelay,
					row.RelativeMagnitudeError,
					mirror.RelativeMagnitudeError,
					mirror.RequestedDelay,
				)
			}
		}
	}
}

// TestOutOfWindowBranchesAreOnlyApproximatelySymmetric records a limit of the
// method rather than a property of it, and is deliberately a loose bound.
//
// Outside the window the two branches are independent penalty-ladder solves of a
// non-convex objective, one of them on the reflected request. Reversal cannot
// change a magnitude, so a global optimiser would return mirrored errors exactly;
// a local one returns whichever optimum its path reached. On the published targets
// the two agree closely at moderate requests — often to ten digits — and diverge
// at the most aggressive one, where the parametric EQ reports 0.0655 against
// 0.0605 for the same request read from either end. Sub-floor and beyond-maximum
// numbers are therefore local optima and should be read with the optimiser budget
// that produced them, not as the best a magnitude concession could do.
func TestOutOfWindowBranchesAreOnlyApproximatelySymmetric(t *testing.T) {
	span := float64(TapCount - 1)

	// Half of the larger value. Anything worse would mean the two tails are not
	// describing the same trade at all.
	const bound = 0.5

	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	for _, target := range targets {
		rows := continuumRowsFor(t, target.Name)

		for _, row := range rows {
			if row.Regime == "window" {
				continue
			}

			mirror, found := findContinuumRow(rows, span-row.RequestedDelay)
			if !found {
				t.Errorf(
					"%s: no row mirrors the request at %.6f samples",
					target.Name,
					row.RequestedDelay,
				)

				continue
			}

			gap := relativeGap(
				row.RelativeMagnitudeError,
				mirror.RelativeMagnitudeError,
			)
			if gap > bound {
				t.Errorf(
					"%s: relative magnitude error at %.4f is %g but %g at its "+
						"reflection %.4f (gap %.3f); the two tails should at "+
						"least describe the same trade",
					target.Name,
					row.RequestedDelay,
					row.RelativeMagnitudeError,
					mirror.RelativeMagnitudeError,
					mirror.RequestedDelay,
					gap,
				)
			}
		}
	}
}

// findContinuumRow locates the row whose request matches delay.
func findContinuumRow(
	rows []ContinuumRow,
	delay float64,
) (ContinuumRow, bool) {
	for _, row := range rows {
		if math.Abs(row.RequestedDelay-delay) < 1e-6 {
			return row, true
		}
	}

	return ContinuumRow{}, false
}

// relativeGap is the difference of two quantities relative to the larger.
func relativeGap(a, b float64) float64 {
	scale := max(math.Abs(a), math.Abs(b))
	if scale == 0 {
		return 0
	}

	return math.Abs(a-b) / scale
}

// TestContinuumEndpointsAreTheMostAccuratePoints pins the accuracy result the
// paper leads with: the phase-pure ends of the continuum are not only the fastest
// and the slowest realisations of a magnitude, they are also the closest to it.
//
// The steep crossover is excluded because it is support-starved at 129 taps.
// Truncation error there dominates every phase choice, which is itself pinned by
// TestSupportStarvedTargetIsIndifferentToPhase.
func TestContinuumEndpointsAreTheMostAccuratePoints(t *testing.T) {
	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	for _, target := range targets {
		if target.Name == "steep-crossover" {
			continue
		}

		rows := continuumRowsFor(t, target.Name)

		var window []ContinuumRow

		for _, row := range rows {
			if row.Regime == "window" {
				window = append(window, row)
			}
		}

		endpoint := window[0].RelativeMagnitudeError

		worst := 0.0
		for _, row := range window[1 : len(window)-1] {
			worst = max(worst, row.RelativeMagnitudeError)
		}

		if endpoint >= worst {
			t.Errorf(
				"%s: minimum-phase relative error %g is not below the worst "+
					"interior value %g; a spectral factor of the target should "+
					"need no compromise where an intermediate phase does",
				target.Name,
				endpoint,
				worst,
			)
		}
	}
}

// TestSupportStarvedTargetIsIndifferentToPhase pins the exception, and the
// warning that goes with it. The steep crossover's magnitude is not realisable in
// 129 taps, so its reachable window is narrow and its accuracy curve is flat: the
// knob still moves the delay but no longer trades anything for it.
func TestSupportStarvedTargetIsIndifferentToPhase(t *testing.T) {
	lowest, highest := math.Inf(1), 0.0

	for _, row := range continuumRowsFor(t, "steep-crossover") {
		if row.Regime != "window" {
			continue
		}

		lowest = min(lowest, row.RelativeMagnitudeError)
		highest = max(highest, row.RelativeMagnitudeError)
	}

	const flat = 2.0

	if highest/lowest > flat {
		t.Errorf(
			"steep-crossover relative error spans %g to %g across its window, "+
				"a ratio of %.2f; a support-starved target is expected to be "+
				"nearly indifferent to phase",
			lowest,
			highest,
			highest/lowest,
		)
	}
}

// TestContinuumWindowNarrowsWithTheFloor pins the ordering behind the reachable
// window: the harder the magnitude is to realise, the higher its floor and the
// less phase freedom is left. It is what makes the window a diagnostic rather
// than a formality.
func TestContinuumWindowNarrowsWithTheFloor(t *testing.T) {
	span := float64(TapCount - 1)

	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	for _, target := range targets {
		rows := continuumRowsFor(t, target.Name)
		floor := rows[0].MinimumPhaseDelay
		width := span - 2*floor

		if width <= 0 {
			t.Errorf(
				"%s: floor %.4f leaves no window in %d taps",
				target.Name,
				floor,
				TapCount,
			)
		}
	}

	steep := continuumRowsFor(t, "steep-crossover")[0].MinimumPhaseDelay
	lowPass := continuumRowsFor(t, "low-pass")[0].MinimumPhaseDelay

	if steep <= lowPass {
		t.Errorf(
			"steep crossover floor %.4f does not exceed the low-pass floor "+
				"%.4f; the steepest target should have the least phase freedom",
			steep,
			lowPass,
		)
	}
}

// TestCommittedContinuumCSVIsReproducible byte-compares the committed artifact
// against fresh generator output, the same gate the other CSVs carry.
func TestCommittedContinuumCSVIsReproducible(t *testing.T) {
	var buffer bytes.Buffer
	if err := WriteContinuumCSV(&buffer, mustContinuum(t)); err != nil {
		t.Fatalf("WriteContinuumCSV() error = %v", err)
	}

	assertCommittedCSV(t, "reference-continuum.csv", buffer.Bytes())
}

// TestCommittedContinuumImpulseCSVIsReproducible gates the impulse artifact.
func TestCommittedContinuumImpulseCSVIsReproducible(t *testing.T) {
	var buffer bytes.Buffer
	if err := WriteContinuumImpulseCSV(&buffer, mustContinuumImpulses(t)); err != nil {
		t.Fatalf("WriteContinuumImpulseCSV() error = %v", err)
	}

	assertCommittedCSV(t, "reference-continuum-impulse.csv", buffer.Bytes())
}

// TestContinuumImpulseReversesAtMaximumPhase pins the time-domain statement of
// the reflection symmetry, which is what the impulse figure shows: the design at
// the far edge of the window is the design at the near edge read backwards.
func TestContinuumImpulseReversesAtMaximumPhase(t *testing.T) {
	rows := mustContinuumImpulses(t)

	span := float64(TapCount - 1)

	byRequest := map[string][]float64{}
	requests := map[string][]float64{}

	for _, row := range rows {
		key := row.Target + "@" + formatFloat(row.RequestedDelay)
		byRequest[key] = append(byRequest[key], row.Coefficient)

		if row.SampleIndex == 0 {
			requests[row.Target] = append(
				requests[row.Target],
				row.RequestedDelay,
			)
		}
	}

	for target, delays := range requests {
		fastest, slowest := math.Inf(1), 0.0

		for _, delay := range delays {
			// The sub-floor snapshot has no reflection in this artifact.
			if math.Abs(delay+delay-span) > span {
				continue
			}

			fastest = min(fastest, delay)
			slowest = max(slowest, delay)
		}

		if math.Abs(fastest+slowest-span) > 1e-6 {
			continue
		}

		fast := byRequest[target+"@"+formatFloat(fastest)]
		slow := byRequest[target+"@"+formatFloat(slowest)]

		peak := 0.0
		for _, tap := range fast {
			peak = max(peak, math.Abs(tap))
		}

		worst := 0.0
		for i, tap := range fast {
			worst = max(worst, math.Abs(tap-slow[len(slow)-1-i]))
		}

		if worst > peak*1e-9 {
			t.Errorf(
				"%s: the design at %.4f samples is not the reverse of the one "+
					"at %.4f; worst deviation %g against a peak tap of %g",
				target,
				slowest,
				fastest,
				worst,
				peak,
			)
		}
	}
}

// TestContinuumGeneratorReportsBrokenTargets covers the error paths of the
// generator. They are unreachable through the published fixtures by design, so a
// deliberately malformed target is the only way to exercise them.
func TestContinuumGeneratorReportsBrokenTargets(t *testing.T) {
	valid, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	empty := Target{Name: "empty-prototype", DelayWeight: valid[0].DelayWeight}

	// A weight vector of the wrong length passes the design and fails the
	// analysis, which is the other error path each row helper carries.
	mismatched := Target{
		Name:        "mismatched-weight",
		Prototype:   valid[0].Prototype,
		DelayWeight: valid[0].DelayWeight[:1],
	}

	if _, err := continuumFloor(empty); err == nil {
		t.Error("continuumFloor accepted an empty prototype")
	}

	if _, err := continuumFloor(mismatched); err == nil {
		t.Error("continuumFloor accepted a mismatched delay weight")
	}

	if _, err := designContinuumAt(empty, 1); err == nil {
		t.Error("designContinuumAt accepted an empty prototype")
	}

	if _, err := continuumRowAt(empty, 1, 1); err == nil {
		t.Error("continuumRowAt accepted an empty prototype")
	}

	if _, err := continuumRowAt(mismatched, 1, 1); err == nil {
		t.Error("continuumRowAt accepted a mismatched delay weight")
	}
}
