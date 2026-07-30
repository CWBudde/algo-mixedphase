package reference

import (
	"math"
	"testing"

	algofft "github.com/cwbudde/algo-fft"
	"github.com/cwbudde/algo-mixedphase/mixedphase"
)

// deepStopbandDB is the mean realised level over the bins where the target has
// already fallen below floorDB.
//
// This is the axis the alternating factorisation actually competes on, and no
// published column carries it: the RMS dB error aggregates it with the passband
// and the relative error is dominated by the passband entirely.
func deepStopbandDB(
	plan *algofft.Plan[complex128],
	targetDB, taps []float64,
	floorDB float64,
) (float64, error) {
	spectrum, err := realSpectrum(plan, taps)
	if err != nil {
		return 0, err
	}

	sum, count := 0.0, 0

	for bin := range FFTSize/2 + 1 {
		if targetDB[bin] <= floorDB {
			sum += magnitudeDB(spectrum[bin])
			count++
		}
	}

	if count == 0 {
		return 0, nil
	}

	return sum / float64(count), nil
}

func targetLevelsDB(
	plan *algofft.Plan[complex128],
	target Target,
) ([]float64, error) {
	spectrum, err := realSpectrum(plan, target.Prototype)
	if err != nil {
		return nil, err
	}

	levels := make([]float64, FFTSize/2+1)
	for bin := range levels {
		levels[bin] = magnitudeDB(spectrum[bin])
	}

	return levels, nil
}

// TestAdaptiveDelaySelectionBeatsTheFixedBudget pins the numbers the package
// docs quote for mixedphase.DesignIterativeAuto against these fixtures.
//
// Budget: TapCount taps on the FFTSize grid with IterativePasses correction
// passes per candidate, and the designer's default coarse step and relative
// error slack. The delay budget is an output here, not an input, which is the
// whole point: DelayBudget is not passed.
//
// The two halves of the claim are that the search declines to spend delay on
// every target whose minimum-phase factor fits its share of the taps — where the
// published budde-iterative row pays 16 samples for nothing — and that it
// outperforms the hand-picked budget on the one target that is starved.
func TestAdaptiveDelaySelectionBeatsTheFixedBudget(t *testing.T) {
	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	// want is the selected delay and, where a non-zero one is selected, the RMS
	// dB magnitude error it reaches.
	want := map[string]struct {
		delay int
		rmsDB float64
	}{
		"low-pass":        {delay: 0},
		"parametric-eq":   {delay: 0},
		"crossover":       {delay: 0},
		"deep-notch":      {delay: 0},
		"room-correction": {delay: 0},
		"steep-crossover": {delay: 22, rmsDB: 3.309975},
	}

	for _, target := range targets {
		t.Run(target.Name, func(t *testing.T) {
			expected, ok := want[target.Name]
			if !ok {
				t.Fatalf("no expectation recorded for target %q", target.Name)
			}

			adaptive, designErr := mixedphase.DesignIterativeAuto(
				target.Prototype,
				mixedphase.AutoIterativeConfig{
					Length:     TapCount,
					FFTSize:    FFTSize,
					Iterations: IterativePasses,
				},
			)
			if designErr != nil {
				t.Fatalf("DesignIterativeAuto() error = %v", designErr)
			}

			if adaptive.Delay != expected.delay {
				t.Errorf(
					"selected delay = %d, want %d",
					adaptive.Delay,
					expected.delay,
				)
			}

			baseline, designErr := mixedphase.DesignIterative(
				target.Prototype,
				mixedphase.IterativeConfig{
					Length:     TapCount,
					Delay:      0,
					Iterations: IterativePasses,
					FFTSize:    FFTSize,
				},
			)
			if designErr != nil {
				t.Fatalf("DesignIterative() error = %v", designErr)
			}

			// The guarantee: never worse than minimum-phase truncation on the
			// objective. On the five fitting targets it must be exactly equal,
			// because selecting zero returns that design unchanged.
			if adaptive.Metrics.RMSMagnitudeErrorDB >
				baseline.Metrics.RMSMagnitudeErrorDB {
				t.Errorf(
					"selected %.6f dB, worse than minimum-phase truncation's %.6f dB",
					adaptive.Metrics.RMSMagnitudeErrorDB,
					baseline.Metrics.RMSMagnitudeErrorDB,
				)
			}

			if expected.delay == 0 {
				if adaptive.Metrics.RMSMagnitudeErrorDB !=
					baseline.Metrics.RMSMagnitudeErrorDB {
					t.Errorf(
						"a zero selection produced %.9g dB rather than the "+
							"minimum-phase design's %.9g dB",
						adaptive.Metrics.RMSMagnitudeErrorDB,
						baseline.Metrics.RMSMagnitudeErrorDB,
					)
				}

				return
			}

			if math.Abs(adaptive.Metrics.RMSMagnitudeErrorDB-expected.rmsDB) > 5e-6 {
				t.Errorf(
					"RMS magnitude error = %.6f dB, want %.6f dB",
					adaptive.Metrics.RMSMagnitudeErrorDB,
					expected.rmsDB,
				)
			}

			analysis, analyzeErr := analyze(target, adaptive.Taps)
			if analyzeErr != nil {
				t.Fatalf("analyze() error = %v", analyzeErr)
			}

			t.Logf(
				"delay %d: %.6f dB RMS, %.6f relative, mean group delay %.2f samples",
				adaptive.Delay,
				adaptive.Metrics.RMSMagnitudeErrorDB,
				adaptive.Metrics.RelativeMagnitudeError,
				analysis.meanGroupDelay,
			)
		})
	}
}

