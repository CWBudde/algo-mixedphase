# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **Breaking.** `DesignPhaseInterpolation` and `DesignComplexLeastSquares` now interpolate
  the exact continuous minimum phase instead of one recovered from the reconstructed
  spectrum with `atan2` and a bin-to-bin unwrapper. The old route lost whole `2*pi` turns
  wherever the phase advanced by more than `pi` between neighbouring bins, which changed
  the prescribed response for every `Mix` strictly between 0 and 1. Endpoint mixes are
  unaffected.
- **Breaking.** Design entry points now reject an `FFTSize` below twice the filter length,
  and odd sizes, with `ErrInvalidLength`. A grid equal to the filter length made the
  reported error exactly zero by construction, because the metrics are evaluated on the
  same grid the design interpolates.
- **Breaking.** Every entry point now rejects non-finite prototype taps with
  `ErrNonFinitePrototype`, and non-finite configuration values with `ErrNonFiniteConfig`.
  Both previously produced all-`NaN` output and a nil error.

### Added

- Mixed Phase Lab: a target selector offering the six fixed fixtures from the published
  comparison alongside the adjustable low-pass. A benchmark target is driven on the harness
  grid with the harness weights, so at 129 taps and 16 samples of delay the lab reproduces
  its row of `docs/reference-results.csv` bit-for-bit; a new test asserts that across all
  thirty rows. Two presets open on the support-starved crossover and on the degenerate case
  it is read against. The magnitude and group-delay plots moved to a logarithmic frequency
  axis, without which an 800 Hz crossover at 48 kHz occupies the leftmost two percent of
  the frame, and the magnitude ceiling rose from +5 to +20 dB so that a boosting target is
  no longer clipped.

- `mixedphase`: fixed-length mixed-phase FIR design.
  - `DesignIterative` — alternating minimum-phase/linear-phase factorisation (DAGA 2012).
  - `DesignPhaseInterpolation` — direct complex-target baseline.
  - `DesignComplexLeastSquares` — weighted complex least squares with Lawson minimax
    refinement.
  - `DesignLowGroupDelay` — magnitude-constrained group-delay minimisation (Wu–Gao–Teo)
    on an L-BFGS backend.
  - Cepstral and discrete-Hilbert minimum-phase reconstruction, plus magnitude, complex
    and group-delay comparison metrics.
- Reference suite: a `steep-crossover` target and a `minphase-truncation` baseline. The
  five original targets are smooth enough that the alternating correction converges to
  the identity, so they measured the minimum-phase reconstruction rather than the
  factorisation; the new target and baseline separate the two.
- Reference table: RMS dB error and group-delay ripple columns. Ripple was computed but
  never published, and it is the axis on which the alternating factorisation is weakest.
- `graphiceq`: hybrid IIR/FIR low-latency octave graphic EQ (DAFx 2022). The lowest bands
  become a cascade of shelving biquads placed at the geometric mean of neighbouring
  centres; the remainder is one linear-phase FIR. Latency halves per offloaded band.
- A fixed five-target reference harness shared by all general mixed-phase
  methods, with committed quality/runtime results in
  `docs/reference-results.csv`; the graphic-EQ CSV remains a separate
  structure-specific comparison.
- `docs/MIXED_PHASE_FILTER_DESIGN.md` with the measured trade-offs and failure modes.

### Changed

- `DesignIterative` now discards the first correction pass whose RMS magnitude
  error rises, returning the preceding stable factors and reporting their
  accepted pass count. A native/JavaScript-WASM golden test guards the result.
- Extracted from [`algo-dsp`](https://github.com/cwbudde/algo-dsp), where this code lived
  as `dsp/filter/mixedphase` and `dsp/filter/graphiceq` during Phase 42. The packages were
  never released from there; algo-dsp remains an upstream dependency. Import paths are now
  `github.com/cwbudde/algo-mixedphase/{mixedphase,graphiceq}`; the package APIs are
  unchanged.
