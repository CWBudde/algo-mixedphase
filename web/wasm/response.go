package main

import "math"

const (
	// responsePoints is the number of frequency points the plots receive. It is
	// independent of the design grid: the curves are evaluated directly from
	// the taps, so the lab shows the filter that would run.
	responsePoints = 512

	responseMagnitudeFloor = 1e-12
)

// plottedResponse holds the curves the lab draws.
type plottedResponse struct {
	magnitudeDB []float64
	groupDelay  []float64
}

// newResponse evaluates taps on a uniform grid from DC to Nyquist. It evaluates
// group delay directly from -d(arg H)/d(omega), avoiding phase-unwrapping
// artefacts around response zeros.
func newResponse(taps []float64) plottedResponse {
	magnitudeDB := make([]float64, responsePoints)
	groupDelay := make([]float64, responsePoints)

	for k := range responsePoints {
		omega := math.Pi * float64(k) / float64(responsePoints-1)
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
		magnitudeDB[k] = 20 * math.Log10(max(magnitude, responseMagnitudeFloor))
		groupDelay[k] = (responseReal*slopeReal + responseImag*slopeImag) /
			max(magnitudeSquared, responseMagnitudeFloor*responseMagnitudeFloor)
	}

	return plottedResponse{magnitudeDB: magnitudeDB, groupDelay: groupDelay}
}

// lowpassPrototype builds the Hann-windowed sinc used as the design target. It
// matches the prototype in examples/mixedphase, so lab and CSV agree.
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

		windowValue := 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(length-1))
		taps[i] = sinc * windowValue
		sum += taps[i]
	}

	for i := range taps {
		taps[i] /= sum
	}

	return taps
}
