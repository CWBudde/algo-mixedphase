package mixedphase

import (
	"fmt"
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-mixedphase/internal/fftx"
)

// fftWorkspace adapts [fftx.Workspace] to this package's short call sites and
// its error prefix.
type fftWorkspace struct {
	*fftx.Workspace

	size int
}

func newFFTWorkspace(size int) (*fftWorkspace, error) {
	workspace, err := fftx.New(size, "mixedphase")
	if err != nil {
		return nil, err
	}

	return &fftWorkspace{Workspace: workspace, size: size}, nil
}

func (w *fftWorkspace) forwardReal(input []float64) ([]complex128, error) {
	return w.ForwardReal(input)
}

func (w *fftWorkspace) forwardComplex(input []complex128) ([]complex128, error) {
	return w.ForwardComplex(input)
}

func (w *fftWorkspace) inverseReal(input []complex128) ([]float64, error) {
	return w.InverseReal(input)
}

func magnitude(spectrum []complex128) []float64 {
	out := make([]float64, len(spectrum))
	for i, value := range spectrum {
		out[i] = cmplx.Abs(value)
	}

	return out
}

// minimumPhaseSpectrum reconstructs the causal minimum-phase spectrum that
// belongs to targetMagnitude using the requested method.
//
// The second result is the *continuous* minimum phase, in radians, on the same
// grid. Both routes compute it exactly — the cepstral one as the imaginary part
// of the complex log spectrum, the Hilbert one as the transform's own output —
// and both then fold it into a complex exponential, which wraps it to
// (-pi, pi]. Callers that need the phase itself must take it from here:
// recovering it from the spectrum with atan2 and a bin-to-bin unwrapper loses
// whole 2*pi turns wherever the phase advances by more than pi between
// neighbouring bins, which a steep or high-order target routinely does.
func minimumPhaseSpectrum(
	w *fftWorkspace,
	targetMagnitude []float64,
	epsilon float64,
	method MinimumPhaseMethod,
) ([]complex128, []float64, error) {
	switch method {
	case MethodCepstrum:
		return cepstrumMinimumPhaseSpectrum(w, targetMagnitude, epsilon)
	case MethodHilbert:
		return hilbertMinimumPhaseSpectrum(w, targetMagnitude, epsilon)
	default:
		return nil, nil, fmt.Errorf("%w: %d", ErrInvalidMethod, int(method))
	}
}

// flooredMagnitude copies targetMagnitude onto the workspace grid and applies
// the magnitude floor that keeps the logarithm finite.
//
// It does not resample: bin i of the input becomes bin i of the output. A
// shorter input is padded with the floor and a longer one is truncated, so
// every caller is expected to supply a spectrum already on the workspace grid.
func flooredMagnitude(
	size int,
	targetMagnitude []float64,
	epsilon float64,
) []float64 {
	out := make([]float64, size)
	for i := range out {
		out[i] = epsilon
		if i < len(targetMagnitude) && targetMagnitude[i] > epsilon {
			out[i] = targetMagnitude[i]
		}
	}

	return out
}

func logMagnitudeSpectrum(floored []float64) []complex128 {
	out := make([]complex128, len(floored))
	for i, value := range floored {
		out[i] = complex(math.Log(value), 0)
	}

	return out
}

// cepstrumMinimumPhaseSpectrum folds the real cepstrum onto its causal half and
// exponentiates the resulting complex log spectrum.
func cepstrumMinimumPhaseSpectrum(
	w *fftWorkspace,
	targetMagnitude []float64,
	epsilon float64,
) ([]complex128, []float64, error) {
	logMagnitude := logMagnitudeSpectrum(
		flooredMagnitude(w.size, targetMagnitude, epsilon),
	)

	cepstrum, err := w.inverseReal(logMagnitude)
	if err != nil {
		return nil, nil, err
	}

	minimumCepstrum := make([]complex128, w.size)
	minimumCepstrum[0] = complex(cepstrum[0], 0)

	half := w.size / 2

	positiveLimit := (w.size + 1) / 2
	for i := 1; i < positiveLimit; i++ {
		minimumCepstrum[i] = complex(2*cepstrum[i], 0)
	}

	if w.size%2 == 0 {
		minimumCepstrum[half] = complex(cepstrum[half], 0)
	}

	logSpectrum, err := w.forwardComplex(minimumCepstrum)
	if err != nil {
		return nil, nil, err
	}

	out := make([]complex128, w.size)
	phase := make([]float64, w.size)

	for i, value := range logSpectrum {
		out[i] = cmplx.Exp(value)
		// imag(logSpectrum) is the continuous phase: it is a linear transform
		// output and never went through atan2, so it carries no 2*pi wraps.
		phase[i] = imag(value)
	}

	return out, phase, nil
}

