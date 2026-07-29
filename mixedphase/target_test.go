package mixedphase

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/spectrum"
)

// steepPhasePrototype is a band-limited low-pass cascaded twice with a notch
// whose zeros sit close to the unit circle. The resulting minimum phase
// advances by far more than pi between neighbouring bins of a design grid,
// which is the regime where recovering phase with atan2 and a bin-to-bin
// unwrapper loses whole turns.
func steepPhasePrototype() []float64 {
	base := make([]float64, 31)
	for i := range base {
		offset := float64(i) - 15

		sinc := 2 * 0.3
		if offset != 0 {
			sinc = math.Sin(2*math.Pi*0.3*offset) / (math.Pi * offset)
		}

		base[i] = sinc * (0.54 - 0.46*math.Cos(2*math.Pi*float64(i)/30))
	}

	notch := []float64{1, -2 * math.Cos(math.Pi/8), 1}

	return convolve(convolve(base, notch), notch)
}

func convolve(a, b []float64) []float64 {
	out := make([]float64, len(a)+len(b)-1)
	for i, left := range a {
		for j, right := range b {
			out[i+j] += left * right
		}
	}

	return out
}

// naiveUnwrappedPhase reproduces the phase a caller would get by recovering it
// from the reconstructed spectrum instead of taking it from
// minimumPhaseSpectrum. It exists only so the tests below can show that the
// difference is real and matters.
func naiveUnwrappedPhase(spectrumValues []complex128, half int) []float64 {
	wrapped := make([]float64, half+1)
	for i := range wrapped {
		wrapped[i] = cmplx.Phase(spectrumValues[i])
	}

	return spectrum.UnwrapPhase(wrapped)
}

func TestMinimumPhaseReturnsTheContinuousBranch(t *testing.T) {
	prototype := steepPhasePrototype()

	w, err := newFFTWorkspace(512)
	if err != nil {
		t.Fatalf("newFFTWorkspace() error = %v", err)
	}

	targetSpectrum, err := w.forwardReal(prototype)
	if err != nil {
		t.Fatalf("forwardReal() error = %v", err)
	}

	targetMagnitude := magnitude(targetSpectrum)
	epsilon := defaultEpsilon(targetMagnitude, 0)
	half := w.size / 2

	cepstrumSpectrum, cepstrumPhase, err := minimumPhaseSpectrum(
		w, targetMagnitude, epsilon, MethodCepstrum,
	)
	if err != nil {
		t.Fatalf("minimumPhaseSpectrum(cepstrum) error = %v", err)
	}

	_, hilbertPhase, err := minimumPhaseSpectrum(
		w, targetMagnitude, epsilon, MethodHilbert,
	)
	if err != nil {
		t.Fatalf("minimumPhaseSpectrum(hilbert) error = %v", err)
	}

	// The two reconstructions share no code path beyond the log magnitude, so
	// agreeing on the branch is independent corroboration that it is the true
	// continuous phase and not merely a self-consistent choice.
	worstDisagreement := 0.0
	for i := 0; i <= half; i++ {
		worstDisagreement = math.Max(
			worstDisagreement,
			math.Abs(cepstrumPhase[i]-hilbertPhase[i]),
		)
	}

	if worstDisagreement > 1e-9 {
		t.Errorf(
			"cepstrum and Hilbert phases disagree by %.6g rad, want agreement to 1e-9",
			worstDisagreement,
		)
	}

	// Both must still describe the reconstructed spectrum.
	for i := range cepstrumSpectrum {
		rebuilt := cmplx.Rect(cmplx.Abs(cepstrumSpectrum[i]), cepstrumPhase[i])
		if difference := cmplx.Abs(rebuilt - cepstrumSpectrum[i]); difference > 1e-12 {
			t.Fatalf(
				"bin %d: Rect(|H|, phase) differs from H by %.3g",
				i,
				difference,
			)
		}
	}

	// And the naive recovery must be demonstrably wrong here, otherwise this
	// fixture is not exercising the hazard the API exists to avoid.
	naive := naiveUnwrappedPhase(cepstrumSpectrum, half)

	worstNaive := 0.0
	for i := 0; i <= half; i++ {
		worstNaive = math.Max(worstNaive, math.Abs(cepstrumPhase[i]-naive[i]))
	}

	if worstNaive < 2*math.Pi {
		t.Fatalf(
			"naive unwrap differs by only %.6g rad; fixture no longer loses a turn",
			worstNaive,
		)
	}
}

