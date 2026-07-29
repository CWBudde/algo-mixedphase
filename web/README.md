# Mixed Phase Lab

An A/B comparison workbench for the designs in `mixedphase`, running directly in the
browser through WebAssembly.

```bash
just web-demo          # build WASM and serve web/ on :8787
just test-web          # state tests plus a real headless-browser smoke test
```

`build-wasm.sh` compiles `web/wasm` to `web/mixedphase_lab.wasm` and copies the matching
`wasm_exec.js` out of `GOROOT`. Both files are build output; regenerate them rather than
editing them.

## What it shows

The lab holds one Hann-windowed low-pass target and tap budget constant while two
independently configured methods compete. It evaluates the filters that would actually
run and overlays:

- realised magnitude response against the common prototype;
- group delay, masked below −60 dB where phase is not meaningful;
- peak-aligned impulse responses with the pre-ringing region shaded; and
- cumulative impulse-response energy.

The sticky comparison table reports raw metrics and the B−A delta. Filter design runs in a
worker, so optimiser work cannot block the controls and a stale response cannot replace a
newer result.

## Audition and reproduction

The Web Audio transport sends one deterministic transient loop through the unfiltered
source and both FIRs simultaneously. The filtered paths are energy-matched, and switching
among source, A, and B changes gains without restarting playback.

Every control is encoded in the URL. The export actions save either both tap sets and their
metrics as JSON or the plotted responses as CSV.

The footer links to the implementation notes and to the CI-built PDF attached to the latest
tagged release. The browser smoke test pins that release-asset URL so it remains independent
of whether the site was built from `main` or a tag.

The default prototype matches `examples/mixedphase`, so a setting in the lab and a row in
the native comparison describe the same filter. `DesignIterative` treats the iteration
control as a maximum: at delay 8 the first rising pass is discarded, so the default budget
of twelve returns the two-pass result at 4.609232 dB on both native and WASM builds. The
cross-build golden test checks its metrics within `2e-9` and selected coefficients within
`1e-10`; see the conditioning measurements in
[MIXED_PHASE_FILTER_DESIGN.md](../docs/MIXED_PHASE_FILTER_DESIGN.md).

`DesignLowGroupDelay` chooses its own delay, so its card replaces the delay control with
the magnitude tolerance—the dial that actually buys delay for that method.

## Tests

`lab-state.test.mjs` covers URL round trips, invalid input normalisation, A/B copy and swap,
and stale-result rejection. `scripts/test-web.sh` builds a local browser session and checks
the real worker/WASM path, metric rendering, URL restoration, A/B recomputation, exports,
browser errors, and the 390-pixel layout.
