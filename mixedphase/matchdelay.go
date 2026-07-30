package mixedphase

import "math"

// matchDelayMagnitudeFloor bounds the 1/|H| factor of the magnitude gradient,
// relative to the target peak. Where the realised response is numerically zero
// the derivative of |H| is not defined; flooring the divisor keeps the push
// towards the target finite and correctly signed instead.
const matchDelayMagnitudeFloor = 1e-6

// prepareMatchDelay switches the problem to the form [DesignContinuum] needs
// outside the reachable window: magnitude error minimised subject to a
// requested weighted mean group delay.
//
// The Wu-Gao-Teo form of [evaluate] takes a magnitude band and reports whatever
// delay it can reach under it, which makes the tolerance the knob and the delay
// an outcome. A delay-parameterised design needs the opposite assignment, so
// the objective and the constraint swap roles,
//
//	minimise   sum_k m_k * (|H_k| - T_k)^2
//	subject to sum_k w_k * tau_k = requested
//
// solved by the same penalty ladder around the same limited-memory BFGS
// minimiser. The magnitude weight m_k is uniform over the grid, with the
// interior bins counted twice for conjugate symmetry, and the objective is
// normalised by the target energy so that its value is the square of the
// relative magnitude error the metrics report. Weighting magnitude uniformly
// rather than by the delay band is deliberate: the delay band deliberately
// masks stopbands, and a magnitude objective that did the same would let them
// drift without penalty.
func (p *lowDelayProblem) prepareMatchDelay(requested float64) {
	p.matchDelay = true
	p.requestedDelay = requested
	p.delayGrad = make([]float64, p.length)
	p.magnitudeWeight = make([]float64, p.bins)

	total := 0.0

	for k := range p.bins {
		multiplicity := 2.0
		if k == 0 || (k == p.bins-1 && p.size%2 == 0) {
			multiplicity = 1
		}

		p.magnitudeWeight[k] = multiplicity
		total += multiplicity * p.target[k] * p.target[k]
	}

	if total == 0 {
		return
	}

	for k := range p.magnitudeWeight {
		p.magnitudeWeight[k] /= total
	}
}

// evaluateMatchDelay returns the penalised objective of the matched form at
// taps and writes its gradient.
//
// The per-bin quantities are the same four sums [evaluate] accumulates: with
// H = A + jB and dH/domega = -j*(C + jD), the group delay is (A*C + B*D)/|H|^2.
// The one structural difference is that the constraint is now a single scalar
// over the whole grid rather than one inequality per bin, so its residual is
// unknown until every bin has been visited. The gradient of the weighted mean
// delay is therefore accumulated separately during the sweep and scaled by the
// residual afterwards, which costs one vector of taps and no second sweep.
func (p *lowDelayProblem) evaluateMatchDelay(taps, grad []float64) float64 {
	for i := range grad {
		grad[i] = 0
	}

	for i := range p.delayGrad {
		p.delayGrad[i] = 0
	}

	magnitudeFloor := math.Sqrt(p.denominatorFloor * matchDelayMagnitudeFloor)
	magnitudeTotal := 0.0
	weightedDelay := 0.0

	for k := range p.bins {
		row := k * p.length
		cosRow := p.cosine[row : row+p.length]
		sinRow := p.sine[row : row+p.length]

		responseReal, responseImag, slopeReal, slopeImag := 0.0, 0.0, 0.0, 0.0

		for n, tap := range taps {
			cosine, sine := cosRow[n], sinRow[n]
			responseReal += tap * cosine
			responseImag -= tap * sine
			slopeReal += float64(n) * tap * cosine
			slopeImag -= float64(n) * tap * sine
		}

		squared := responseReal*responseReal + responseImag*responseImag
		denominator := max(squared, p.denominatorFloor)

		delay := (responseReal*slopeReal + responseImag*slopeImag) / denominator
		weight := p.weight[k]
		weightedDelay += weight * delay

		// As in evaluate, the derivative of a floored denominator vanishes, so a
		// null stops steering the search rather than dominating it.
		derivativeScale := 2.0
		if squared <= p.denominatorFloor {
			derivativeScale = 0
		}

		firstTerm := weight * (slopeReal - derivativeScale*delay*responseReal) /
			denominator
		secondTerm := weight * (slopeImag - derivativeScale*delay*responseImag) /
			denominator
		delayScale := weight / denominator

		response := math.Sqrt(squared)
		deviation := response - p.target[k]
		magnitudeWeight := p.magnitudeWeight[k]
		magnitudeTotal += magnitudeWeight * deviation * deviation

		// d/dh of (|H| - T)^2 is 2*(|H| - T)/|H| times d(|H|^2/2)/dh.
		magnitudeScale := 2 * magnitudeWeight * deviation /
			max(response, magnitudeFloor)

		for n := range taps {
			cosine, sine := cosRow[n], sinRow[n]
			energy := responseReal*cosine - responseImag*sine

			grad[n] += magnitudeScale * energy
			p.delayGrad[n] += cosine*firstTerm - sine*secondTerm +
				float64(n)*energy*delayScale
		}
	}

	residual := weightedDelay - p.requestedDelay
	scale := 2 * p.penalty * residual

	for n := range grad {
		grad[n] += scale * p.delayGrad[n]
	}

	return magnitudeTotal + p.penalty*residual*residual
}
