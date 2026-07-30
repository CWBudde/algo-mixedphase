package mixedphase

import (
	"errors"
	"math"
	"testing"
)

// continuumPrototype is a 257-tap Hamming-windowed sinc low-pass, the fixture
// the phase-continuum documentation quotes. It is designed into 129 taps
// throughout, so the projection has real work to do and the affine delay law is
// tested against a residual rather than against an identity.
func continuumPrototype() []float64 {
	return windowedSincPrototype(257, 0.12)
}

// windowedSincPrototype is a Hamming-windowed sinc low-pass at the given
// normalised cutoff.
func windowedSincPrototype(taps int, cutoff float64) []float64 {
	out := make([]float64, taps)
	centre := float64(taps-1) / 2

	for i := range out {
		offset := float64(i) - centre

		sinc := 2 * cutoff
		if offset != 0 {
			sinc = math.Sin(2*math.Pi*cutoff*offset) / (math.Pi * offset)
		}

		window := 0.54 - 0.46*math.Cos(2*math.Pi*float64(i)/float64(taps-1))
		out[i] = sinc * window
	}

	return out
}

// continuumFittingPrototype is a 65-tap low-pass of the same family, short
// enough that its minimum-phase factor fits comfortably inside the 129-tap
// output. Where continuumPrototype is support-starved and therefore almost
// indifferent to phase, this one shows the accuracy structure of the continuum.
func continuumFittingPrototype() []float64 {
	return windowedSincPrototype(65, 0.12)
}

const (
	continuumTestLength = 129
	continuumTestGrid   = 2048
)

// continuumFloor measures the minimum-phase group delay of a prototype on the
// same band and grid DesignContinuum uses, so tests can talk about the reachable
// window without hard-coding it.
func continuumFloor(t *testing.T, prototype []float64) float64 {
	t.Helper()

	base, err := DesignPhaseInterpolation(prototype, PhaseInterpolationConfig{
		Length:  continuumTestLength,
		Mix:     0,
		FFTSize: continuumTestGrid,
	})
	if err != nil {
		t.Fatalf("design minimum phase: %v", err)
	}

	mean, _, err := continuumMeasure(t, prototype, base.Taps)
	if err != nil {
		t.Fatalf("measure minimum-phase group delay: %v", err)
	}

	return mean
}

// continuumMeasure applies the package's own group-delay measure to taps on the
// default weight band of a prototype.
func continuumMeasure(
	t *testing.T,
	prototype []float64,
	taps []float64,
) (float64, float64, error) {
	t.Helper()

	w, err := newFFTWorkspace(continuumTestGrid)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	spectrum, err := w.forwardReal(prototype)
	if err != nil {
		t.Fatalf("transform prototype: %v", err)
	}

	targetMagnitude := magnitude(spectrum)

	weight, err := delayWeights(nil, targetMagnitude[:continuumTestGrid/2+1])
	if err != nil {
		t.Fatalf("build delay weights: %v", err)
	}

	peak := 0.0
	for _, value := range targetMagnitude[:continuumTestGrid/2+1] {
		peak = max(peak, value)
	}

	return measureGroupDelay(w, taps, weight, continuumDelayFloor*peak*peak)
}

// designAt is DesignContinuum on the shared fixture at one requested delay, with
// an optimiser budget small enough for a unit test.
func designAt(t *testing.T, prototype []float64, delay float64) Result {
	t.Helper()

	out, err := DesignContinuum(prototype, ContinuumConfig{
		Length:           continuumTestLength,
		TargetGroupDelay: delay,
		FFTSize:          continuumTestGrid,
		Iterations:       40,
		PenaltyStages:    3,
	})
	if err != nil {
		t.Fatalf("design at %g samples: %v", delay, err)
	}

	return out
}

