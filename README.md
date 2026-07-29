# algo-mixedphase

Mixed-phase FIR filter design in Go, and a reproducible comparison of the methods that
produce it. This repository is the software companion to Christian-W. Budde's DAGA 2012
paper on fixed-support mixed-phase filter design. Read the
[revised English paper (PDF)][paper-pdf] or browse its
[Typst sources](docs/paper/).

**Module**: `github.com/cwbudde/algo-mixedphase`

Every filter has to spend its delay budget somewhere. A linear-phase FIR spends all of it
and pays with pre-ringing; a minimum-phase FIR spends almost none and pays with phase
distortion. The interesting designs live in between, and this repository implements four
ways of getting there — then measures them against each other under identical tap budgets,
delay constraints, target samples and frequency weights.

## Packages

| Package      | Description                                                                                                                                                                                                                                                                                                        |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `mixedphase` | Fixed-length mixed-phase FIR design: DAGA 2012 alternating factorisation, direct phase-interpolation baseline, weighted complex least squares with Lawson minimax refinement, direct low-group-delay optimisation (Wu–Gao–Teo), cepstral and discrete-Hilbert minimum-phase reconstruction, and comparison metrics |
| `graphiceq`  | Low-latency octave graphic EQ after Bruschi–Välimäki–Liski–Cecchi (DAFx 2022): the lowest bands become a cascade of shelving biquads and the rest one linear-phase FIR, halving latency per offloaded band                                                                                                         |

`graphiceq` is deliberately _not_ part of the general mixed-phase API. It answers the same
question — how to buy latency back — but only for targets that are a set of band gains, by
changing the filter structure rather than its phase.

## Quick Start

```bash
go get github.com/cwbudde/algo-mixedphase@latest
```

```go
package main

import (
	"fmt"

	"github.com/cwbudde/algo-mixedphase/mixedphase"
)

func main() {
	prototype := []float64{ /* linear-phase prototype impulse response */ }

	result, err := mixedphase.DesignIterative(prototype, mixedphase.IterativeConfig{
		Length: 65,
		Delay:  8, // samples of the 32 a linear-phase design would need
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(result.Metrics.RMSMagnitudeErrorDB, result.Metrics.EnergyCentroid)
}
```

## Web demo

Try the [Mixed Phase Lab](https://cwbudde.github.io/algo-mixedphase/) to compare the
design methods interactively in the browser. The Go implementations run directly through
WebAssembly.

To run the lab locally:

```bash
just web-demo # Build and serve at http://localhost:8787
```

## Comparing the methods

The common reference suite covers low-pass, parametric-EQ, crossover,
deep-notch, and measured room-correction targets. It records quality, delay,
pre-ringing, coefficient range, iteration, and runtime metrics for all four
general mixed-phase methods. The separate graphic-EQ comparison remains scoped
to its octave-band structure.

```bash
just compare       # Regenerate the committed comparison and paper-response CSVs
just compare-check # Prove those files are byte-reproducible
```

The comparison artifacts carry no timings, so they are byte-identical on every run and
every machine, and CI fails if regenerating them changes a single byte. Machine-local
runtimes live separately in `docs/reference-timings.csv` (`just compare-timings`).

`docs/graphiceq-results.csv` places each hybrid split next to an all-FIR design
forced to the same tap count, which is the comparison that actually decides
whether the split is worth taking:

```
method,iir_bands,taps,latency,rms_error_db,max_error_db
hybrid,0,3073,1536,0.014209,0.579788
hybrid,2,769,384,0.024067,0.477574
all-fir-equal-latency,0,769,384,0.113490,2.665199
```

See [docs/MIXED_PHASE_FILTER_DESIGN.md](docs/MIXED_PHASE_FILTER_DESIGN.md) for the
methods, the measured trade-offs, and the failure modes each one has.

## Relationship to algo-dsp

This repository builds on [`algo-dsp`](https://github.com/cwbudde/algo-dsp) — windows,
convolution, spectrum utilities, biquad runtime and biquad designers — and on
[`algo-fft`](https://github.com/cwbudde/algo-fft) for transforms. It depends on the public
algo-dsp API only; nothing is duplicated here.

The split is one of purpose. algo-dsp is a general-purpose, production-quality DSP library
heading for a stable v1.0. This repository is a research companion: it carries a paper, a
comparison harness, an interactive lab, and a roadmap that is driven by which method to
try next rather than by API stability.

## Development

Requirements: Go 1.25+ (the floor is set by `algo-dsp` and `algo-fft`), `just` (optional).
Building the paper additionally requires Typst 0.15.0.

```bash
just test       # Run all tests
just test-cross-build # Check native vs JavaScript/WASM determinism
just test-web   # Exercise lab state and the real worker/WASM browser path
just test-race  # Run tests with race detector
just lint       # Run golangci-lint
just fmt        # Format code
just bench      # Run benchmarks
just ci         # Run all CI checks
just web-demo   # Build and serve the Mixed Phase Lab locally
just paper      # Build the revised English paper
just paper-watch # Rebuild the paper while editing
```

## Project Docs

- [PLAN.md](PLAN.md) -- roadmap
- [CHANGELOG.md](CHANGELOG.md) -- release notes
- [docs/MIXED_PHASE_FILTER_DESIGN.md](docs/MIXED_PHASE_FILTER_DESIGN.md) -- method notes and measurements
- [Revised English paper (PDF)][paper-pdf] -- CI-built asset from the latest tagged release
- [docs/paper](docs/paper/) -- canonical Typst sources and build notes

## License

See [LICENSE](LICENSE).

[paper-pdf]: https://github.com/cwbudde/algo-mixedphase/releases/latest/download/mixed-phase-filter-design-en.pdf
