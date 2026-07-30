package mixedphase

import (
	"fmt"
)

// DesignPhaseInterpolation interpolates between the unwrapped minimum-phase
// response and a pure-delay linear-phase response, then projects that complex
// response onto the requested finite causal support.
//
// This direct method is intentionally simple. It provides a useful baseline
// for measuring how much the alternating factorisation recovers from the
// truncation error inherent in phase interpolation.
//
// # The continuum and its reflection symmetry
//
// [PhaseInterpolationConfig.Mix] runs over the whole phase continuum for a
// fixed magnitude: zero is minimum phase, one is linear phase and two is
// maximum phase. The family is symmetric about linear phase,
//
//	Design(2-mix).Taps == reverse(Design(mix).Taps)
//
// exactly, because reversing a real response negates its phase and a
// compensating delay of Length-1 restores causality, which is the same
// prescription the mix 2-mix produces. TestPhaseContinuumReflectsAboutLinearPhase
// pins it on a 257-tap windowed-sinc low-pass designed into 129 taps on a
// 4096-point grid, where the worst deviation is 1.2e-15 against a peak tap of
// 0.371.
//
// Two consequences are worth relying on. Every magnitude measure is symmetric
// in mix about one, since time reversal leaves |H| unchanged: on that fixture
// mix 0 and mix 2 both give 43.598 dB RMS error, and 0.25 and 1.75 both give
// 41.021 dB. And the group delay reflects, mean(2-mix) = (Length-1) -
// mean(mix), so maximum phase is the most expensive point of the continuum in
// both latency and pre-ringing. The useful working range is therefore mix in
// [0, 1]; the upper half is included because the continuum is only complete
// with it, not because it is a good place to sit.
//
// Group-delay deviation is likewise symmetric and reaches zero only at mix one.
// This is the axis on which the method differs from the alternating
// factorisation of [DesignIterative], whose deviation is fixed by its
// minimum-phase factor and does not respond to its delay budget at all.
func DesignPhaseInterpolation(
	prototype []float64,
	cfg PhaseInterpolationConfig,
) (Result, error) {
	if err := validatePrototype(prototype); err != nil {
		return Result{}, err
	}

	if err := validateFinite("mix", cfg.Mix); err != nil {
		return Result{}, err
	}

	if cfg.Mix < 0 || cfg.Mix > maximumPhaseMix {
		return Result{}, ErrInvalidPhaseMix
	}

	if err := validateFinite("epsilon", cfg.Epsilon); err != nil {
		return Result{}, err
	}

	if cfg.Epsilon < 0 {
		return Result{}, fmt.Errorf("%w: got %g", ErrInvalidEpsilon, cfg.Epsilon)
	}

	if !cfg.Method.valid() {
		return Result{}, fmt.Errorf("%w: %d", ErrInvalidMethod, int(cfg.Method))
	}

	length := cfg.Length
	if length == 0 {
		length = len(prototype)
	}

	if length <= 0 {
		return Result{}, ErrInvalidLength
	}

	fftSize, err := nextDesignFFTSize(max(length, len(prototype)), cfg.FFTSize)
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
	epsilon := defaultEpsilon(targetMagnitude, cfg.Epsilon)

	_, minimumPhase, err := minimumPhaseSpectrum(
		w,
		targetMagnitude,
		epsilon,
		cfg.Method,
	)
	if err != nil {
		return Result{}, err
	}

	desired := prescribedResponse(
		w,
		targetMagnitude,
		minimumPhase,
		cfg.Mix,
		float64(length-1)/2,
	)

	impulse, err := w.inverseReal(desired)
	if err != nil {
		return Result{}, fmt.Errorf(
			"mixedphase: project interpolated phase: %w",
			err,
		)
	}

	taps := append([]float64(nil), impulse[:length]...)

	metrics, err := Analyze(prototype, taps, fftSize)
	if err != nil {
		return Result{}, err
	}

	return Result{Taps: taps, Metrics: metrics}, nil
}
