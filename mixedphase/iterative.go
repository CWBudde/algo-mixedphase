package mixedphase

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/conv"
	"github.com/cwbudde/algo-dsp/dsp/window"
)

const (
	defaultIterations = 12
	defaultTolerance  = 1e-7
)

// DesignIterative implements the alternating mixed-phase factorisation from
// “Gemischtphasige Filter”, DAGA 2012.
//
// The requested Delay determines the split under a fixed tap budget:
//
//	linear length  = 2*Delay + 1
//	minimum length = Length - linear length + 1
//
// Each pass redesigns both factors in turn: first the minimum-phase factor from
// the regularised spectral quotient of the target and the current linear-phase
// factor, then the linear-phase factor from the quotient of the target and the
// freshly designed minimum-phase factor. Each is truncated and windowed back to
// its own support before the next quotient is formed, so the alternation is
// what lets one factor absorb the other's windowing error.
//
// Two delays short-circuit the alternation because one factor degenerates: a
// Delay of zero reduces the design to a truncated minimum-phase filter, and the
// largest admissible Delay leaves the minimum-phase factor a single tap and
// reduces it to a linear-phase filter. Both return with Iterations zero.
func DesignIterative(prototype []float64, cfg IterativeConfig) (Result, error) {
	if err := validatePrototype(prototype); err != nil {
		return Result{}, err
	}

	length := cfg.Length
	if length == 0 {
		length = len(prototype)
	}

	if length <= 0 {
		return Result{}, ErrInvalidLength
	}

	if err := validateFiniteFields(
		field{"epsilon", cfg.Epsilon},
		field{"window alpha", cfg.WindowAlpha},
		field{"tolerance", cfg.ToleranceDB},
	); err != nil {
		return Result{}, err
	}

	if cfg.Epsilon < 0 {
		return Result{}, fmt.Errorf("%w: got %g", ErrInvalidEpsilon, cfg.Epsilon)
	}

	if cfg.WindowAlpha < 0 {
		return Result{}, fmt.Errorf(
			"%w: got %g",
			ErrInvalidWindowAlpha,
			cfg.WindowAlpha,
		)
	}

	if !cfg.Method.valid() {
		return Result{}, fmt.Errorf("%w: %d", ErrInvalidMethod, int(cfg.Method))
	}

	maxDelay := (length - 1) / 2
	if cfg.Delay < 0 || cfg.Delay > maxDelay {
		return Result{}, fmt.Errorf(
			"%w: got %d, allowed range is [0, %d]",
			ErrInvalidDelay,
			cfg.Delay,
			maxDelay,
		)
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

	linearLength := 2*cfg.Delay + 1

	minimumLength := length - linearLength + 1
	if minimumLength <= 0 {
		return Result{}, ErrInvalidLength
	}

	if cfg.Delay == 0 {
		minimumPart, designErr := designMinimumPart(
			w,
			targetMagnitude,
			minimumLength,
			epsilon,
			cfg,
		)
		if designErr != nil {
			return Result{}, designErr
		}

		metrics, analyzeErr := analyzeAgainstSpectrum(
			w,
			targetSpectrum,
			minimumPart,
		)
		if analyzeErr != nil {
			return Result{}, analyzeErr
		}

		return Result{
			Taps:             append([]float64(nil), minimumPart...),
			MinimumPhasePart: minimumPart,
			LinearPhasePart:  []float64{1},
			Metrics:          metrics,
		}, nil
	}

	if minimumLength == 1 {
		linearPart, designErr := designLinearPart(
			w,
			targetMagnitude,
			linearLength,
			cfg,
		)
		if designErr != nil {
			return Result{}, designErr
		}

		metrics, analyzeErr := analyzeAgainstSpectrum(
			w,
			targetSpectrum,
			linearPart,
		)
		if analyzeErr != nil {
			return Result{}, analyzeErr
		}

		return Result{
			Taps:             append([]float64(nil), linearPart...),
			MinimumPhasePart: []float64{1},
			LinearPhasePart:  linearPart,
			Metrics:          metrics,
		}, nil
	}

	minimumPart, err := designMinimumPart(
		w,
		targetMagnitude,
		minimumLength,
		epsilon,
		cfg,
	)
	if err != nil {
		return Result{}, err
	}

	minimumSpectrum, err := w.forwardReal(minimumPart)
	if err != nil {
		return Result{}, err
	}

	residualMagnitude := regularisedMagnitudeDivision(
		targetSpectrum,
		minimumSpectrum,
		epsilon,
	)

	linearPart, err := designLinearPart(w, residualMagnitude, linearLength, cfg)
	if err != nil {
		return Result{}, err
	}

	iterations := cfg.Iterations
	if iterations == 0 {
		iterations = defaultIterations
	}

	tolerance := cfg.ToleranceDB
	if tolerance == 0 {
		tolerance = defaultTolerance
	}

	previousError := math.Inf(1)
	performed := 0

	for i := 0; i < iterations; i++ {
		linearSpectrum, transformErr := w.forwardReal(linearPart)
		if transformErr != nil {
			return Result{}, transformErr
		}

		residualMagnitude = regularisedMagnitudeDivision(
			targetSpectrum,
			linearSpectrum,
			epsilon,
		)

		candidateMinimumPart, transformErr := designMinimumPart(
			w,
			residualMagnitude,
			minimumLength,
			epsilon,
			cfg,
		)
		if transformErr != nil {
			return Result{}, transformErr
		}

		minimumSpectrum, transformErr = w.forwardReal(candidateMinimumPart)
		if transformErr != nil {
			return Result{}, transformErr
		}

		residualMagnitude = regularisedMagnitudeDivision(
			targetSpectrum,
			minimumSpectrum,
			epsilon,
		)

		candidateLinearPart, transformErr := designLinearPart(
			w,
			residualMagnitude,
			linearLength,
			cfg,
		)
		if transformErr != nil {
			return Result{}, transformErr
		}

		taps, convolutionErr := conv.Direct(
			candidateMinimumPart,
			candidateLinearPart,
		)
		if convolutionErr != nil {
			return Result{}, fmt.Errorf(
				"mixedphase: convolve factors: %w",
				convolutionErr,
			)
		}

		metrics, analyzeErr := analyzeAgainstSpectrum(w, targetSpectrum, taps)
		if analyzeErr != nil {
			return Result{}, analyzeErr
		}

		currentError := metrics.RMSMagnitudeErrorDB

		if tolerance >= 0 && currentError > previousError {
			break
		}

		minimumPart = candidateMinimumPart
		linearPart = candidateLinearPart
		performed = i + 1

		if tolerance >= 0 &&
			math.Abs(previousError-currentError) < tolerance {
			break
		}

		previousError = currentError
	}

	taps, err := conv.Direct(minimumPart, linearPart)
	if err != nil {
		return Result{}, fmt.Errorf("mixedphase: convolve factors: %w", err)
	}

	metrics, err := analyzeAgainstSpectrum(w, targetSpectrum, taps)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Taps:             taps,
		MinimumPhasePart: minimumPart,
		LinearPhasePart:  linearPart,
		Iterations:       performed,
		Metrics:          metrics,
	}, nil
}

