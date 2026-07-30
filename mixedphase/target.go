package mixedphase

import (
	"math"
	"math/cmplx"
)

// maximumPhaseMix is the mix at which the prescribed phase reaches maximum
// phase, and therefore the upper bound both direct designs accept.
//
// The prescription is phi(mix) = (1-mix)*phi_min + mix*phi_lin with
// phi_lin = -omega*(N-1)/2, so mix = 2 gives phi = -omega*(N-1) - phi_min. For
// a real response that is exactly the maximum-phase spectrum of the same
// magnitude: negating the phase reverses the impulse response about the origin
// and the added full-length delay makes it causal again. The continuum
// therefore closes at two rather than at one.
const maximumPhaseMix = 2.0

// prescribedResponse builds the conjugate-symmetric complex target whose
// magnitude follows targetMagnitude and whose phase interpolates between the
// unwrapped minimum phase and a pure delay of the given number of samples.
//
// A mix of zero prescribes minimum phase, one prescribes linear phase and
// [maximumPhaseMix] prescribes maximum phase. Everything in between is the
// mixed-phase family both direct designs approximate.
//
// minimumPhase must be the continuous phase produced by [minimumPhaseSpectrum]
// rather than one recovered from a spectrum. Interpolating a wrapped phase is
// not the same operation: at a bin where the true phase has passed -3*pi, a
// wrapped value of -pi would be blended with the linear-phase target as though
// the design were a full turn ahead of where it is, and the interpolated
// response would be wrong by 2*pi*mix radians there. The endpoints mix = 0 and
// mix = 1 happen to be immune, because a whole turn is invisible after
// [cmplx.Rect]; every value in between is not.
func prescribedResponse(
	w *fftWorkspace,
	targetMagnitude []float64,
	minimumPhase []float64,
	mix float64,
	delay float64,
) []complex128 {
	half := w.size / 2

	desired := make([]complex128, w.size)

	for i := 0; i <= half; i++ {
		omega := 2 * math.Pi * float64(i) / float64(w.size)
		linearPhase := -omega * delay
		phase := (1-mix)*minimumPhase[i] + mix*linearPhase
		value := cmplx.Rect(targetMagnitude[i], phase)

		switch {
		case i == 0:
			desired[i] = complex(real(value), 0)
		case i == half && w.size%2 == 0:
			desired[i] = complex(real(value), 0)
		default:
			desired[i] = value
			desired[w.size-i] = cmplx.Conj(value)
		}
	}

	return desired
}
