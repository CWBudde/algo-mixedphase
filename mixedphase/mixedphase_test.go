package mixedphase

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/window"
)

// TestFactorWindowsDifferInSlope pins the only reason applyMinimumWindow and
// applyLinearWindow are separate functions: the minimum-phase factor is causal
// with its energy at the front, so it may only be tapered on the right, while
// the linear-phase residual is symmetric about its centre and is tapered on
// both edges. Tapering the head of the minimum-phase factor would attack
// exactly the samples that carry the response.
func TestFactorWindowsDifferInSlope(t *testing.T) {
	const length = 33

	ones := func() []float64 {
		out := make([]float64, length)
		for i := range out {
			out[i] = 1
		}

		return out
	}

	cfg := IterativeConfig{Window: window.TypeHann}

	minimumPart := ones()
	applyMinimumWindow(minimumPart, cfg)

	linearPart := ones()
	applyLinearWindow(linearPart, cfg)

	const unity = 1e-9

	if math.Abs(minimumPart[0]-1) > unity {
		t.Errorf(
			"minimum-phase factor head = %g, want an untouched 1: a right "+
				"slope must not taper the leading samples",
			minimumPart[0],
		)
	}

	if minimumPart[length-1] >= 1-unity {
		t.Errorf(
			"minimum-phase factor tail = %g, want a taper below 1",
			minimumPart[length-1],
		)
	}

	if linearPart[0] >= 1-unity || linearPart[length-1] >= 1-unity {
		t.Errorf(
			"linear-phase factor edges = %g/%g, want both tapered below 1",
			linearPart[0],
			linearPart[length-1],
		)
	}

	if math.Abs(linearPart[0]-linearPart[length-1]) > unity {
		t.Errorf(
			"linear-phase factor edges = %g/%g, want a symmetric taper",
			linearPart[0],
			linearPart[length-1],
		)
	}
}

// TestFactorWindowAlphaAndRectangular covers the two remaining branches of the
// factor windows: the rectangular early return, and the parametric path that
// forwards WindowAlpha as the window's alpha/beta parameter.
func TestFactorWindowAlphaAndRectangular(t *testing.T) {
	const length = 33

	ones := func() []float64 {
		out := make([]float64, length)
		for i := range out {
			out[i] = 1
		}

		return out
	}

	rectangular := ones()
	applyMinimumWindow(rectangular, IterativeConfig{})
	applyLinearWindow(rectangular, IterativeConfig{})

	for i, value := range rectangular {
		if value != 1 {
			t.Fatalf("rectangular window changed tap %d to %g", i, value)
		}
	}

	narrow := ones()
	applyLinearWindow(
		narrow,
		IterativeConfig{Window: window.TypeKaiser, WindowAlpha: 2},
	)

	wide := ones()
	applyLinearWindow(
		wide,
		IterativeConfig{Window: window.TypeKaiser, WindowAlpha: 12},
	)

	// A larger Kaiser beta is a narrower window, so its edges sit lower.
	if wide[0] >= narrow[0] {
		t.Errorf(
			"Kaiser beta 12 edge = %g, want below the beta 2 edge %g: "+
				"WindowAlpha is not reaching the window",
			wide[0],
			narrow[0],
		)
	}
}

func TestMinimumPhaseMovesEnergyForward(t *testing.T) {
	prototype := lowpassPrototype(129, 0.12)

	taps, err := MinimumPhase(prototype, 4096)
	if err != nil {
		t.Fatalf("MinimumPhase() error = %v", err)
	}

	linearMetrics, err := Analyze(prototype, prototype, 4096)
	if err != nil {
		t.Fatalf("Analyze(linear) error = %v", err)
	}

	minimumMetrics, err := Analyze(prototype, taps, 4096)
	if err != nil {
		t.Fatalf("Analyze(minimum) error = %v", err)
	}

	t.Logf("minimum-phase metrics: %+v", minimumMetrics)

	if minimumMetrics.EnergyCentroid >= linearMetrics.EnergyCentroid/2 {
		t.Fatalf(
			"minimum-phase centroid = %f, linear centroid = %f",
			minimumMetrics.EnergyCentroid,
			linearMetrics.EnergyCentroid,
		)
	}

	if minimumMetrics.RelativeMagnitudeError > 2e-4 {
		t.Fatalf(
			"minimum-phase relative magnitude error = %g, want <= 2e-4",
			minimumMetrics.RelativeMagnitudeError,
		)
	}
}