// TestContinuumDelayLawMatchesTheAffinePrediction pins the identity the knob is
// built on: because group delay is linear in phase and the prescription blends
// two phases linearly, the realised mean delay is the same blend of the endpoint
// delays. Only the least-squares projection onto a finite support can break it,
// so the residual is what this measures.
func TestContinuumDelayLawMatchesTheAffinePrediction(t *testing.T) {
	prototype := continuumPrototype()
	minimum := continuumFloor(t, prototype)
	centre := float64(continuumTestLength-1) / 2

	// A tenth of a sample over a window 116 samples wide is a residual of under
	// a tenth of a percent. It is a projection error, not a modelling error.
	const tolerance = 0.1

	for _, mix := range []float64{0, 0.25, 0.5, 1, 1.5, 1.75, 2} {
		designed, err := DesignPhaseInterpolation(
			prototype,
			PhaseInterpolationConfig{
				Length:  continuumTestLength,
				Mix:     mix,
				FFTSize: continuumTestGrid,
			},
		)
		if err != nil {
			t.Fatalf("design at mix %g: %v", mix, err)
		}

		measured, _, err := continuumMeasure(t, prototype, designed.Taps)
		if err != nil {
			t.Fatalf("measure at mix %g: %v", mix, err)
		}

		predicted := (1-mix)*minimum + mix*centre

		if math.Abs(measured-predicted) > tolerance {
			t.Errorf(
				"mix %g: measured mean group delay %.4f, affine law predicts "+
					"%.4f (residual %.4f > %.4f). The knob inverts this law in "+
					"closed form, so a residual this large would make a "+
					"requested delay unreachable.",
				mix,
				measured,
				predicted,
				math.Abs(measured-predicted),
				tolerance,
			)
		}
	}
}

// TestContinuumKnobHitsItsRequestedDelay is the property the whole entry point
// exists for: inside the reachable window, asking for a delay in samples gets
// that delay in samples.
func TestContinuumKnobHitsItsRequestedDelay(t *testing.T) {
	prototype := continuumPrototype()
	minimum := continuumFloor(t, prototype)
	maximum := float64(continuumTestLength-1) - minimum

	const tolerance = 0.1

	for _, fraction := range []float64{0, 0.1, 0.25, 0.5, 0.75, 0.9, 1} {
		requested := minimum + fraction*(maximum-minimum)
		designed := designAt(t, prototype, requested)

		if designed.Regime != RegimeWindow {
			t.Errorf(
				"requested %.3f samples: regime is %v, want %v — the request "+
					"lies inside [%.3f, %.3f]",
				requested,
				designed.Regime,
				RegimeWindow,
				minimum,
				maximum,
			)
		}

		if math.Abs(designed.AchievedGroupDelay-requested) > tolerance {
			t.Errorf(
				"requested %.3f samples, achieved %.3f (miss %.4f > %.4f)",
				requested,
				designed.AchievedGroupDelay,
				math.Abs(designed.AchievedGroupDelay-requested),
				tolerance,
			)
		}
	}
}

// TestContinuumReachableWindowMatchesTheFloor pins the dispatch. The window is
// a property of the requested magnitude, not of the tap count alone, and a
// request outside it must be routed to the branch that concedes magnitude rather
// than silently clamped.
func TestContinuumReachableWindowMatchesTheFloor(t *testing.T) {
	prototype := continuumPrototype()
	minimum := continuumFloor(t, prototype)
	span := float64(continuumTestLength - 1)
	maximum := span - minimum

	cases := []struct {
		name      string
		requested float64
		want      ContinuumRegime
	}{
		{"below the floor", minimum / 2, RegimeSubMinimum},
		{"at the floor", minimum, RegimeWindow},
		{"linear phase", span / 2, RegimeWindow},
		{"at maximum phase", maximum, RegimeWindow},
		{"beyond maximum phase", maximum + (span-maximum)/2, RegimeSuperMaximum},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			designed := designAt(t, prototype, testCase.requested)

			if designed.Regime != testCase.want {
				t.Errorf(
					"requested %.3f samples: regime %v, want %v (window is "+
						"[%.3f, %.3f])",
					testCase.requested,
					designed.Regime,
					testCase.want,
					minimum,
					maximum,
				)
			}
		})
	}
}

// TestContinuumEndpointsAreTimeReversals pins the reflection symmetry that lets
// one solver cover both tails of the continuum: reversing a real filter negates
// its phase, so the design at the far edge of the window is the design at the
// near edge read backwards.
func TestContinuumEndpointsAreTimeReversals(t *testing.T) {
	prototype := continuumPrototype()
	minimum := continuumFloor(t, prototype)
	span := float64(continuumTestLength - 1)

	fast := designAt(t, prototype, minimum)
	slow := designAt(t, prototype, span-minimum)

	if len(fast.Taps) != len(slow.Taps) {
		t.Fatalf(
			"tap counts differ: %d and %d",
			len(fast.Taps),
			len(slow.Taps),
		)
	}

	peak := 0.0
	for _, tap := range fast.Taps {
		peak = max(peak, math.Abs(tap))
	}

	worst := 0.0
	for i, tap := range fast.Taps {
		worst = max(worst, math.Abs(tap-slow.Taps[len(slow.Taps)-1-i]))
	}

	// The two designs are separate least-squares projections, so this is a
	// numerical identity rather than an approximation.
	tolerance := peak * 1e-9

	if worst > tolerance {
		t.Errorf(
			"maximum-phase design deviates from the reversed minimum-phase "+
				"design by %g against a peak tap of %g (tolerance %g)",
			worst,
			peak,
			tolerance,
		)
	}
}

