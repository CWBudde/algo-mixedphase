//go:build js && wasm

// Command mixedphase-lab exposes the filter designs to the Mixed Phase Lab page.
package main

import (
	"syscall/js"

	"github.com/cwbudde/algo-mixedphase/graphiceq"
	"github.com/cwbudde/algo-mixedphase/internal/labresponse"
)

func main() {
	api := js.Global().Get("Object").New()
	api.Set("designMixedPhase", export(designMixedPhase))
	api.Set("designGraphicEQ", export(designGraphicEQ))
	api.Set("targets", targetNames())
	js.Global().Set("mixedphaseLab", api)

	select {}
}

// targetNames publishes the accepted target names so that the page's own copy
// of the list can be checked against the engine rather than trusted.
func targetNames() js.Value {
	names, err := labresponse.TargetNames()
	if err != nil {
		return js.Global().Get("Array").New(0)
	}

	out := js.Global().Get("Array").New(len(names))
	for index, name := range names {
		out.SetIndex(index, name)
	}

	return out
}

// designMixedPhase designs the requested target with the requested method and
// returns the taps together with the plotted response.
//
// The single argument object carries: method, target, length, cutoff, delay,
// toleranceDB and iterations. cutoff applies only to the adjustable low-pass;
// every other target is a fixed comparison fixture.
func designMixedPhase(args []js.Value) any {
	if len(args) < 1 {
		return errorObject("missing request")
	}

	request := args[0]

	design, err := labresponse.Design(labresponse.Request{
		Method:      stringField(request, "method", ""),
		Target:      stringField(request, "target", labresponse.LowpassTarget),
		Length:      intField(request, "length", 129),
		Cutoff:      floatField(request, "cutoff", 0.08),
		Delay:       intField(request, "delay", 0),
		ToleranceDB: floatField(request, "toleranceDB", 1),
		Iterations:  intField(request, "iterations", 12),
	})
	if err != nil {
		return errorObject(err.Error())
	}

	result := design.Result

	out := js.Global().Get("Object").New()
	out.Set("taps", floatArray(result.Taps))
	out.Set("magnitudeDB", floatArray(design.Realised.MagnitudeDB))
	out.Set("groupDelay", floatArray(design.Realised.GroupDelay))
	out.Set("referenceMagnitudeDB", floatArray(design.Prototype.MagnitudeDB))
	out.Set("iterations", result.Iterations)
	out.Set("usedDelay", design.UsedDelay)
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

func stringField(object js.Value, name, fallback string) string {
	value := object.Get(name)
	if value.Type() != js.TypeString {
		return fallback
	}

	return value.String()
}
