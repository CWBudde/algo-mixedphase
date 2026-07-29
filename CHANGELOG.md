# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `mixedphase`: fixed-length mixed-phase FIR design.
  - `DesignIterative` — alternating minimum-phase/linear-phase factorisation (DAGA 2012).
  - `DesignPhaseInterpolation` — direct complex-target baseline.
  - `DesignComplexLeastSquares` — weighted complex least squares with Lawson minimax
    refinement.
  - `DesignLowGroupDelay` — magnitude-constrained group-delay minimisation (Wu–Gao–Teo)
    on an L-BFGS backend.
  - Cepstral and discrete-Hilbert minimum-phase reconstruction, plus magnitude, complex
    and group-delay comparison metrics.
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