// TestContinuumCentreIsLinearPhase pins the middle of the continuum. A request
// of exactly (Length-1)/2 must produce a symmetric filter, which is the only
// point where group-delay ripple vanishes.
func TestContinuumCentreIsLinearPhase(t *testing.T) {
	prototype := continuumPrototype()
	centre := float64(continuumTestLength-1) / 2

	designed := designAt(t, prototype, centre)

	peak := 0.0
	for _, tap := range designed.Taps {
		peak = max(peak, math.Abs(tap))
	}

	worst := 0.0
	for i, tap := range designed.Taps {
		worst = max(worst, math.Abs(tap-designed.Taps[len(designed.Taps)-1-i]))
	}

	tolerance := peak * 1e-9

	if worst > tolerance {
		t.Errorf(
			"design at the centre of the window is asymmetric by %g against a "+
				"peak tap of %g (tolerance %g): the centre must be linear phase",
			worst,
			peak,
			tolerance,
		)
	}
}

// TestContinuumIsMostAccurateAtThePhasePureEndpoints pins the result that makes
// the knob a genuine trade rather than a free choice.
//
// The endpoints of the continuum are not merely the fastest and the slowest
// realisations of a magnitude, they are also the most accurate ones: a spectral
// factor of the target needs no compromise, while every intermediate phase has
// to be approximated on the same support. The error therefore rises steeply as
// soon as the knob leaves either end, which is the opposite of the monotone
// latency-versus-accuracy curve a latency budget is usually assumed to buy.
func TestContinuumIsMostAccurateAtThePhasePureEndpoints(t *testing.T) {
	prototype := continuumFittingPrototype()
	minimum := continuumFloor(t, prototype)
	span := float64(continuumTestLength - 1)

	errorAt := func(delay float64) float64 {
		return designAt(t, prototype, delay).Metrics.RelativeMagnitudeError
	}

	atMinimum := errorAt(minimum)
	atMaximum := errorAt(span - minimum)

	// The interior is sampled rather than optimised over, so this is a lower
	// bound on the true peak.
	worstInterior := 0.0
	for _, fraction := range []float64{0.03, 0.0625, 0.125, 0.25, 0.5, 0.75} {
		worstInterior = max(
			worstInterior,
			errorAt(minimum+fraction*(span-2*minimum)),
		)
	}

	const leastPenalty = 5

	if worstInterior < leastPenalty*atMinimum {
		t.Errorf(
			"worst sampled interior relative magnitude error %g is less than "+
				"%d times the minimum-phase value %g; leaving a spectral "+
				"factor is meant to cost magnitude accuracy",
			worstInterior,
			leastPenalty,
			atMinimum,
		)
	}

	// Reversal leaves the magnitude untouched, so the two ends must agree.
	if relativeGap(atMinimum, atMaximum) > 1e-6 {
		t.Errorf(
			"minimum-phase error %g and maximum-phase error %g differ; time "+
				"reversal cannot change a magnitude response",
			atMinimum,
			atMaximum,
		)
	}
}

// TestContinuumErrorIsSymmetricAboutLinearPhase pins the consequence of the
// reflection symmetry for the accuracy curve, and with it the reason linear phase
// is always a stationary point of that curve.
//
// Which kind of stationary point it is depends on the target: it is a local
// minimum of magnitude error for some and a local maximum for others, so linear
// phase cannot be recommended or dismissed on accuracy grounds in general. What
// is invariant is the symmetry.
func TestContinuumErrorIsSymmetricAboutLinearPhase(t *testing.T) {
	prototype := continuumFittingPrototype()
	minimum := continuumFloor(t, prototype)
	span := float64(continuumTestLength - 1)

	errorAt := func(delay float64) float64 {
		return designAt(t, prototype, delay).Metrics.RelativeMagnitudeError
	}

	for _, fraction := range []float64{0.1, 0.25, 0.4} {
		offset := fraction * (span/2 - minimum)

		fast := errorAt(span/2 - offset)
		slow := errorAt(span/2 + offset)

		if relativeGap(fast, slow) > 1e-6 {
			t.Errorf(
				"delays %.3f and %.3f are reflections about linear phase but "+
					"give relative magnitude errors %g and %g",
				span/2-offset,
				span/2+offset,
				fast,
				slow,
			)
		}
	}
}

