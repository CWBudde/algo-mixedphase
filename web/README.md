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

The lab holds one target and tap budget constant while two independently configured
methods compete. The target is either the adjustable Hann-windowed low-pass, whose
cutoff the visitor moves, or one of the six fixed fixtures from the published
comparison (`internal/reference`); selecting a fixed one hides the cutoff, because
its prototype is not rebuilt from it. It evaluates the filters that would actually
run and overlays:

- realised magnitude response against the common prototype, on a logarithmic frequency
  axis so that the bands the benchmark targets shape are legible;
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

The lab's default matches the cross-build golden in
`mixedphase.TestIterativeCrossBuildDeterminism`, so the opening screen and that test
describe the same filter. `DesignIterative` treats the iteration control as a maximum: at
delay 8 the first rising pass is discarded, so the default budget of twelve returns the
two-pass result at 4.609232 dB on both native and WASM builds. That test checks its metrics
within `2e-9` and selected coefficients within `1e-10`; see the conditioning measurements in
[MIXED_PHASE_FILTER_DESIGN.md](../docs/MIXED_PHASE_FILTER_DESIGN.md).

A benchmark target goes further: it is driven on the harness grid with the harness weights,
so at 129 taps, 16 samples of delay and the published iteration budget the lab reproduces
its row of [reference-results.csv](../docs/reference-results.csv) exactly rather than
approximately. `internal/labresponse.TestLabReproducesThePublishedComparison` asserts that
bit-for-bit across all thirty published rows, which is also what keeps the lab from quietly
offering a different experiment under the same labels. The `support-starved` preset opens on
the one target whose minimum-phase factor does not fit the taps the split allocates — the
only fixture where the alternating correction has real work to do.

`DesignLowGroupDelay` chooses its own delay, so its card replaces the delay control with
the magnitude tolerance—the dial that actually buys delay for that method. The adaptive
alternating method also chooses its own, by searching the budget rather than optimising
against a tolerance, so its delay slider is hidden too; the "Delay used" metric row is
where both report what they settled on.

## Tests

`lab-state.test.mjs` covers URL round trips, invalid input normalisation, A/B copy and swap,
stale-result rejection, and that every preset names a target the engine publishes.
`scripts/test-web.sh` builds a local browser session and checks the real worker/WASM path,
metric rendering, URL restoration, A/B recomputation, exports, browser errors, and the
390-pixel layout. It also designs every target in turn and compares the page's target list
against the one the engine reports, because the page needs its own copy to validate a shared
URL before the WASM module has loaded and two hand-maintained lists would otherwise drift.
