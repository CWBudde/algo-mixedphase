# Mixed Phase Lab

An interactive front end for the designs in `mixedphase`, running as WebAssembly.

```bash
just web-demo          # builds the WASM and serves web/ on :8787
```

`build-wasm.sh` compiles `web/wasm` to `web/mixedphase_lab.wasm` and copies the matching
`wasm_exec.js` out of `GOROOT`. Both artefacts are build output — regenerate them rather
than editing them.

## What it shows

The lab designs a Hann-windowed low-pass with the selected method and plots the filter that
would actually run: magnitude against the prototype, group delay, and the impulse response
with the peak marked, so the energy sitting ahead of the peak is visible as pre-ringing.

The prototype matches `examples/mixedphase`, so a setting in the lab and a row in the CSV
describe the same filter — with one documented exception.

`DesignPhaseInterpolation`, `DesignComplexLeastSquares` and `DesignIterative` at zero delay
reproduce the CSV values exactly in the browser. `DesignIterative` with a nonzero delay does
not: its correction loop amplifies rounding differences geometrically, so the lab and the
native CSV agree to ten digits for one to four passes (5.366154, 4.609232, 4.917232,
4.686326 dB), part ways at six (4.605 native, 4.597 in WASM) and end far apart at the
default twelve (10.74 against 15.83). The numbers the lab shows for that method are
therefore platform-specific — see the conditioning item in [PLAN.md](../PLAN.md).

The delay control applies to the methods that prescribe a phase. `DesignLowGroupDelay`
chooses its own delay, so for that method the control is replaced by the magnitude
tolerance — which is the dial that actually buys delay there.

## Not implemented yet

A/B transient playback: designing is instant, but hearing pre-ringing needs a short
transient convolved against two designs and switched under a single gain. Tracked as
Phase 3 in [PLAN.md](../PLAN.md).