// TestSupportStarvedTargetFlattensTheContinuum pins the failure mode of the
// knob, and the reason the two results above need a target that fits.
//
// When the requested magnitude is not realisable in the available taps,
// truncation error dominates every phase choice and the accuracy curve goes
// flat: the knob still moves the group delay, but it no longer buys or costs
// anything measurable in magnitude. That is the same condition under which the
// reachable window collapses, so a caller who finds the window narrow should not
// expect the accuracy structure above to be there either.
func TestSupportStarvedTargetFlattensTheContinuum(t *testing.T) {
	prototype := continuumPrototype()
	minimum := continuumFloor(t, prototype)
	span := float64(continuumTestLength - 1)

	lowest, highest := math.Inf(1), 0.0

	for _, fraction := range []float64{0, 0.0625, 0.25, 0.5, 0.75, 1} {
		relative := designAt(
			t,
			prototype,
			minimum+fraction*(span-2*minimum),
		).Metrics.RelativeMagnitudeError

		lowest = min(lowest, relative)
		highest = max(highest, relative)
	}

	// Two-to-one would still be a structure worth choosing within; the starved
	// case is far below that.
	const flat = 2.0

	if highest/lowest > flat {
		t.Errorf(
			"relative magnitude error spans %g to %g across the continuum, a "+
				"ratio of %.2f: a support-starved target is expected to be "+
				"almost indifferent to phase",
			lowest,
			highest,
			highest/lowest,
		)
	}
}

// relativeGap is the difference of two positive quantities relative to the
// larger of them.
func relativeGap(a, b float64) float64 {
	scale := max(math.Abs(a), math.Abs(b))
	if scale == 0 {
		return 0
	}

	return math.Abs(a-b) / scale
}

// TestSubFloorSolveBuysDelayBelowTheFloor pins regime (a). No phase choice
// reaches below the minimum-phase delay, so the only way down is to concede
// magnitude — and the swapped objective must actually make that trade rather
// than return the minimum-phase filter unchanged.
func TestSubFloorSolveBuysDelayBelowTheFloor(t *testing.T) {
	prototype := continuumPrototype()
	minimum := continuumFloor(t, prototype)

	requested := minimum / 2
	designed := designAt(t, prototype, requested)

	if designed.Regime != RegimeSubMinimum {
		t.Fatalf(
			"regime is %v, want %v for a request below the %.3f-sample floor",
			designed.Regime,
			RegimeSubMinimum,
			minimum,
		)
	}

	if designed.AchievedGroupDelay >= minimum {
		t.Errorf(
			"achieved %.4f samples, which is not below the floor of %.4f: the "+
				"solve bought no delay at all",
			designed.AchievedGroupDelay,
			minimum,
		)
	}

	// The concession is real and must be visible, but a design that threw the
	// magnitude away to reach the delay would be useless.
	if designed.Metrics.RelativeMagnitudeError > 0.5 {
		t.Errorf(
			"relative magnitude error %g at %.4f samples: the sub-floor solve "+
				"is meant to keep the error low while conceding delay",
			designed.Metrics.RelativeMagnitudeError,
			designed.AchievedGroupDelay,
		)
	}
}

// TestSuperMaximumSolveBuysDelayBeyondMaximumPhase is the mirror of the
// sub-floor case, and guards the reversal that implements it.
func TestSuperMaximumSolveBuysDelayBeyondMaximumPhase(t *testing.T) {
	prototype := continuumPrototype()
	minimum := continuumFloor(t, prototype)
	span := float64(continuumTestLength - 1)
	maximum := span - minimum

	requested := maximum + (span-maximum)/2
	designed := designAt(t, prototype, requested)

	if designed.Regime != RegimeSuperMaximum {
		t.Fatalf(
			"regime is %v, want %v",
			designed.Regime,
			RegimeSuperMaximum,
		)
	}

	if designed.AchievedGroupDelay <= maximum {
		t.Errorf(
			"achieved %.4f samples, which is not beyond maximum phase at "+
				"%.4f: the reflected solve bought no delay",
			designed.AchievedGroupDelay,
			maximum,
		)
	}
}

