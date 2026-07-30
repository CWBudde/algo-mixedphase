package mixedphase

import (
	"fmt"
	"math"
)

const (
	// continuumWindowSlack absorbs rounding when a request is compared against
	// the window edges, so that asking for exactly the measured floor takes the
	// prescribed-phase branch rather than the optimiser.
	continuumWindowSlack = 1e-9

	// continuumPeakWeightShare selects which bins count towards the reported
	// peak group delay, matching the rule [lowDelayProblem.summarise] uses.
	continuumPeakWeightShare = 1e-2

	// continuumDelayFloor bounds the 1/|H|^2 factor of the group-delay measure,
	// relative to the squared target peak, and excludes bins below it from the
	// weighted average entirely. A spectral null has no meaningful group delay,
	// and averaging one in would corrupt the very quantity the knob sets.
	continuumDelayFloor = 1e-12
)

// DesignContinuum designs a fixed-support FIR for a requested group delay,
// which is the method's only knob.
//
// The other entry points in this package each expose a different parameter —
// a mix, a latency budget, a magnitude tolerance — and leave the caller to work
// out which one moves group delay and by how much. This one takes the quantity
// a latency-constrained caller actually has, a number of samples, and selects
// the mechanism that serves it.
//
// # The reachable window
//
// A requested magnitude implies a minimum-phase group delay tau_min, and no
// causal realisation of that magnitude is faster (see [DesignLowGroupDelay] for
// the all-pass argument). Prescribing a phase between minimum and maximum
// reaches every delay in
//
//	[tau_min, Length-1-tau_min]
//
// and nothing outside it. The window is centred on (Length-1)/2 and its width is
// Length-1-2*tau_min, so it narrows as the target's own floor rises: at 129 taps
// the six reference targets give windows from 0.5 … 127.5 samples for a room
// correction, whose floor is half a sample, down to 49.4 … 78.6 for an LR8
// crossover, whose floor is 49.37. A magnitude that is hard to realise in the
// available taps dictates its own latency, and there is little left to choose.
//
// # The affine delay law
//
// Inside the window the request is met by prescribing
//
//	phi(mix) = (1-mix)*phi_min + mix*(-omega*(Length-1)/2)
//
// Group delay is linear in phase, so the prescription's weighted mean delay is
// the same blend of the endpoint delays,
//
//	tau(mix) = (1-mix)*tau_min + mix*(Length-1)/2
//
// which inverts in closed form. The mix that realises a requested tau is
//
//	mix(tau) = (tau - tau_min) / ((Length-1)/2 - tau_min)
//
// so the whole in-window regime costs one [DesignPhaseInterpolation] call and no
// search. The law holds for the prescription exactly and for the realised filter
// up to the least-squares projection onto Length taps; measured across the six
// reference targets that residual is under a tenth of a sample, and it is
// committed as the difference between the predicted and achieved columns of
// docs/reference-continuum.csv.
//
// # What each regime spends
//
// Inside the window only latency is spent, and mix one — a delay of exactly
// (Length-1)/2 — is the linear-phase filter whose group-delay ripple is zero.
// Outside the window phase choice is exhausted and the magnitude gives way
// instead: [prepareMatchDelay] minimises magnitude error subject to the
// requested delay. Requests beyond maximum phase are served by solving the
// reflected request and reversing the result, which is exact because reversing a
// real filter maps a delay of tau to Length-1-tau.
//
// Two properties of the knob are worth knowing before turning it. Group-delay
// ripple falls monotonically to zero at the centre of the window and rises again
// by reflection, but magnitude error does not follow it. The endpoints of the
// continuum are the most accurate points as well as the fastest and the slowest,
// because a spectral factor of the target needs no compromise while every
// intermediate phase has to be approximated on the same support: the error rises
// steeply as soon as the knob leaves either end, by three orders of magnitude on
// the reference low-pass. Being a reflection of itself, the error curve is exactly
// symmetric about the centre, so linear phase is always one of its stationary
// points — but a local minimum for some targets and a local maximum for others,
// which is why it cannot be recommended on accuracy grounds in general.
//
// Both properties disappear when the requested magnitude does not fit the
// available taps. Truncation error then dominates every phase choice and the
// accuracy curve goes flat, which is the same condition that collapses the
// window, so a narrow window is a warning that there is nothing to choose
// between phases either.
//
// The realised delay is reported as [Result.AchievedGroupDelay] and the branch
// taken as [Result.Regime]. Outside the window the optimiser is a penalty ladder
// around a local minimiser, so its iteration budget is a second
// delay-versus-accuracy dial rather than a convergence threshold, and results
// should be quoted with the budget that produced them.
func DesignContinuum(prototype []float64, cfg ContinuumConfig) (Result, error) {
	length, fftSize, err := validateContinuum(prototype, cfg)
	if err != nil {
		return Result{}, err
	}

	w, err := newFFTWorkspace(fftSize)
	if err != nil {
		return Result{}, err
	}

	targetSpectrum, err := w.forwardReal(prototype)
	if err != nil {
		return Result{}, err
	}

	targetMagnitude := magnitude(targetSpectrum)

	weight, err := delayWeights(cfg.DelayWeight, targetMagnitude[:fftSize/2+1])
	if err != nil {
		return Result{}, err
	}

	peak := 0.0
	for _, value := range targetMagnitude[:fftSize/2+1] {
		peak = max(peak, value)
	}

	if peak == 0 {
		return Result{}, fmt.Errorf("%w: reference magnitude", ErrZeroResponse)
	}

	floor := continuumDelayFloor * peak * peak

	// The floor is measured on the realised mix-zero design rather than on the
	// prescription, so that a request of exactly tau_min is met exactly and the
	// residual of the affine law is attributable to the projection alone.
	base, err := DesignPhaseInterpolation(prototype, PhaseInterpolationConfig{
		Length:  length,
		Mix:     0,
		FFTSize: fftSize,
		Epsilon: cfg.Epsilon,
		Method:  cfg.Method,
	})
	if err != nil {
		return Result{}, err
	}

	minimum, _, err := measureGroupDelay(w, base.Taps, weight, floor)
	if err != nil {
		return Result{}, err
	}

	return dispatchContinuum(continuumRequest{
		prototype: prototype,
		cfg:       cfg,
		length:    length,
		fftSize:   fftSize,
		workspace: w,
		weight:    weight,
		floor:     floor,
		base:      base,
		minimum:   minimum,
	})
}

