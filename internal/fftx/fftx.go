// Package fftx holds the complex-plan workspace both design packages wrap.
//
// It exists only to keep one copy of the padding and real-part extraction that
// [github.com/cwbudde/algo-fft] deliberately leaves to its callers: the plan
// operates on complex128 slices of exactly the transform size, while every
// design here starts from a shorter real impulse response. Nothing in this
// package reimplements a transform.
package fftx

import (
	"fmt"

	algofft "github.com/cwbudde/algo-fft"
)

// Workspace owns one FFT plan and the buffers-sized conversions around it.
//
// The prefix is prepended to every wrapped error so each package keeps
// reporting failures under its own name.
type Workspace struct {
	size   int
	prefix string
	plan   *algofft.Plan[complex128]
}

// New allocates a workspace of the given transform size.
func New(size int, prefix string) (*Workspace, error) {
	plan, err := algofft.NewPlan64(size)
	if err != nil {
		return nil, fmt.Errorf("%s: create FFT plan: %w", prefix, err)
	}

	return &Workspace{size: size, prefix: prefix, plan: plan}, nil
}

// Size reports the transform length.
func (w *Workspace) Size() int {
	return w.size
}

// ForwardReal transforms a real signal, zero-padding or truncating it to the
// transform size.
func (w *Workspace) ForwardReal(input []float64) ([]complex128, error) {
	src := make([]complex128, w.size)
	for i := 0; i < len(input) && i < w.size; i++ {
		src[i] = complex(input[i], 0)
	}

	return w.forward(src)
}

// ForwardComplex transforms a complex signal, zero-padding it to the transform
// size.
func (w *Workspace) ForwardComplex(input []complex128) ([]complex128, error) {
	src := make([]complex128, w.size)
	copy(src, input)

	return w.forward(src)
}

func (w *Workspace) forward(src []complex128) ([]complex128, error) {
	dst := make([]complex128, w.size)
	if err := w.plan.Forward(dst, src); err != nil {
		return nil, fmt.Errorf("%s: forward FFT: %w", w.prefix, err)
	}

	return dst, nil
}

// InverseReal transforms a spectrum back and keeps the real part.
func (w *Workspace) InverseReal(input []complex128) ([]float64, error) {
	src := make([]complex128, w.size)
	copy(src, input)

	dst := make([]complex128, w.size)
	if err := w.plan.Inverse(dst, src); err != nil {
		return nil, fmt.Errorf("%s: inverse FFT: %w", w.prefix, err)
	}

	out := make([]float64, w.size)
	for i := range out {
		out[i] = real(dst[i])
	}

	return out, nil
}

// NextPowerOfTwo returns the smallest power of two that is at least both
// minimum and eight times length, which is the oversampling both packages use
// when the caller does not pick a grid.
func NextPowerOfTwo(length, minimum int) int {
	target := max(minimum, 8*length)

	size := 1
	for size < target {
		size <<= 1
	}

	return size
}
