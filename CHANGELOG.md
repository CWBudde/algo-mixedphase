# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- `DesignIterativeAuto`'s passband guard rejected designs far better on its own
  objective. The guard bounded relative magnitude error at a multiple of the
  zero-delay design's, which stops meaning anything once that design is already
  accurate: designing the reference eighth-order crossover into 193 taps, the
  three-times ceiling declined an eleven-sample budget worth **16 dB** of stopband
  depth in exchange for a passband regression under a hundredth of a decibel. The
  ceiling is now `max((1+RelativeErrorSlack)*E_rel(0), RelativeErrorFloor)`, with
  the new `RelativeErrorFloor` defaulting to 1e-2 — about 0.09 dB, below audibility.
  No published row moves: at the published operating point the guard never bound,
  and removing it entirely already gave the same six selections.
- `DesignIterativeAuto` bought latency for gains too small to matter, because its
  objective does not price latency. A 129-tap low-pass improves from 0.800 dB to
  0.498 dB at a one-sample budget, and the search took it — abandoning the exact
  minimum-phase design for three tenths of a decibel. A non-zero budget must now
  beat the zero-delay design by the new `MinimumImprovementDB`, default 1 dB, and
  ties go to the shorter delay. The gate is measured against the zero-delay design
  rather than the running incumbent, so the result does not depend on the order
  candidates are visited in.
- The documented cost of the delay search, "about 25 designs", was specific to 129
  taps. The count grows with the output length — roughly 24, 40, 72 and 136 designs
  at 129, 257, 513 and 1025 taps — and each design also costs more, so the search is
  about quadratic in length. Measured runtimes are now stated.

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

- **The phase continuum now reaches maximum phase.** `Mix` on
  `DesignPhaseInterpolation` and `DesignComplexLeastSquares` was clamped to
  `[0, 1]`, minimum through linear phase; it now accepts `[0, 2]`, where 2 is
  maximum phase. No new mathematics was needed: the prescription is
  `(1-mix)*phi_min + mix*phi_lin` with `phi_lin = -w*(N-1)/2`, so `mix = 2` gives
  `-w*(N-1) - phi_min`, which is exactly the maximum-phase response of the same
  magnitude. The family is consequently symmetric about linear phase —
  `Design(2-mix).Taps == reverse(Design(mix).Taps)`, pinned to 1e-12 against a
  peak tap of 0.371 by `TestPhaseContinuumReflectsAboutLinearPhase` — so every
  magnitude measure is symmetric in `Mix` and the upper half costs latency
  without recovering accuracy. It is included because the continuum is only
  complete with it, and because an equaliser or room correction wants the whole
  range exposed as one control. The change is additive: the bound only widens, so
  no existing call behaves differently and no published row moves.

- `docs/reference-phase-regimes.csv` — the two regimes on either side of the
  group-delay floor, written by `examples/mixedphase -regimes` and gated by
  `just compare-check`. Unlike the length sweep it deliberately reuses the
  published fixtures and operating point, so every row is comparable with
  `docs/reference-results.csv` rather than being a separate experiment.

  **The framing this settles.** A magnitude request implies a group-delay floor:
  every causal realisation differs from the minimum-phase one by an all-pass
  factor, whose group delay is non-negative, so no design of that magnitude is
  faster. The floors here are 0.50, 5.86, 6.21, 8.63, 10.41 and 49.37 samples,
  which is why a fixed 16-sample budget is generous for five targets and
  meaningless for the sixth. Latency above the floor *is* the all-pass factor,
  and the alternating factorisation takes the trivial one — hence the ripple
  invariance recorded above, now correctly stated as a property of the
  construction rather than a limit on what latency can buy. A prescribed phase
  over the same latency does convert it into flatness: at 44.9 samples on the
  low-pass the factorisation still carries 1.117 samples of ripple where the
  continuum is at 0.419.

  **Below the floor** no phase choice helps and only the magnitude can give way.
  Widening the low-group-delay optimiser's tolerance from 0.25 to 2 dB buys delay
  under the floor on five targets, room correction conceding the largest share of
  it at 80% and the parametric EQ the most samples at 4.28;
  the eighth-order crossover buys nothing, having no accuracy left to concede at
  129 taps. The ladder stops at 2 dB because wider tolerances stop measuring: at
  4 dB the optimiser has not converged and tracks its iteration budget rather
  than its tolerance, and at 8 dB the constraint admits a spectral null where
  group delay is undefined. Both are pinned by
  `TestLooseToleranceLeavesTheMeasurableRegime`.

