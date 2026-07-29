package labresponse

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

	prototype, err := LowpassPrototype(length, cutoff)
	if err != nil {
		t.Fatalf("LowpassPrototype: %v", err)
	}

	response := New(prototype)
	visibleBeyondPointOne := 0

	for index, magnitudeDB := range response.MagnitudeDB {
		frequency := 0.5 * float64(index) / float64(Points-1)
		if frequency <= 0.1 || magnitudeDB <= visibleThreshold {
			continue
		}

		visibleBeyondPointOne++

		if difference := math.Abs(response.GroupDelay[index] - wantDelay); difference > 1e-8 {
			t.Fatalf(
				"group delay at frequency %.6f = %.12g, want %.12g (difference %.3g)",
				frequency,
				response.GroupDelay[index],
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
	response := New(taps)

	for index, delay := range response.GroupDelay {
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

func TestLowpassPrototypeRejectsOutOfRangeRequests(t *testing.T) {
	tests := []struct {
		name   string
		length int
		cutoff float64
	}{
		{name: "negative length", length: -1, cutoff: 0.1},
		{name: "zero length", length: 0, cutoff: 0.1},
		{name: "single tap divides by zero", length: 1, cutoff: 0.1},
		{name: "two taps", length: 2, cutoff: 0.1},
		{name: "length above bound", length: MaxPrototypeLength + 1, cutoff: 0.1},
		{name: "zero cutoff", length: 65, cutoff: 0},
		{name: "negative cutoff", length: 65, cutoff: -0.1},
		{name: "cutoff at Nyquist", length: 65, cutoff: 0.5},
		{name: "cutoff above Nyquist", length: 65, cutoff: 0.9},
		{name: "NaN cutoff", length: 65, cutoff: math.NaN()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			taps, err := LowpassPrototype(test.length, test.cutoff)
			if err == nil {
				t.Fatalf("LowpassPrototype(%d, %v) = %v, want error",
					test.length, test.cutoff, taps)
			}

			if taps != nil {
				t.Fatalf("taps = %v, want nil alongside the error", taps)
			}
		})
	}
}

func TestLowpassPrototypeIsFiniteAndNormalised(t *testing.T) {
	taps, err := LowpassPrototype(65, 0.08)
	if err != nil {
		t.Fatalf("LowpassPrototype: %v", err)
	}

	sum := 0.0

	for i, tap := range taps {
		if math.IsNaN(tap) || math.IsInf(tap, 0) {
			t.Fatalf("tap[%d] = %v, want finite", i, tap)
		}

		sum += tap
	}

	if math.Abs(sum-1) > 1e-12 {
		t.Fatalf("tap sum = %.12g, want 1", sum)
	}
}