func TestDesignIterativeHonoursTapBudget(t *testing.T) {
	prototype := lowpassPrototype(129, 0.12)

	result, err := DesignIterative(prototype, IterativeConfig{
		Length:     129,
		Delay:      16,
		Iterations: 8,
		FFTSize:    4096,
	})
	if err != nil {
		t.Fatalf("DesignIterative() error = %v", err)
	}

	t.Logf("iterative metrics: %+v", result.Metrics)

	if len(result.Taps) != 129 {
		t.Fatalf("len(Taps) = %d, want 129", len(result.Taps))
	}

	if len(result.LinearPhasePart) != 33 {
		t.Fatalf(
			"len(LinearPhasePart) = %d, want 33",
			len(result.LinearPhasePart),
		)
	}

	if len(result.MinimumPhasePart) != 97 {
		t.Fatalf(
			"len(MinimumPhasePart) = %d, want 97",
			len(result.MinimumPhasePart),
		)
	}

	if result.Metrics.RelativeMagnitudeError > 0.01 {
		t.Fatalf(
			"relative magnitude error = %g, want <= 0.01",
			result.Metrics.RelativeMagnitudeError,
		)
	}

	if result.Metrics.PeakIndex >= 32 {
		t.Fatalf("PeakIndex = %d, want < 32", result.Metrics.PeakIndex)
	}
}

func TestIterativeZeroDelayIsMinimumPhaseEndpoint(t *testing.T) {
	prototype := lowpassPrototype(129, 0.12)

	want, err := MinimumPhase(prototype, 4096)
	if err != nil {
		t.Fatalf("MinimumPhase() error = %v", err)
	}

	result, err := DesignIterative(prototype, IterativeConfig{
		Length:  129,
		Delay:   0,
		FFTSize: 4096,
	})
	if err != nil {
		t.Fatalf("DesignIterative() error = %v", err)
	}

	if len(result.LinearPhasePart) != 1 ||
		result.LinearPhasePart[0] != 1 {
		t.Fatalf(
			"LinearPhasePart = %v, want identity factor",
			result.LinearPhasePart,
		)
	}

	for i := range want {
		if math.Abs(result.Taps[i]-want[i]) > 1e-10 {
			t.Fatalf(
				"Taps[%d] = %.16g, want %.16g",
				i,
				result.Taps[i],
				want[i],
			)
		}
	}
}

// TestIterativeMaximumDelayIsLinearPhaseEndpoint covers the opposite endpoint
// to TestIterativeZeroDelayIsMinimumPhaseEndpoint. At Delay = (Length-1)/2 the
// linear factor consumes the whole budget, the minimum-phase factor collapses
// to the identity, and no alternating correction is possible — the design must
// return a symmetric linear-phase FIR in one pass.
func TestIterativeMaximumDelayIsLinearPhaseEndpoint(t *testing.T) {
	const length = 129

	prototype := lowpassPrototype(length, 0.12)

	result, err := DesignIterative(prototype, IterativeConfig{
		Length:  length,
		Delay:   (length - 1) / 2,
		FFTSize: 4096,
	})
	if err != nil {
		t.Fatalf("DesignIterative() error = %v", err)
	}

	if len(result.MinimumPhasePart) != 1 ||
		result.MinimumPhasePart[0] != 1 {
		t.Fatalf(
			"MinimumPhasePart = %v, want identity factor",
			result.MinimumPhasePart,
		)
	}

	if len(result.Taps) != length {
		t.Fatalf("len(Taps) = %d, want %d", len(result.Taps), length)
	}

	if result.Iterations != 0 {
		t.Errorf(
			"Iterations = %d, want 0: the endpoint admits no correction",
			result.Iterations,
		)
	}

	for i := range length / 2 {
		mirrored := length - 1 - i
		if math.Abs(result.Taps[i]-result.Taps[mirrored]) > 1e-12 {
			t.Fatalf(
				"Taps[%d] = %.16g and Taps[%d] = %.16g are not symmetric",
				i,
				result.Taps[i],
				mirrored,
				result.Taps[mirrored],
			)
		}
	}
}