// TestDesignLowGroupDelayIsUnchangedByTheNewProblemForm guards the additive
// rule. The matched form is reached only through DesignContinuum, so a problem
// built for DesignLowGroupDelay must still evaluate the Wu-Gao-Teo objective
// through the new dispatch.
func TestDesignLowGroupDelayIsUnchangedByTheNewProblemForm(t *testing.T) {
	prototype := continuumPrototype()

	problem, taps, err := newLowDelayProblem(prototype, LowGroupDelayConfig{
		Length:  continuumTestLength,
		FFTSize: continuumTestGrid,
	})
	if err != nil {
		t.Fatalf("build problem: %v", err)
	}

	if problem.matchDelay {
		t.Fatal(
			"a problem built from LowGroupDelayConfig has matchDelay set; the " +
				"swapped objective must be opt-in through DesignContinuum only",
		)
	}

	problem.penalty = 1

	viaDispatch := make([]float64, len(taps))
	direct := make([]float64, len(taps))

	dispatched := problem.objective(taps, viaDispatch)
	evaluated := problem.evaluate(taps, direct)

	if dispatched != evaluated {
		t.Errorf(
			"objective returned %g but evaluate returned %g",
			dispatched,
			evaluated,
		)
	}

	for i := range direct {
		if viaDispatch[i] != direct[i] {
			t.Fatalf(
				"gradient differs at tap %d: %g through the dispatch, %g direct",
				i,
				viaDispatch[i],
				direct[i],
			)
		}
	}
}

// TestContinuumRejectsUnreachableRequests covers the validation surface.
func TestContinuumRejectsUnreachableRequests(t *testing.T) {
	prototype := continuumPrototype()

	cases := []struct {
		name string
		cfg  ContinuumConfig
		want error
	}{
		{
			name: "negative delay",
			cfg:  ContinuumConfig{Length: 129, TargetGroupDelay: -1},
			want: ErrDelayOutOfReach,
		},
		{
			name: "delay beyond the support",
			cfg:  ContinuumConfig{Length: 129, TargetGroupDelay: 129},
			want: ErrDelayOutOfReach,
		},
		{
			name: "not a number",
			cfg:  ContinuumConfig{Length: 129, TargetGroupDelay: math.NaN()},
			want: ErrNonFiniteConfig,
		},
		{
			name: "negative epsilon",
			cfg:  ContinuumConfig{Length: 129, Epsilon: -1},
			want: ErrInvalidEpsilon,
		},
		{
			name: "unknown method",
			cfg:  ContinuumConfig{Length: 129, Method: MinimumPhaseMethod(7)},
			want: ErrInvalidMethod,
		},
		{
			name: "negative iterations",
			cfg:  ContinuumConfig{Length: 129, Iterations: -1},
			want: ErrInvalidIterations,
		},
		{
			name: "negative length",
			cfg:  ContinuumConfig{Length: -1},
			want: ErrInvalidLength,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := DesignContinuum(prototype, testCase.cfg)
			if !errors.Is(err, testCase.want) {
				t.Errorf("got %v, want %v", err, testCase.want)
			}
		})
	}
}

// TestContinuumRejectsAnEmptyPrototype keeps the entry point consistent with the
// rest of the package.
func TestContinuumRejectsAnEmptyPrototype(t *testing.T) {
	if _, err := DesignContinuum(nil, ContinuumConfig{}); !errors.Is(
		err,
		ErrEmptyPrototype,
	) {
		t.Errorf("got %v, want %v", err, ErrEmptyPrototype)
	}
}

// TestContinuumRegimeNames pins the strings the comparison artifacts record.
func TestContinuumRegimeNames(t *testing.T) {
	cases := map[ContinuumRegime]string{
		RegimeUnspecified:  "unspecified",
		RegimeSubMinimum:   "sub-minimum",
		RegimeWindow:       "window",
		RegimeSuperMaximum: "super-maximum",
		ContinuumRegime(9): "unknown",
	}

	for regime, want := range cases {
		if got := regime.String(); got != want {
			t.Errorf("regime %d is %q, want %q", int(regime), got, want)
		}
	}
}