func TestPrescribedResponseInterpolatesTheContinuousPhase(t *testing.T) {
	prototype := steepPhasePrototype()

	w, err := newFFTWorkspace(512)
	if err != nil {
		t.Fatalf("newFFTWorkspace() error = %v", err)
	}

	targetSpectrum, err := w.forwardReal(prototype)
	if err != nil {
		t.Fatalf("forwardReal() error = %v", err)
	}

	targetMagnitude := magnitude(targetSpectrum)
	epsilon := defaultEpsilon(targetMagnitude, 0)
	half := w.size / 2
	delay := float64(len(prototype)-1) / 2

	exactSpectrum, exactPhase, err := minimumPhaseSpectrum(
		w, targetMagnitude, epsilon, MethodCepstrum,
	)
	if err != nil {
		t.Fatalf("minimumPhaseSpectrum() error = %v", err)
	}

	naive := naiveUnwrappedPhase(exactSpectrum, half)

	// wrapToPi maps a phase difference into (-pi, pi] so the comparison is
	// insensitive to the representative, only to the interpolated value.
	wrapToPi := func(value float64) float64 {
		return math.Mod(math.Mod(value+math.Pi, 2*math.Pi)+2*math.Pi, 2*math.Pi) - math.Pi
	}

	for _, mix := range []float64{0.25, 0.5, 0.75} {
		desired := prescribedResponse(w, targetMagnitude, exactPhase, mix, delay)
		naiveDesired := prescribedResponse(w, targetMagnitude, naive, mix, delay)

		worstExact := 0.0
		worstNaive := 0.0

		for i := 1; i < half; i++ {
			omega := 2 * math.Pi * float64(i) / float64(w.size)
			want := (1-mix)*exactPhase[i] + mix*(-omega*delay)

			worstExact = math.Max(
				worstExact,
				math.Abs(wrapToPi(cmplx.Phase(desired[i])-want)),
			)
			worstNaive = math.Max(
				worstNaive,
				math.Abs(wrapToPi(cmplx.Phase(naiveDesired[i])-want)),
			)
		}

		if worstExact > 1e-9 {
			t.Errorf(
				"mix %.2f: prescribed phase deviates from the continuous "+
					"interpolation by %.6g rad",
				mix,
				worstExact,
			)
		}

		// A lost turn scales by (1-mix), so it survives interpolation for every
		// mix strictly between the endpoints. This is the defect the exact
		// branch prevents; if it ever stops showing up, the guard above has
		// stopped guarding anything.
		if worstNaive < 1e-3 {
			t.Errorf(
				"mix %.2f: naive phase reproduced the correct interpolation "+
					"(deviation %.6g); the test no longer discriminates",
				mix,
				worstNaive,
			)
		}
	}
}

func TestPhaseInterpolationEndpointsAreUnaffectedByTheBranch(t *testing.T) {
	// Whole turns vanish under Rect at the endpoints, so mix 0 and mix 1 agree
	// whichever phase is used. This documents why the endpoint tests elsewhere
	// in the package cannot catch a branch error.
	prototype := steepPhasePrototype()

	w, err := newFFTWorkspace(512)
	if err != nil {
		t.Fatalf("newFFTWorkspace() error = %v", err)
	}

	targetSpectrum, err := w.forwardReal(prototype)
	if err != nil {
		t.Fatalf("forwardReal() error = %v", err)
	}

	targetMagnitude := magnitude(targetSpectrum)
	epsilon := defaultEpsilon(targetMagnitude, 0)
	half := w.size / 2
	delay := float64(len(prototype)-1) / 2

	exactSpectrum, exactPhase, err := minimumPhaseSpectrum(
		w, targetMagnitude, epsilon, MethodCepstrum,
	)
	if err != nil {
		t.Fatalf("minimumPhaseSpectrum() error = %v", err)
	}

	naive := naiveUnwrappedPhase(exactSpectrum, half)

	for _, mix := range []float64{0, 1} {
		exactDesired := prescribedResponse(w, targetMagnitude, exactPhase, mix, delay)
		naiveDesired := prescribedResponse(w, targetMagnitude, naive, mix, delay)

		for i := range exactDesired {
			if difference := cmplx.Abs(exactDesired[i] - naiveDesired[i]); difference > 1e-9 {
				t.Fatalf(
					"mix %.0f bin %d: endpoints differ by %.3g, expected the "+
						"branch to be invisible here",
					mix,
					i,
					difference,
				)
			}
		}
	}
}
