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
