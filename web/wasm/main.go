//go:build js && wasm

// Command mixedphase-lab exposes the filter designs to the Mixed Phase Lab page.
package main

import (
	"syscall/js"

	"github.com/cwbudde/algo-mixedphase/graphiceq"
	"github.com/cwbudde/algo-mixedphase/mixedphase"
)

func main() {
	api := js.Global().Get("Object").New()
	api.Set("designMixedPhase", export(designMixedPhase))
	api.Set("designGraphicEQ", export(designGraphicEQ))
	js.Global().Set("mixedphaseLab", api)

	select {}
}

// designMixedPhase designs a low-pass with the requested method and returns the
// taps together with the plotted response.
//
// The single argument object carries: method, length, cutoff, delay,
// toleranceDB and iterations.
func designMixedPhase(args []js.Value) any {
	if len(args) < 1 {
		return errorObject("missing request")
	}

	request := args[0]
	length := intField(request, "length", 129)
	cutoff := floatField(request, "cutoff", 0.08)
	delay := intField(request, "delay", 0)
	iterations := intField(request, "iterations", 12)
	toleranceDB := floatField(request, "toleranceDB", 1)

	prototype := lowpassPrototype(length, cutoff)
	maximumDelay := (length - 1) / 2
	mix := 0.0

	if maximumDelay > 0 {
		mix = min(float64(delay)/float64(maximumDelay), 1)
	}

	var (
		result mixedphase.Result
		err    error
	)

	switch request.Get("method").String() {
	case "iterative":
		result, err = mixedphase.DesignIterative(prototype, mixedphase.IterativeConfig{
			Length:     length,
			Delay:      min(delay, maximumDelay),
			Iterations: iterations,
		})
	case "interpolation":
		result, err = mixedphase.DesignPhaseInterpolation(
			prototype,
			mixedphase.PhaseInterpolationConfig{Length: length, Mix: mix},
		)
	case "minimax":
		result, err = mixedphase.DesignComplexLeastSquares(
			prototype,
			mixedphase.ComplexLeastSquaresConfig{
				Length:            length,
				Mix:               mix,
				MinimaxIterations: iterations,
			},
		)
	case "lowdelay":
		result, err = mixedphase.DesignLowGroupDelay(
			prototype,
			mixedphase.LowGroupDelayConfig{
				Length:      length,
				FFTSize:     512,
				ToleranceDB: toleranceDB,
				Iterations:  iterations,
			},
		)
	default:
		return errorObject("unknown method")
	}

	if err != nil {
		return errorObject(err.Error())
	}

	response := newResponse(result.Taps)
	reference := newResponse(prototype)

	out := js.Global().Get("Object").New()
	out.Set("taps", floatArray(result.Taps))
	out.Set("magnitudeDB", floatArray(response.magnitudeDB))
	out.Set("groupDelay", floatArray(response.groupDelay))
	out.Set("referenceMagnitudeDB", floatArray(reference.magnitudeDB))
	out.Set("iterations", result.Iterations)
	out.Set("rmsErrorDB", result.Metrics.RMSMagnitudeErrorDB)
	out.Set("maxErrorDB", result.Metrics.MaxMagnitudeErrorDB)
	out.Set("relativeError", result.Metrics.RelativeMagnitudeError)
	out.Set("peakIndex", result.Metrics.PeakIndex)
	out.Set("energyCentroid", result.Metrics.EnergyCentroid)
	out.Set("prePeakEnergy", result.Metrics.PrePeakEnergyRatio)

	return out
}

// designGraphicEQ designs the hybrid equaliser and returns the FIR taps, the
// shelf cascade and the resulting latency.
//
// The single argument object carries: sampleRate, gains (an array of octave band
// gains starting at 31.25 Hz) and iirBands.
func designGraphicEQ(args []js.Value) any {
	if len(args) < 1 {
		return errorObject("missing request")
	}

	request := args[0]
	sampleRate := floatField(request, "sampleRate", 48000)
	gains := request.Get("gains")

	if gains.Type() != js.TypeObject || gains.Length() == 0 {
		return errorObject("gains must be a non-empty array")
	}

	bands := make([]graphiceq.Band, gains.Length())
	frequency := 31.25

	for i := range bands {
		bands[i] = graphiceq.Band{Frequency: frequency, GainDB: gains.Index(i).Float()}
		frequency *= 2
	}

	result, err := graphiceq.Design(graphiceq.Config{
		SampleRate: sampleRate,
		Bands:      bands,
		IIRBands:   intField(request, "iirBands", 0),
	})
	if err != nil {
		return errorObject(err.Error())
	}

	out := js.Global().Get("Object").New()
	out.Set("taps", floatArray(result.Taps))
	out.Set("latency", result.Latency)
	out.Set("sections", len(result.Sections))
	out.Set("rmsErrorDB", result.Metrics.RMSErrorDB)
	out.Set("maxErrorDB", result.Metrics.MaxErrorDB)
	out.Set("bandErrorDB", floatArray(result.Metrics.BandErrorDB))

	return out
}

func export(fn func(args []js.Value) any) js.Func {
	return js.FuncOf(func(_ js.Value, args []js.Value) any {
		return fn(args)
	})
}

func errorObject(message string) js.Value {
	out := js.Global().Get("Object").New()
	out.Set("error", message)

	return out
}

func floatArray(values []float64) js.Value {
	out := js.Global().Get("Array").New(len(values))
	for i, value := range values {
		out.SetIndex(i, value)
	}

	return out
}

func intField(object js.Value, name string, fallback int) int {
	value := object.Get(name)
	if value.Type() != js.TypeNumber {
		return fallback
	}

	return value.Int()
}

func floatField(object js.Value, name string, fallback float64) float64 {
	value := object.Get(name)
	if value.Type() != js.TypeNumber {
		return fallback
	}

	return value.Float()
}
