// Package labresponse evaluates the curves the Mixed Phase Lab draws.
//
// It lives outside the WebAssembly command so that it builds and tests on every
// platform: the command itself is guarded by a js/wasm build tag, and a package
// that only that command could compile would never be exercised by
// "go test ./...".
package labresponse

import (
	"fmt"
	"math"
)

const (
	// Points is the number of frequency points the plots receive. It is
	// independent of the design grid: the curves are evaluated directly from
	// the taps, so the lab shows the filter that would run.
	Points = 512

	magnitudeFloor = 1e-12

	// MinPrototypeLength is the shortest prototype LowpassPrototype accepts.
	// Two taps would make the window denominator length-1 degenerate to one
	// sample and a single tap would divide by zero.
	MinPrototypeLength = 3

	// MaxPrototypeLength bounds the work a single lab request can ask for.
	MaxPrototypeLength = 4097
)

// Response holds the curves the lab draws.
type Response struct {
	MagnitudeDB []float64
	GroupDelay  []float64
}

// New evaluates taps on a uniform grid from DC to Nyquist. It evaluates group
// delay directly from -d(arg H)/d(omega), avoiding phase-unwrapping artefacts
// around response zeros.
func New(taps []float64) Response {
	magnitudeDB := make([]float64, Points)
	groupDelay := make([]float64, Points)

	for k := range Points {
		omega := math.Pi * float64(k) / float64(Points-1)
		responseReal := 0.0
		responseImag := 0.0
		slopeReal := 0.0
		slopeImag := 0.0

		for n, tap := range taps {
			sine, cosine := math.Sincos(omega * float64(n))
			termReal := tap * cosine
			termImag := -tap * sine
			responseReal += termReal
			responseImag += termImag
			slopeReal += float64(n) * termReal
			slopeImag += float64(n) * termImag
		}

		magnitudeSquared := responseReal*responseReal + responseImag*responseImag
		magnitude := math.Sqrt(magnitudeSquared)
		magnitudeDB[k] = 20 * math.Log10(max(magnitude, magnitudeFloor))
		groupDelay[k] = (responseReal*slopeReal + responseImag*slopeImag) /
			max(magnitudeSquared, magnitudeFloor*magnitudeFloor)
	}

	return Response{MagnitudeDB: magnitudeDB, GroupDelay: groupDelay}
}

// LowpassPrototype builds a Hann-windowed sinc for the lab to design against.
//
// It is the lab's own fixture and is deliberately not shared with the reference
// suite, which builds its prototypes from magnitude curves in
// internal/reference. The two therefore need not agree.
//
// The length must lie in [MinPrototypeLength, MaxPrototypeLength] and the
// cutoff in (0, 0.5); both are rejected rather than clamped so that a bad lab
// request surfaces as an error instead of a silently wrong curve.
func LowpassPrototype(length int, cutoff float64) ([]float64, error) {
	if length < MinPrototypeLength || length > MaxPrototypeLength {
		return nil, fmt.Errorf(
			"length %d is outside [%d, %d]",
			length,
			MinPrototypeLength,
			MaxPrototypeLength,
		)
	}

	if !(cutoff > 0) || !(cutoff < 0.5) || math.IsNaN(cutoff) {
		return nil, fmt.Errorf("cutoff %v is outside (0, 0.5)", cutoff)
	}

	taps := make([]float64, length)
	middle := float64(length-1) / 2
	sum := 0.0

	for i := range taps {
		x := float64(i) - middle

		sinc := 2 * cutoff
		if x != 0 {
			sinc = math.Sin(2*math.Pi*cutoff*x) / (math.Pi * x)
		}

		windowValue := 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(length-1))
		taps[i] = sinc * windowValue
		sum += taps[i]
	}

	if sum == 0 {
		return nil, fmt.Errorf("prototype has zero DC gain")
	}

	for i := range taps {
		taps[i] /= sum
	}

	return taps, nil
}