- `docs/reference-delay-sweep.csv` — an output-length and delay-budget sweep,
  written by `examples/mixedphase -sweep` and gated by `just compare-check`. The six
  reference curves are rebuilt as 2049-tap fixtures and designed at 129, 257, 513 and
  1025 output taps with the budget strided over the admissible range, alongside a
  linear-phase family for comparison at equal latency. It is a separate artifact
  because a 257-tap prototype is shorter than a 513-tap filter, so at the published
  fixture length every method reproduces the target exactly; each row carries its
  `prototype_taps` so the two artifacts cannot be read as one.

  Its finding is that **the budget is a function of the output support rather than
  of the available latency**: the best RMS dB error a non-zero budget saves is
  57.19 dB at 129 taps and 23.05 dB at 257 on the eighth-order crossover, then
  nothing from 513 upwards, and at most hundredths of a decibel on every other
  target at every length. A longer filter behind a generous delay line therefore
  wants a *smaller* budget, not a larger one.

  What the construction does deliver is latency. At magnitude accuracy matched to
  within a thousandth of a decibel, a linear-phase filter needs 22 times the latency
  of a 1025-tap zero-budget design on the deep notch, 19 on the parametric EQ, 18 on
  the LR4 crossover, 16 on the low-pass and 8 on the LR8 crossover; on broadband room
  correction no sampled linear-phase latency matches it at all. The price is the
  minimum-phase factor's group-delay deviation, 0.8 to 8.0 samples, which no budget
  reduces.

- `mixedphase.DesignIterativeAuto` — the alternating factorisation with the delay
  budget as an output rather than an input. It minimises RMS dB magnitude error,
  the measure sensitive to the stopband depth the factorisation actually buys,
  subject to linear-magnitude accuracy staying within a configurable multiple of
  the zero-delay design's. Because a zero budget is always evaluated and always
  admissible, the result can never be worse than minimum-phase truncation on that
  objective, which removes the factorisation's main practical hazard in both
  directions: on the five reference targets whose minimum-phase factor fits its
  tap share it selects zero and returns that design bit-for-bit instead of paying
  16 samples for a delayed copy, and on the support-starved eighth-order crossover
  it selects 22 and reaches 3.310 dB against the hand-picked budget's 6.901 dB. It
  also avoids the opposite trap, where a one-sample budget on that target is worse
  than either extreme (77.5% relative error against 1.227% at zero). The search is
  a strided scan plus local refinement, so it costs about 25 designs rather than
  one and is only exact with `CoarseStep: 1`; `Result.Delay` reports what it
  chose. No existing design's output changes.

- Reference suite: `budde-adaptive`, the sixth published method, driving
  `DesignIterativeAuto` under the same tap and grid budget. It selects a zero
  budget on the five targets whose minimum-phase factor fits its tap share — where
  its row is identical to `minphase-truncation` — and 22 samples on
  `steep-crossover`, where it reaches 3.310 dB against `budde-iterative`'s
  6.901 dB. Across the six targets it holds the lowest RMS dB error on all six,
  the only method to lead a column outright, while `budde-iterative` leads none.
  The `phase_delay_samples` column now carries the selected budget for that
  method; every existing row is byte-identical.

- Mixed Phase Lab: the adaptive method, and a "Delay used" metric row reporting
  the budget each design actually applied — the adaptive method's only visible
  output, and "not prescribed" for the low-delay optimiser.

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