// continuumRequest carries the resolved inputs of one [DesignContinuum] call
// through the branch selection.
type continuumRequest struct {
	prototype []float64
	cfg       ContinuumConfig
	length    int
	fftSize   int
	workspace *fftWorkspace
	weight    []float64
	floor     float64
	base      Result
	minimum   float64
}

// dispatchContinuum selects the regime that serves the request and finishes the
// result.
func dispatchContinuum(r continuumRequest) (Result, error) {
	span := float64(r.length - 1)
	centre := span / 2
	maximum := span - r.minimum
	requested := r.cfg.TargetGroupDelay

	// A floor at or beyond the centre leaves no window to interpolate across:
	// the target is already as slow as its taps allow, and every request is
	// served by conceding magnitude.
	hasWindow := centre-r.minimum > continuumWindowSlack

	switch {
	case hasWindow && requested >= r.minimum-continuumWindowSlack &&
		requested <= maximum+continuumWindowSlack:
		return finishWindow(r, centre)

	case requested < r.minimum:
		return finishMatched(r, requested, RegimeSubMinimum)

	default:
		return finishMatched(r, span-requested, RegimeSuperMaximum)
	}
}

// finishWindow serves an in-window request by inverting the affine delay law.
func finishWindow(r continuumRequest, centre float64) (Result, error) {
	mix := (r.cfg.TargetGroupDelay - r.minimum) / (centre - r.minimum)
	mix = min(max(mix, 0), maximumPhaseMix)

	out := r.base

	if mix != 0 {
		designed, err := DesignPhaseInterpolation(
			r.prototype,
			PhaseInterpolationConfig{
				Length:  r.length,
				Mix:     mix,
				FFTSize: r.fftSize,
				Epsilon: r.cfg.Epsilon,
				Method:  r.cfg.Method,
			},
		)
		if err != nil {
			return Result{}, err
		}

		out = designed
	}

	out.Regime = RegimeWindow

	return summariseContinuum(r, out)
}

// finishMatched serves an out-of-window request with the swapped problem form,
// reversing the result for the maximum-phase side.
//
// The reflected request is solved rather than the original one because reversing
// a real filter maps a group delay of tau to Length-1-tau exactly, so one solver
// covers both tails.
func finishMatched(
	r continuumRequest,
	requested float64,
	regime ContinuumRegime,
) (Result, error) {
	lowCfg := LowGroupDelayConfig{
		Length:        r.length,
		FFTSize:       r.fftSize,
		Epsilon:       r.cfg.Epsilon,
		Method:        r.cfg.Method,
		DelayWeight:   r.cfg.DelayWeight,
		Iterations:    r.cfg.Iterations,
		PenaltyStages: r.cfg.PenaltyStages,
	}

	problem, taps, err := newLowDelayProblem(r.prototype, lowCfg)
	if err != nil {
		return Result{}, err
	}

	problem.prepareMatchDelay(requested)

	iterations := runPenaltyLadder(problem, taps, lowCfg)

	if regime == RegimeSuperMaximum {
		reverseTaps(taps)
	}

	metrics, err := Analyze(r.prototype, taps, r.fftSize)
	if err != nil {
		return Result{}, err
	}

	return summariseContinuum(r, Result{
		Taps:       taps,
		Iterations: iterations,
		Metrics:    metrics,
		Regime:     regime,
	})
}