// TestSmallDelayBudgetsAreTheWorstChoice pins the number the package docs and
// mixedphase.defaultRelativeErrorSlack both cite as the reason a delay budget
// should not be guessed.
//
// Budget: as above, TapCount taps on the FFTSize grid with IterativePasses
// passes. On the support-starved target a single sample of delay is not a cheap
// compromise between the two extremes — it is far worse than either, because a
// three-tap linear factor forced to carry the whole residual quotient cannot
// approximate it at all.
func TestSmallDelayBudgetsAreTheWorstChoice(t *testing.T) {
	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	target, err := findTarget(targets, DegenerateContrastTarget)
	if err != nil {
		t.Fatalf("findTarget() error = %v", err)
	}

	relativeErrorAt := func(delay int) float64 {
		result, designErr := mixedphase.DesignIterative(
			target.Prototype,
			mixedphase.IterativeConfig{
				Length:     TapCount,
				Delay:      delay,
				Iterations: IterativePasses,
				FFTSize:    FFTSize,
			},
		)
		if designErr != nil {
			t.Fatalf("delay %d: DesignIterative() error = %v", delay, designErr)
		}

		return result.Metrics.RelativeMagnitudeError
	}

	cases := []struct {
		delay int
		want  float64
	}{
		{delay: 0, want: 0.012269},
		{delay: 1, want: 0.774710},
		{delay: 22, want: 0.026394},
	}

	for _, testCase := range cases {
		got := relativeErrorAt(testCase.delay)
		if math.Abs(got-testCase.want) > 5e-6 {
			t.Errorf(
				"delay %d: relative magnitude error = %.6f, want %.6f",
				testCase.delay,
				got,
				testCase.want,
			)
		}
	}

	// The claim that matters is ordinal, not the exact figures: one sample must
	// be worse than both neighbours it sits between.
	one, zero, selected := relativeErrorAt(1), relativeErrorAt(0), relativeErrorAt(22)
	if one <= zero || one <= selected {
		t.Errorf(
			"a one-sample budget (%.6f) is no longer worse than both zero "+
				"(%.6f) and the selected 22 (%.6f); the warning in the "+
				"mixedphase package docs has stopped being true",
			one,
			zero,
			selected,
		)
	}
}