func designMinimumPart(
	w *fftWorkspace,
	targetMagnitude []float64,
	length int,
	epsilon float64,
	cfg IterativeConfig,
) ([]float64, error) {
	spectrum, _, err := minimumPhaseSpectrum(
		w,
		targetMagnitude,
		epsilon,
		cfg.Method,
	)
	if err != nil {
		return nil, err
	}

	impulse, err := w.inverseReal(spectrum)
	if err != nil {
		return nil, err
	}

	part := append([]float64(nil), impulse[:length]...)
	applyMinimumWindow(part, cfg)

	return part, nil
}

func designLinearPart(
	w *fftWorkspace,
	targetMagnitude []float64,
	length int,
	cfg IterativeConfig,
) ([]float64, error) {
	spectrum := make([]complex128, w.size)
	for i := range spectrum {
		spectrum[i] = complex(targetMagnitude[i], 0)
	}

	zeroPhase, err := w.inverseReal(spectrum)
	if err != nil {
		return nil, err
	}

	half := (length - 1) / 2
	part := make([]float64, length)

	for offset := -half; offset <= half; offset++ {
		index := offset
		if index < 0 {
			index += w.size
		}

		part[offset+half] = zeroPhase[index]
	}

	applyLinearWindow(part, cfg)

	return part, nil
}

func applyMinimumWindow(part []float64, cfg IterativeConfig) {
	if cfg.Window == window.TypeRectangular {
		return
	}

	options := []window.Option{window.WithSlope(window.SlopeRight)}
	if cfg.WindowAlpha > 0 {
		options = append(options, window.WithAlpha(cfg.WindowAlpha))
	}

	window.Apply(cfg.Window, part, options...)
}

func applyLinearWindow(part []float64, cfg IterativeConfig) {
	if cfg.Window == window.TypeRectangular {
		return
	}

	options := make([]window.Option, 0, 1)
	if cfg.WindowAlpha > 0 {
		options = append(options, window.WithAlpha(cfg.WindowAlpha))
	}

	window.Apply(cfg.Window, part, options...)
}