// summariseContinuum measures the realised group delay and records it.
func summariseContinuum(r continuumRequest, out Result) (Result, error) {
	mean, peak, err := measureGroupDelay(
		r.workspace,
		out.Taps,
		r.weight,
		r.floor,
	)
	if err != nil {
		return Result{}, err
	}

	out.AchievedGroupDelay = mean
	out.GroupDelay = GroupDelayMetrics{Mean: mean, Peak: peak}

	return out, nil
}

// reverseTaps reverses a filter in place, which negates its phase and turns a
// group delay of tau into len(taps)-1-tau.
func reverseTaps(taps []float64) {
	for i, j := 0, len(taps)-1; i < j; i, j = i+1, j-1 {
		taps[i], taps[j] = taps[j], taps[i]
	}
}

// measureGroupDelay returns the weighted mean and the peak group delay of taps.
//
// Per bin the group delay is (A*C + B*D)/|H|^2 with H = A + jB the response and
// C + jD the transform of n*h[n], so two real transforms replace the per-bin
// sums over taps that the optimiser needs for its gradient. Bins whose response
// falls below the floor are dropped rather than floored: unlike the optimiser,
// which needs a defined derivative everywhere, an average only needs to exclude
// the bins where the quantity does not exist.
func measureGroupDelay(
	w *fftWorkspace,
	taps []float64,
	weight []float64,
	floor float64,
) (float64, float64, error) {
	response, err := w.forwardReal(taps)
	if err != nil {
		return 0, 0, err
	}

	weighted := make([]float64, len(taps))
	for n, tap := range taps {
		weighted[n] = float64(n) * tap
	}

	slope, err := w.forwardReal(weighted)
	if err != nil {
		return 0, 0, err
	}

	largest := 0.0
	for _, value := range weight {
		largest = max(largest, value)
	}

	threshold := largest * continuumPeakWeightShare

	total, mean := 0.0, 0.0
	peak := math.Inf(-1)

	for bin, share := range weight {
		if share == 0 {
			continue
		}

		squared := real(response[bin])*real(response[bin]) +
			imag(response[bin])*imag(response[bin])
		if squared < floor {
			continue
		}

		delay := (real(response[bin])*real(slope[bin]) +
			imag(response[bin])*imag(slope[bin])) / squared

		total += share
		mean += share * delay

		if share >= threshold {
			peak = max(peak, delay)
		}
	}

	if total == 0 {
		return 0, 0, fmt.Errorf(
			"%w: group-delay band has no usable response bins",
			ErrZeroResponse,
		)
	}

	if math.IsInf(peak, -1) {
		peak = 0
	}

	return mean / total, peak, nil
}

// validateContinuum checks a [ContinuumConfig] and resolves the output length
// and design grid.
func validateContinuum(
	prototype []float64,
	cfg ContinuumConfig,
) (int, int, error) {
	if err := validatePrototype(prototype); err != nil {
		return 0, 0, err
	}

	if err := validateFiniteFields(
		field{"epsilon", cfg.Epsilon},
		field{"target group delay", cfg.TargetGroupDelay},
	); err != nil {
		return 0, 0, err
	}

	if cfg.Epsilon < 0 {
		return 0, 0, fmt.Errorf("%w: got %g", ErrInvalidEpsilon, cfg.Epsilon)
	}

	if !cfg.Method.valid() {
		return 0, 0, fmt.Errorf("%w: %d", ErrInvalidMethod, int(cfg.Method))
	}

	if cfg.Iterations < 0 || cfg.PenaltyStages < 0 {
		return 0, 0, fmt.Errorf(
			"%w: iterations %d, penalty stages %d",
			ErrInvalidIterations,
			cfg.Iterations,
			cfg.PenaltyStages,
		)
	}

	length := cfg.Length
	if length == 0 {
		length = len(prototype)
	}

	if length <= 0 {
		return 0, 0, ErrInvalidLength
	}

	if cfg.TargetGroupDelay < 0 ||
		cfg.TargetGroupDelay > float64(length-1) {
		return 0, 0, fmt.Errorf(
			"%w: got %g samples for %d taps",
			ErrDelayOutOfReach,
			cfg.TargetGroupDelay,
			length,
		)
	}

	fftSize, err := nextDesignFFTSize(max(length, len(prototype)), cfg.FFTSize)
	if err != nil {
		return 0, 0, err
	}

	return length, fftSize, nil
}