// hilbertMinimumPhaseSpectrum obtains the minimum phase from the discrete
// Hilbert transform of the log magnitude and pairs it with the floored target
// magnitude.
//
// Unlike the cepstral route the magnitude never passes through an exponential,
// so it is reproduced exactly on the design grid.
func hilbertMinimumPhaseSpectrum(
	w *fftWorkspace,
	targetMagnitude []float64,
	epsilon float64,
) ([]complex128, []float64, error) {
	floored := flooredMagnitude(w.size, targetMagnitude, epsilon)

	phase, err := discreteHilbertPhase(w, logMagnitudeSpectrum(floored))
	if err != nil {
		return nil, nil, err
	}

	out := make([]complex128, w.size)
	for i := range out {
		out[i] = cmplx.Rect(floored[i], phase[i])
	}

	// phase is returned unwrapped; cmplx.Rect above has already wrapped the
	// copy embedded in out.
	return out, phase, nil
}

// discreteHilbertPhase evaluates the minimum phase belonging to the given log
// magnitude.
//
// For a minimum-phase spectrum log magnitude and phase form a Hilbert pair. The
// transform is applied as a sign multiplication in the quefrency domain, which
// is the discrete equivalent of convolving the log magnitude with the
// cot(omega/2) kernel while avoiding that kernel's singularity. The DC and
// Nyquist quefrencies are excluded because they carry no quadrature component.
func discreteHilbertPhase(
	w *fftWorkspace,
	logMagnitude []complex128,
) ([]float64, error) {
	quefrency, err := w.inverseReal(logMagnitude)
	if err != nil {
		return nil, err
	}

	half := w.size / 2
	positiveLimit := (w.size + 1) / 2
	signed := make([]complex128, w.size)

	for i := 1; i < positiveLimit; i++ {
		signed[i] = complex(quefrency[i], 0)
	}

	for i := half + 1; i < w.size; i++ {
		signed[i] = complex(-quefrency[i], 0)
	}

	transformed, err := w.forwardComplex(signed)
	if err != nil {
		return nil, err
	}

	phase := make([]float64, w.size)
	for i, value := range transformed {
		phase[i] = imag(value)
	}

	return phase, nil
}

// regularisedMagnitudeDivision forms the magnitude quotient |N|/|D| in the
// Tikhonov-regularised form
//
//	|N|*|D| / (|D|^2 + epsilon^2)
//
// which agrees with the plain quotient wherever |D| is comfortably above
// epsilon and stays finite where it is not. The behaviour near a null of the
// denominator is deliberate and worth knowing: the quotient peaks at
// |N|/(2*epsilon) when |D| equals epsilon and then falls back to zero, rather
// than diverging. A factor is therefore never asked to correct for a null in
// the other factor by an unbounded amount — but neither does it correct fully,
// so a target with a genuine spectral zero is approached from below.
//
// Only magnitudes take part: the phase of both operands is discarded, and the
// caller re-imposes whichever phase the factor is supposed to carry. That is
// what makes the alternating factorisation's quotient step a magnitude
// operation rather than the complex division the 2012 paper describes.
func regularisedMagnitudeDivision(
	numerator, denominator []complex128,
	epsilon float64,
) []float64 {
	out := make([]float64, len(numerator))

	for i := range out {
		num := cmplx.Abs(numerator[i])
		den := cmplx.Abs(denominator[i])
		out[i] = num * den / (den*den + epsilon*epsilon)
	}

	return out
}

// minimumDesignOversampling is the smallest ratio between the design grid and
// the filter length that this package will accept.
//
// A grid equal to the filter length makes every reported error meaningless
// rather than merely coarse: the design is then an interpolation problem with
// as many free coefficients as constraints, so the projection reproduces the
// prescribed response exactly at every grid point and the metrics — which are
// evaluated on that same grid — read zero however badly the filter behaves in
// between. Two is the least ratio that leaves the metrics something to measure;
// the default below is considerably more generous.
const minimumDesignOversampling = 2

func nextDesignFFTSize(filterLength, requested int) (int, error) {
	if requested != 0 {
		if requested < minimumDesignOversampling*filterLength {
			return 0, fmt.Errorf(
				"%w: FFT size %d does not oversample filter length %d by at "+
					"least %d; metrics on such a grid are self-confirming",
				ErrInvalidLength,
				requested,
				filterLength,
				minimumDesignOversampling,
			)
		}

		if requested%2 != 0 {
			return 0, fmt.Errorf(
				"%w: FFT size %d is odd; the Nyquist bin handling assumes an "+
					"even grid",
				ErrInvalidLength,
				requested,
			)
		}

		return requested, nil
	}

	return fftx.NextPowerOfTwo(filterLength, 16), nil
}

func defaultEpsilon(targetMagnitude []float64, requested float64) float64 {
	if requested > 0 {
		return requested
	}

	peak := 0.0
	for _, value := range targetMagnitude {
		peak = max(peak, value)
	}

	if peak == 0 {
		return 1e-12
	}

	return peak * 1e-12
}