// TestAdaptiveSelectionBuysStopbandDepthForLatency pins the trade the
// mixedphase.DesignIterativeAuto doc comment quotes: what the selected budget
// costs in latency and what it returns in rejection.
//
// Budget: TapCount taps on the FFTSize grid with IterativePasses passes per
// candidate and the designer's defaults. The stopband figure is the mean realised
// level over the bins where the target is already at or below -80 dB.
func TestAdaptiveSelectionBuysStopbandDepthForLatency(t *testing.T) {
	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	target, err := findTarget(targets, DegenerateContrastTarget)
	if err != nil {
		t.Fatalf("findTarget() error = %v", err)
	}

	plan, err := algofft.NewPlan64(FFTSize)
	if err != nil {
		t.Fatalf("NewPlan64() error = %v", err)
	}

	levels, err := targetLevelsDB(plan, target)
	if err != nil {
		t.Fatalf("targetLevelsDB() error = %v", err)
	}

	adaptive, err := mixedphase.DesignIterativeAuto(
		target.Prototype,
		mixedphase.AutoIterativeConfig{
			Length:     TapCount,
			FFTSize:    FFTSize,
			Iterations: IterativePasses,
		},
	)
	if err != nil {
		t.Fatalf("DesignIterativeAuto() error = %v", err)
	}

	baseline, err := mixedphase.DesignIterative(
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

	adaptiveStop, err := deepStopbandDB(plan, levels, adaptive.Taps, -80)
	if err != nil {
		t.Fatalf("deepStopbandDB() error = %v", err)
	}

	baselineStop, err := deepStopbandDB(plan, levels, baseline.Taps, -80)
	if err != nil {
		t.Fatalf("deepStopbandDB() error = %v", err)
	}

	adaptiveAnalysis, err := analyze(target, adaptive.Taps)
	if err != nil {
		t.Fatalf("analyze() error = %v", err)
	}

	baselineAnalysis, err := analyze(target, baseline.Taps)
	if err != nil {
		t.Fatalf("analyze() error = %v", err)
	}

	gainDB := baselineStop - adaptiveStop
	latencyCost := adaptiveAnalysis.meanGroupDelay - baselineAnalysis.meanGroupDelay

	t.Logf(
		"stopband %.1f dB -> %.1f dB (%.1f dB deeper) for %.2f samples of mean group delay",
		baselineStop,
		adaptiveStop,
		gainDB,
		latencyCost,
	)

	if math.Abs(gainDB-70.7) > 0.5 {
		t.Errorf("stopband gain = %.1f dB, want 70.7 dB", gainDB)
	}

	if math.Abs(latencyCost-2.63) > 0.05 {
		t.Errorf("mean group delay cost = %.2f samples, want 2.63", latencyCost)
	}
}

// guardDefectLength is an output length at which the purely multiplicative
// passband guard misfires on the committed eighth-order crossover fixture.
//
// It is not the published TapCount, and deliberately so: at 129 taps the guard
// never binds, which is why the defect went unnoticed. Nothing here touches the
// committed CSVs.
const guardDefectLength = 193

// TestAdaptiveGuardDoesNotRejectAMuchBetterDesign pins the fix for a defect in
// the passband guard of [mixedphase.DesignIterativeAuto].
//
// The guard bounds how much linear-magnitude accuracy a delay budget may trade
// away, as a multiple of the zero-delay design's error. That form is meaningless
// once the zero-delay design is already accurate. Designing steep-crossover into
// 193 taps puts its zero-delay relative error near 2e-3, so a three-times ceiling
// sits near 7e-3 — and that rejects the eleven-sample budget, whose relative
// error is 7.5e-3 but whose RMS dB error is sixteen decibels better. The guard
// would trade 16 dB of stopband depth for a passband regression of under a
// hundredth of a decibel.
//
// The absolute floor keeps the guard active only where the relative form still
// carries meaning. Budget: the committed steep-crossover prototype designed into
// guardDefectLength taps on the FFTSize grid with IterativePasses correction
// passes and an exhaustive delay scan, so the comparison isolates the guard
// rather than the stride.
func TestAdaptiveGuardDoesNotRejectAMuchBetterDesign(t *testing.T) {
	prototype := prototypeNamed(t, DegenerateContrastTarget)

	design := func(floor float64) mixedphase.Result {
		t.Helper()

		result, err := mixedphase.DesignIterativeAuto(
			prototype,
			mixedphase.AutoIterativeConfig{
				Length:             guardDefectLength,
				FFTSize:            FFTSize,
				Iterations:         IterativePasses,
				CoarseStep:         1,
				RelativeErrorFloor: floor,
			},
		)
		if err != nil {
			t.Fatalf("DesignIterativeAuto() error = %v", err)
		}

		return result
	}

	// A floor small enough to be inert recovers the old purely multiplicative
	// ceiling, which is the behaviour being guarded against.
	multiplicative := design(1e-300)
	floored := design(0)

	if multiplicative.Delay != 0 {
		t.Fatalf(
			"the multiplicative ceiling selected delay %d, so it no longer "+
				"misfires here and this test no longer pins the defect",
			multiplicative.Delay,
		)
	}

	if floored.Delay == 0 {
		t.Fatal("the floored guard still declines a budget worth sixteen decibels")
	}

	improvement := multiplicative.Metrics.RMSMagnitudeErrorDB -
		floored.Metrics.RMSMagnitudeErrorDB
	if improvement < 10 {
		t.Errorf(
			"floored guard recovered only %.3f dB (%.4f at delay 0 -> %.4f at "+
				"delay %d)",
			improvement,
			multiplicative.Metrics.RMSMagnitudeErrorDB,
			floored.Metrics.RMSMagnitudeErrorDB,
			floored.Delay,
		)
	}

	// The recovered design must still be accurate in absolute terms. The point is
	// that the guard was measuring the wrong thing, not that it should be absent.
	if floored.Metrics.RelativeMagnitudeError > 1e-2 {
		t.Errorf(
			"selected design has relative magnitude error %g, above the 1e-2 floor",
			floored.Metrics.RelativeMagnitudeError,
		)
	}
}

// TestAdaptiveDelayBudgetCannotFlattenGroupDelay pins a hard limit of the
// construction, and the reason this package measures magnitude rather than phase
// when it selects a delay budget.
//
// The linear-phase factor is symmetric, so it contributes exactly linear phase.
// The cascade's group-delay deviation from linear is therefore its minimum-phase
// factor's deviation, and raising the delay budget cannot reduce it — it only
// shifts the whole curve. Spending latency to flatten group delay does not work
// in this structure, whatever the budget.
//
// Budget: every committed target designed into TapCount taps on the FFTSize grid
// with IterativePasses correction passes, over the delays the split admits.
func TestAdaptiveDelayBudgetCannotFlattenGroupDelay(t *testing.T) {
	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	for _, target := range targets {
		t.Run(target.Name, func(t *testing.T) {
			var referenceDelay, referenceRipple float64

			for index, delay := range []int{0, 8, 16, 24, 32} {
				result, designErr := mixedphase.DesignIterative(
					target.Prototype,
					mixedphase.IterativeConfig{
						Length:     TapCount,
						Delay:      delay,
						Iterations: IterativePasses,
						FFTSize:    FFTSize,
					},
				)
				if designErr != nil {
					t.Fatalf("delay %d: DesignIterative() error = %v", delay, designErr)
				}

				cascade, analyzeErr := analyze(target, result.Taps)
				if analyzeErr != nil {
					t.Fatalf("delay %d: analyze() error = %v", delay, analyzeErr)
				}

				factor, factorErr := analyze(target, result.MinimumPhasePart)
				if factorErr != nil {
					t.Fatalf("delay %d: analyze factor: %v", delay, factorErr)
				}

				// The identity itself: the cascade's ripple is the minimum-phase
				// factor's, because the symmetric factor adds no phase deviation.
				if math.Abs(cascade.groupDelayRipple-factor.groupDelayRipple) > 1e-9 {
					t.Errorf(
						"delay %d: cascade ripple %.9f but its minimum-phase "+
							"factor's is %.9f; the symmetric factor is "+
							"contributing phase deviation",
						delay,
						cascade.groupDelayRipple,
						factor.groupDelayRipple,
					)
				}

				if index == 0 {
					referenceDelay = cascade.meanGroupDelay
					referenceRipple = factor.groupDelayRipple

					continue
				}

				// The budget shifts the whole group-delay curve by exactly its
				// own value — but only while the minimum-phase factor is itself
				// unchanged. Once the split starves that factor, truncating it
				// alters its own delay too, and the shift is no longer the
				// budget. An unchanged factor ripple is the test for that, so
				// this assertion applies exactly where it is meaningful.
				if math.Abs(factor.groupDelayRipple-referenceRipple) > 1e-9 {
					continue
				}

				shift := cascade.meanGroupDelay - referenceDelay
				if math.Abs(shift-float64(delay)) > 0.5 {
					t.Errorf(
						"delay %d shifted the mean group delay by %.3f samples "+
							"while leaving the minimum-phase factor unchanged, "+
							"want a shift of %d",
						delay,
						shift,
						delay,
					)
				}
			}
		})
	}
}

// prototypeNamed returns one committed target's prototype by name.
func prototypeNamed(t *testing.T, name string) []float64 {
	t.Helper()

	targets, err := Targets()
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}

	for _, target := range targets {
		if target.Name == name {
			return target.Prototype
		}
	}

	t.Fatalf("reference target %q not found", name)

	return nil
}
