package main

import (
	"math"
	"testing"
)

func TestResponseGroupDelayDoesNotWrapAroundStopbandZeros(t *testing.T) {
	const (
		length           = 129
		cutoff           = 0.08
		visibleThreshold = -60.0
		wantDelay        = float64(length-1) / 2
	)

	response := newResponse(lowpassPrototype(length, cutoff))
	visibleBeyondPointOne := 0

	for index, magnitudeDB := range response.magnitudeDB {
		frequency := 0.5 * float64(index) / float64(responsePoints-1)
		if frequency <= 0.1 || magnitudeDB <= visibleThreshold {
			continue
		}

		visibleBeyondPointOne++
		if difference := math.Abs(response.groupDelay[index] - wantDelay); difference > 1e-8 {
			t.Fatalf(
				"group delay at frequency %.6f = %.12g, want %.12g (difference %.3g)",
				frequency,
				response.groupDelay[index],
				wantDelay,
				difference,
			)
		}
	}

	if visibleBeyondPointOne == 0 {
		t.Fatal("default response has no visible bins beyond frequency 0.1")
	}
}

func TestResponseGroupDelayOfPureDelay(t *testing.T) {
	taps := make([]float64, 9)
	taps[3] = 1
	response := newResponse(taps)

	for index, delay := range response.groupDelay {
		if difference := math.Abs(delay - 3); difference > 1e-12 {
			t.Fatalf(
				"group delay[%d] = %.12g, want 3 (difference %.3g)",
				index,
				delay,
				difference,
			)
		}
	}
}