// TestAnalyzeValidation covers the public metric entry point's guards,
// including the zero-response case that leaves every relative error undefined.
func TestAnalyzeValidation(t *testing.T) {
	prototype := lowpassPrototype(33, 0.2)

	tests := []struct {
		name      string
		reference []float64
		candidate []float64
		fftSize   int
		want      error
	}{
		{
			name:      "empty reference",
			candidate: prototype,
			want:      ErrEmptyPrototype,
		},
		{
			name:      "empty candidate",
			reference: prototype,
			want:      ErrInvalidLength,
		},
		{
			name:      "grid shorter than the signals",
			reference: prototype,
			candidate: prototype,
			fftSize:   8,
			want:      ErrInvalidLength,
		},
		{
			name:      "silent reference",
			reference: make([]float64, len(prototype)),
			candidate: prototype,
			want:      ErrZeroResponse,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Analyze(test.reference, test.candidate, test.fftSize)
			if !errors.Is(err, test.want) {
				t.Errorf("Analyze() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestIterativeImprovesUncorrectedFactorisation(t *testing.T) {
	prototype := lowpassPrototype(129, 0.08)

	initial, err := DesignIterative(prototype, IterativeConfig{
		Length:      129,
		Delay:       12,
		Iterations:  -1,
		FFTSize:     4096,
		ToleranceDB: -1,
	})
	if err != nil {
		t.Fatalf("initial DesignIterative() error = %v", err)
	}

	corrected, err := DesignIterative(prototype, IterativeConfig{
		Length:      129,
		Delay:       12,
		Iterations:  8,
		FFTSize:     4096,
		ToleranceDB: -1,
	})
	if err != nil {
		t.Fatalf("corrected DesignIterative() error = %v", err)
	}

	t.Logf(
		"uncorrected/corrected RMS magnitude error: %f/%f dB",
		initial.Metrics.RMSMagnitudeErrorDB,
		corrected.Metrics.RMSMagnitudeErrorDB,
	)

	if corrected.Metrics.RMSMagnitudeErrorDB >=
		initial.Metrics.RMSMagnitudeErrorDB {
		t.Fatalf(
			"corrected error = %f dB, initial error = %f dB",
			corrected.Metrics.RMSMagnitudeErrorDB,
			initial.Metrics.RMSMagnitudeErrorDB,
		)
	}
}

func TestIterativeStopsBeforeRisingError(t *testing.T) {
	prototype := lowpassPrototype(129, 0.08)
	cfg := IterativeConfig{
		Length:     129,
		Delay:      8,
		Iterations: 12,
	}

	result, err := DesignIterative(prototype, cfg)
	if err != nil {
		t.Fatalf("DesignIterative() error = %v", err)
	}

	if result.Iterations != 2 {
		t.Fatalf("Iterations = %d, want 2 accepted passes", result.Iterations)
	}

	cfg.Iterations = 2
	cfg.ToleranceDB = -1

	twoPass, err := DesignIterative(prototype, cfg)
	if err != nil {
		t.Fatalf("two-pass DesignIterative() error = %v", err)
	}

	cfg.Iterations = 3

	threePass, err := DesignIterative(prototype, cfg)
	if err != nil {
		t.Fatalf("three-pass DesignIterative() error = %v", err)
	}

	if result.Metrics != twoPass.Metrics {
		t.Fatalf(
			"early-stopped metrics = %+v, two-pass metrics = %+v",
			result.Metrics,
			twoPass.Metrics,
		)
	}

	if threePass.Metrics.RMSMagnitudeErrorDB <=
		twoPass.Metrics.RMSMagnitudeErrorDB {
		t.Fatalf(
			"three-pass RMS error = %g dB, two-pass error = %g dB",
			threePass.Metrics.RMSMagnitudeErrorDB,
			twoPass.Metrics.RMSMagnitudeErrorDB,
		)
	}
}

func TestIterativeCrossBuildDeterminism(t *testing.T) {
	result, err := DesignIterative(
		lowpassPrototype(129, 0.08),
		IterativeConfig{
			Length:     129,
			Delay:      8,
			Iterations: 12,
		},
	)
	if err != nil {
		t.Fatalf("DesignIterative() error = %v", err)
	}

	if result.Iterations != 2 {
		t.Fatalf("Iterations = %d, want 2 accepted passes", result.Iterations)
	}

	wantMetrics := Metrics{
		RMSMagnitudeErrorDB:    4.609232122872466,
		MaxMagnitudeErrorDB:    21.610395552671665,
		RelativeMagnitudeError: 0.0054262072164596,
		PeakIndex:              21,
		EnergyCentroid:         23.536691702175123,
		PrePeakEnergyRatio:     0.3932616859589017,
	}

	assertClose := func(name string, got, want, tolerance float64) {
		t.Helper()

		if math.Abs(got-want) > tolerance {
			t.Errorf("%s = %.17g, want %.17g ± %g", name, got, want, tolerance)
		}
	}

	assertClose(
		"RMSMagnitudeErrorDB",
		result.Metrics.RMSMagnitudeErrorDB,
		wantMetrics.RMSMagnitudeErrorDB,
		2e-9,
	)
	assertClose(
		"MaxMagnitudeErrorDB",
		result.Metrics.MaxMagnitudeErrorDB,
		wantMetrics.MaxMagnitudeErrorDB,
		2e-9,
	)
	assertClose(
		"RelativeMagnitudeError",
		result.Metrics.RelativeMagnitudeError,
		wantMetrics.RelativeMagnitudeError,
		2e-9,
	)
	assertClose(
		"EnergyCentroid",
		result.Metrics.EnergyCentroid,
		wantMetrics.EnergyCentroid,
		2e-9,
	)
	assertClose(
		"PrePeakEnergyRatio",
		result.Metrics.PrePeakEnergyRatio,
		wantMetrics.PrePeakEnergyRatio,
		2e-9,
	)

	if result.Metrics.PeakIndex != wantMetrics.PeakIndex {
		t.Errorf(
			"PeakIndex = %d, want %d",
			result.Metrics.PeakIndex,
			wantMetrics.PeakIndex,
		)
	}

	wantTaps := map[int]float64{
		0:   -1.194549316658406e-6,
		8:   -0.0010995246975603714,
		21:  0.13952194757418454,
		64:  0.012394538199736068,
		128: -1.4609143543942293e-7,
	}
	for index, want := range wantTaps {
		assertClose(
			fmt.Sprintf("Taps[%d]", index),
			result.Taps[index],
			want,
			1e-10,
		)
	}
}

func TestIterativeConditioning(t *testing.T) {
	prototype := lowpassPrototype(129, 0.08)
	testCases := []struct {
		name       string
		iterations int
		epsilon    float64
		wantNative float64
		wantWASM   float64
		wantLinear float64
	}{
		{"uncorrected", -1, 0, 3.716940096, 3.716940096, 0.021793880},
		{"pass 1", 1, 0, 5.366154225, 5.366154225, 0},
		{"pass 2", 2, 0, 4.609232123, 4.609232122, 0.005426207},
		{"pass 3", 3, 0, 4.917231861, 4.917231836, 0},
		{"pass 4", 4, 0, 4.686326143, 4.686326150, 0},
		{"pass 5", 5, 0, 4.852711194, 4.852727335, 0},
		{"pass 6", 6, 0, 4.605097725, 4.596586007, 0},
		{"pass 12", 12, 0, 10.739657501, 15.829906398, 0},
		{"regularised pass 12", 12, 1e-6, 4.767565981, 4.855304690, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := DesignIterative(prototype, IterativeConfig{
				Length:      129,
				Delay:       8,
				Iterations:  tc.iterations,
				Epsilon:     tc.epsilon,
				ToleranceDB: -1,
			})
			if err != nil {
				t.Fatalf("DesignIterative() error = %v", err)
			}

			want := tc.wantNative
			if runtime.GOOS == "js" {
				want = tc.wantWASM
			}

			if math.Abs(result.Metrics.RMSMagnitudeErrorDB-want) > 1e-6 {
				t.Errorf(
					"RMSMagnitudeErrorDB = %.9f, want %.9f ± 1e-6",
					result.Metrics.RMSMagnitudeErrorDB,
					want,
				)
			}

			if tc.wantLinear > 0 &&
				math.Abs(
					result.Metrics.RelativeMagnitudeError-tc.wantLinear,
				) > 1e-6 {
				t.Errorf(
					"RelativeMagnitudeError = %.9f, want %.9f ± 1e-6",
					result.Metrics.RelativeMagnitudeError,
					tc.wantLinear,
				)
			}
		})
	}
}

func TestIterativeBeatsDirectPhaseProjectionAtSameDelay(t *testing.T) {
	prototype := lowpassPrototype(129, 0.08)
	delay := 16
	maxDelay := (len(prototype) - 1) / 2

	iterative, err := DesignIterative(prototype, IterativeConfig{
		Length:     len(prototype),
		Delay:      delay,
		Iterations: 8,
		FFTSize:    4096,
	})
	if err != nil {
		t.Fatalf("DesignIterative() error = %v", err)
	}

	direct, err := DesignPhaseInterpolation(
		prototype,
		PhaseInterpolationConfig{
			Length:  len(prototype),
			Mix:     float64(delay) / float64(maxDelay),
			FFTSize: 4096,
		},
	)
	if err != nil {
		t.Fatalf("DesignPhaseInterpolation() error = %v", err)
	}

	// The dB metrics weight the whole response, so they expose the stopband
	// damage that truncating an interpolated phase causes. The linear relative
	// error is dominated by the passband, where the direct projection keeps the
	// target magnitude by construction and therefore stays ahead.
	if iterative.Metrics.RMSMagnitudeErrorDB >=
		direct.Metrics.RMSMagnitudeErrorDB {
		t.Fatalf(
			"iterative RMS dB error = %g, direct RMS dB error = %g",
			iterative.Metrics.RMSMagnitudeErrorDB,
			direct.Metrics.RMSMagnitudeErrorDB,
		)
	}

	if iterative.Metrics.MaxMagnitudeErrorDB >=
		direct.Metrics.MaxMagnitudeErrorDB {
		t.Fatalf(
			"iterative max dB error = %g, direct max dB error = %g",
			iterative.Metrics.MaxMagnitudeErrorDB,
			direct.Metrics.MaxMagnitudeErrorDB,
		)
	}
}

func TestPhaseInterpolationMovesPeakContinuously(t *testing.T) {
	prototype := lowpassPrototype(129, 0.12)

	minimum, err := DesignPhaseInterpolation(
		prototype,
		PhaseInterpolationConfig{Length: 129, Mix: 0, FFTSize: 4096},
	)
	if err != nil {
		t.Fatalf("minimum DesignPhaseInterpolation() error = %v", err)
	}

	mixed, err := DesignPhaseInterpolation(
		prototype,
		PhaseInterpolationConfig{Length: 129, Mix: 0.5, FFTSize: 4096},
	)
	if err != nil {
		t.Fatalf("mixed DesignPhaseInterpolation() error = %v", err)
	}

	linear, err := DesignPhaseInterpolation(
		prototype,
		PhaseInterpolationConfig{Length: 129, Mix: 1, FFTSize: 4096},
	)
	if err != nil {
		t.Fatalf("linear DesignPhaseInterpolation() error = %v", err)
	}

	t.Logf(
		"interpolation metrics minimum/mixed/linear: %+v / %+v / %+v",
		minimum.Metrics,
		mixed.Metrics,
		linear.Metrics,
	)

	if minimum.Metrics.PeakIndex >= mixed.Metrics.PeakIndex ||
		mixed.Metrics.PeakIndex >= linear.Metrics.PeakIndex {
		t.Fatalf(
			"peak indices minimum/mixed/linear = %d/%d/%d",
			minimum.Metrics.PeakIndex,
			mixed.Metrics.PeakIndex,
			linear.Metrics.PeakIndex,
		)
	}
}

func TestDesignValidation(t *testing.T) {
	_, err := DesignIterative(nil, IterativeConfig{})
	if !errors.Is(err, ErrEmptyPrototype) {
		t.Fatalf("empty prototype error = %v", err)
	}

	_, err = DesignIterative(
		[]float64{1},
		IterativeConfig{Length: 5, Delay: 3},
	)
	if !errors.Is(err, ErrInvalidDelay) {
		t.Fatalf("invalid delay error = %v", err)
	}

	_, err = DesignIterative(
		[]float64{1},
		IterativeConfig{Epsilon: -1e-9},
	)
	if !errors.Is(err, ErrInvalidEpsilon) {
		t.Fatalf("negative iterative epsilon error = %v", err)
	}

	_, err = DesignIterative(
		[]float64{1},
		IterativeConfig{Window: window.TypeKaiser, WindowAlpha: -3},
	)
	if !errors.Is(err, ErrInvalidWindowAlpha) {
		t.Fatalf("negative window alpha error = %v", err)
	}

	_, err = DesignPhaseInterpolation(
		[]float64{1},
		PhaseInterpolationConfig{Mix: 1.1},
	)
	if !errors.Is(err, ErrInvalidPhaseMix) {
		t.Fatalf("invalid phase mix error = %v", err)
	}

	_, err = DesignPhaseInterpolation(
		[]float64{1},
		PhaseInterpolationConfig{Epsilon: -1e-9},
	)
	if !errors.Is(err, ErrInvalidEpsilon) {
		t.Fatalf("negative interpolation epsilon error = %v", err)
	}
}

// TestPhaseInterpolationHandlesPhaseWraps guards against unwrapping that
// compares an already corrected sample with a raw one: such a bug accumulates
// spurious 2*pi steps on prototypes whose minimum phase crosses +/-pi many
// times, and for fractional mixes those steps no longer cancel.
func TestPhaseInterpolationHandlesPhaseWraps(t *testing.T) {
	// A long, narrow lowpass accumulates several hundred radians of phase
	// across the design grid, so the wrapped phase crosses the branch cut
	// repeatedly.
	prototype := lowpassPrototype(257, 0.03)

	for _, mix := range []float64{0.25, 0.5, 0.75} {
		result, err := DesignPhaseInterpolation(
			prototype,
			PhaseInterpolationConfig{Length: 257, Mix: mix, FFTSize: 8192},
		)
		if err != nil {
			t.Fatalf("DesignPhaseInterpolation(mix=%g) error = %v", mix, err)
		}

		t.Logf("mix=%g metrics: %+v", mix, result.Metrics)

		if result.Metrics.RelativeMagnitudeError > 1e-2 {
			t.Fatalf(
				"mix=%g relative magnitude error = %g, want <= 1e-2",
				mix,
				result.Metrics.RelativeMagnitudeError,
			)
		}
	}
}

func lowpassPrototype(length int, cutoff float64) []float64 {
	taps := make([]float64, length)
	middle := float64(length-1) / 2
	sum := 0.0

	for i := range taps {
		x := float64(i) - middle

		sinc := 2 * cutoff
		if x != 0 {
			sinc = math.Sin(2*math.Pi*cutoff*x) / (math.Pi * x)
		}

		windowValue := 0.5 -
			0.5*math.Cos(2*math.Pi*float64(i)/float64(length-1))
		taps[i] = sinc * windowValue
		sum += taps[i]
	}

	for i := range taps {
		taps[i] /= sum
	}

	return taps
}
