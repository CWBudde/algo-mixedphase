# Mixed-phase FIR filter design

This document turns Christian-W. Budde's DAGA 2012 paper,
[“Gemischtphasige Filter”][budde-2012], into an implementation and research
roadmap for this repository. It builds on `algo-dsp` for the underlying DSP
primitives and on `algo-fft` for transforms.

## The actual design problem

“Mixed phase” is not one specification. A useful comparison must hold these
quantities fixed:

1. the target magnitude response and its frequency weighting;
2. the total number of FIR taps;
3. the permitted delay or pre-ringing;
4. the error norm (linear magnitude, dB magnitude, complex response, or
   equiripple); and
5. whether phase is prescribed or may be chosen by the designer.

This distinction matters. Approximating a prescribed _complex_ response with
FIR coefficients is a convex problem for several useful norms. Choosing the
phase that best balances magnitude accuracy, delay, and temporal compactness is
generally non-convex.

## The DAGA 2012 method

The paper starts with a target response and a fixed total tap budget. It:

1. reconstructs the target as a minimum-phase response;
2. truncates that response to a minimum-phase factor of length `LA`;
3. divides the target spectrum by the truncated factor;
4. forces the residual to zero/linear phase and truncates it to length `LB`;
5. alternately recomputes each factor from the target divided by the other
   factor; and
6. stops when the convolved factors no longer improve.

For the minimum-plus-linear form implemented here,

```text
LB = 2 * delay + 1
LA = totalLength - LB + 1
len(convolve(A, B)) = totalLength
```

The iteration is the important part. A direct interpolation between minimum and
linear phase generally creates a response with energy outside the available
causal support. Truncation then changes the magnitude response. Alternating the
two residual designs makes each factor compensate for the other factor's
truncation error.

The first implementation is in `mixedphase`:

- `MinimumPhase`: real-cepstrum minimum-phase reconstruction;
- `DesignIterative`: the alternating two-factor method;
- `DesignPhaseInterpolation`: a deliberately simple direct baseline;
- `DesignComplexLeastSquares`: weighted complex approximation with an optional
  Lawson minimax path;
- `DesignLowGroupDelay`: direct magnitude-constrained delay optimisation; and
- `Analyze`: common frequency- and time-domain metrics.

`examples/mixedphase` runs the fixed multi-target reference suite described
below and emits the committed `docs/reference-results.csv`.

### Conditioning and stopping the alternating loop

The correction loop is not a contraction. On the 129-tap, 0.08-cutoff
low-pass at delay 8, with the default 2048-point grid and scale-relative
`Epsilon` (approximately `1e-12`), native RMS magnitude error over passes one
through six is 5.366154, 4.609232, 4.917232, 4.686326, 4.852711, and
4.605098 dB. The updates oscillate before the truncated-factor division becomes
ill-conditioned: at pass twelve the native result is 10.739658 dB and the
JavaScript/WASM result is 15.829906 dB.

Raising `Epsilon` to `1e-6` bounds the pass-twelve result to 4.767566 dB
native and 4.855305 dB in WASM, but does not make the updates monotone or
platform-independent. The oscillation is therefore inherent to alternating
truncated projections; the late catastrophic growth is caused by dividing
through factor nulls with too little regularisation.

`DesignIterative` now accepts the first pass unconditionally and stops before
the first subsequent rise, returning the previous factors. Pass two is the
first reproducible local minimum for the delay-8 case: native and WASM produce
4.609232 dB and agree on the checked coefficients within `1e-10`. Continuing
through a rise reaches a slightly lower pass-six value, but that path is already
sensitive to platform rounding and is not a reproducible stopping rule.

There is a metric trade-off even before correction: the uncorrected
factorisation has a lower RMS dB error of 3.716940 dB, but a much larger
relative linear-magnitude error of 0.021794, against 0.005426 after the two
accepted passes. The initial factorisation is therefore not included when
detecting a rise between complete correction passes.

The delay-sweep regression uses a maximum budget of 12 passes and the default
`1e-7` dB stopping tolerance:

| Delay | Accepted passes | Relative magnitude error | RMS error |
| ----: | --------------: | -----------------------: | --------: |
|     0 |               0 |              0.000096348 |  0.800068 |
|     8 |               2 |              0.005426207 |  4.609232 |
|    16 |               1 |              0.013427020 |  7.469879 |
|    32 |               1 |              0.004819747 |  8.623192 |
|    64 |               0 |              0.001575002 | 21.639157 |

The zero- and maximum-delay endpoints need no alternating correction.
`TestIterativeCrossBuildDeterminism` pins these metrics and
`scripts/test-cross-build.sh` runs the delay-8 golden case in both native and
`GOOS=js GOARCH=wasm` builds.

## Established alternatives

### More accurate minimum-phase reconstruction

The real cepstrum is convenient and maps well onto `algo-fft`, but its result
depends on FFT oversampling, the log-magnitude floor, and truncation.
Damera-Venkata, Evans, and McCaslin describe a non-iterative discrete Hilbert
transform method intended to avoid systematic factorisation error
([IEEE TSP 2000][damera-venkata-2000]). Olivier revisited finite-precision
factorability and reported that common methods can leave considerably more
residual error than necessary ([IET Signal Processing 2022][olivier-2022]).

This is the best next replacement for the current cepstral primitive. It
improves every design method that depends on minimum-phase conversion without
changing the public mixed-phase API.

### Prescribed magnitude and phase: complex-response optimisation

When the desired phase or group-delay curve is supplied, direct coefficient
optimisation is preferable to factorising and windowing:

- Potchinkov and Reemtsen formulate complex Chebyshev FIR design as convex
  semi-infinite optimisation ([Signal Processing 1995][potchinkov-1995]).
- Yan and Ma give a second-order-cone framework for arbitrary magnitude and
  phase with L1, L2, and L-infinity objectives
  ([Digital Signal Processing 2004][yan-ma-2004]).
- Lee, Caccetta, Teo, and Rehbock directly address arbitrary magnitude and
  group delay, including equiripple and peak-constrained least-squares designs
  ([IEEE TSP 2006][lee-2006]).
- Lai combines constrained least-squares and Chebyshev designs with explicit
  phase-error bounds ([IEEE TSP 2009][lai-2009]).

These methods can outperform the DAGA iteration when a defensible desired
phase/group-delay curve exists. They solve a somewhat different problem:
Budde's method derives a compact phase distribution from a latency budget
without requiring the entire phase curve as input.

`DesignComplexLeastSquares` implements the first useful version of this route in
pure Go: the weighted normal equations are Toeplitz on a uniform DFT grid, so
both the autocorrelation and the cross-correlation reduce to a single inverse
transform each, and a dense Cholesky factorisation solves the system. Lawson
reweighting then drives the sequence towards the complex Chebyshev solution
without a conic solver. Two properties are worth knowing before using it:

- With uniform weights the solution is identical to `DesignPhaseInterpolation`,
  because truncating the inverse transform already minimises the unweighted
  mean-square complex error. The weight is the entire point.
- The objective measures absolute complex deviation, so an unweighted minimax
  design concentrates on the passband and lets stopband depth slip. Weight
  inversely to the target magnitude when attenuation matters.

### Phase free, delay constrained: direct non-convex optimisation

Wu, Gao, and Teo minimise group delay while constraining magnitude response,
without first prescribing a phase curve. Their formulation exposes the direct
trade-off between delay and magnitude error and permits group-delay
constraints. Their examples report lower delay than a convex magnitude design
followed by minimum-phase spectral factorisation
([Signal Processing 2013][wu-2013]).

This is the closest published alternative to the design goal in the DAGA
paper. It needs a reliable constrained optimiser and is more complex than the
current alternating FFT method.

`DesignLowGroupDelay` implements this formulation: it minimises the
magnitude-weighted passband group delay subject to a per-bin magnitude
tolerance, using a penalty ladder around a limited-memory BFGS minimiser with
an analytic gradient. The gradient is checked against finite differences in the
test suite, because every claim below rests on it.

Three measured properties, all on the 65-tap low-pass of the example harness:

- It beats a truncated minimum-phase design, but not by much. Mean passband
  group delay falls from 12.70 to 12.62 samples within a 1 dB tolerance and to
  12.08 within 6 dB; peak passband delay falls from 23.58 to 22.72 and 21.18.
  Minimum phase is already nearly delay-optimal for a fixed magnitude, so the
  available gain is essentially the room the tolerance grants.
- Initialisation decides the outcome. From the linear-phase prototype the same
  1 dB design settles at a mean delay near 31.5 samples instead of 12.6, and
  that point is a true local minimum — it improves on its own start and cannot
  be pushed further. Escaping would require moving a zero across the unit
  circle, which no descent direction does.
- Convergence is slow rather than sharp. At 6 dB the mean delay is 12.35 after
  50 iterations per penalty stage, 12.08 after 200 and 10.79 after 800, while
  the relative magnitude error grows from 1.2e-2 to 2.5e-2 to 1.4e-1. The
  iteration budget acts as a second delay-versus-accuracy control, so any
  reported number must name the budget that produced it.

The design is also markedly more expensive than the transform-based methods:
each objective evaluation is O(bins × taps) and there are hundreds per penalty
stage.

### Structure-specific low-latency filters

For an octave graphic equaliser, Bruschi, Välimäki, Liski, and Cecchi replace
the lowest linear-phase FIR band with an IIR shelving filter and retain the FIR
structure for the remaining bands. They report a 50% latency reduction relative
to their all-linear-phase design ([DAFx 2022][bruschi-2022]).

This is attractive when the target is specifically a graphic equaliser. It is
not a general arbitrary-response mixed-phase designer, so it belongs in a
separate comparison track rather than in the core API.

The `graphiceq` package is that track. It generalises the paper's single
shelving band: the lowest `IIRBands` bands become a cascade of low shelves, each
placed at the geometric mean of two neighbouring centres and carrying their gain
difference, and one linear-phase FIR realises the rest. Because octave centres
double, every band moved into the cascade halves the FIR length the remaining
response needs, so the latency halves with it.

Measured on ten octave bands from 31.25 Hz at 48 kHz with the gains
6, -3, 0, 4, -6, 2, 0, -2, 5, 0 dB:

| IIR bands | taps | latency | RMS error | peak error |
| --------- | ---- | ------- | --------- | ---------- |
| 0         | 3073 | 1536    | 0.014 dB  | 0.580 dB   |
| 1         | 1537 | 768     | 0.016 dB  | 0.603 dB   |
| 2         | 769  | 384     | 0.024 dB  | 0.478 dB   |
| 3         | 385  | 192     | 0.048 dB  | 0.774 dB   |
| 4         | 193  | 96      | 0.068 dB  | 0.914 dB   |

The first row is the all-FIR reference, so the paper's 50% latency reduction is
reproduced at an unchanged peak error, and it keeps working for several further
bands. Against an all-FIR design cut to the same tap count the hybrid is three
to five times more accurate in RMS and two to five times in peak error
throughout, which is what makes the split worth taking.

Two limits are worth stating with it. The shelves step monotonically between
neighbouring band gains, so a target alternating between +12 and -12 dB per
octave breaks the method: the peak error is 3.6 dB for the all-FIR design and
19.6 dB with two bands offloaded. And the shelf gains are taken directly from
the requested gain differences without an interaction solve, which is adequate
for the few bands measured here and would not be for a deeper split.

## What appears genuinely useful to implement

Recommended order:

1. **Current baseline:** validate the DAGA iteration and direct phase
   interpolation against MATLAB/NumPy reference vectors.
2. **DHT minimum phase:** replace or complement the cepstral reconstruction;
   compare factorisation error and runtime.
3. **Prescribed complex response:** add weighted least-squares first, followed
   by an IRLS/minimax path. This is practical in pure Go and does not require a
   general conic solver for the first useful version.
4. **Direct delay optimisation:** reproduce the Wu–Gao–Teo low-group-delay
   experiments with the same magnitude constraints used by the other methods.
5. **Optional audio structures:** compare a hybrid IIR/FIR equaliser only for
   targets where that structure is applicable.

Automatic differentiation or a generic nonlinear optimiser can make step 4
easier to implement, but using one is an engineering choice rather than a new
filter-design principle.

## Common reference suite

`internal/reference` is the one harness used for the general mixed-phase
methods. `graphiceq` stays separate because an octave-band shelf/FIR structure
cannot represent arbitrary low-pass, crossover, or notch targets. Every
reference row uses 48 kHz, 129 output taps, a 1024-point design/analysis grid,
the same target samples, and target-magnitude-squared group-delay weights over
the relevant band.

The three phase-controlled methods receive a 16-sample linear-phase allocation.
`DesignIterative` may accept at most 12 correction passes;
`DesignComplexLeastSquares` may run 16 Lawson passes. The low-group-delay
method chooses its own phase, so the phase-delay column is empty for it; its
fixed budget is a 2 dB magnitude tolerance and four penalty stages of at most
80 L-BFGS steps each. The room result starts from the default minimum-phase
initialisation, as do the other low-delay rows.

The five 257-tap reference prototypes are:

1. a first-order 1 kHz low-pass;
2. a +9 dB, 3 kHz parametric-EQ bell with a 0.18-octave Gaussian width;
3. a fourth-order Linkwitz–Riley 2 kHz low-pass crossover branch;
4. a -60 dB, 6 kHz notch with a 0.10-octave Gaussian width; and
5. a room-correction curve obtained by inverting and capping at ±12 dB a
   one-third-octave reduction of the measured OpenAIR
   [`r8-omni-conf_b.wav`][openair-room] studio response.

The CSV records relative linear-magnitude error; RMS and maximum dB error at
the `Analyze` -120 dB floor; weighted group-delay mean, RMS ripple, and peak;
peak index, energy centroid, and energy before the peak; coefficient peak and
dynamic range; iterations; constraint violation; and runtime. Coefficient
range is the peak divided by the smallest coefficient no more than 240 dB
below it. Group-delay bins below `1e-6` absolute magnitude are excluded because
phase at a null is not meaningful.

### Results

The compact table shows the principal trade-offs; the committed
[`reference-results.csv`](reference-results.csv) contains every metric. Relative
error and pre-peak energy are percentages. Runtime is the fastest of three
complete design calls on Go 1.26.1/linux-amd64 on a 12th-generation Intel
Core i7-1255U; it is a machine-local comparison, while all quality columns are
golden-tested independently of runtime.

<!-- reference-results:start -->

| Target          | Method              | Rel. error | Mean delay | Pre-peak |   Runtime |
| :-------------- | :------------------ | ---------: | ---------: | -------: | --------: |
| low-pass        | Budde iterative     |   0.00010% |      21.86 |   17.74% |   0.72 ms |
|                 | phase interpolation |   0.22262% |      20.40 |    1.74% |   0.28 ms |
|                 | complex minimax     |   0.40294% |      20.39 |    1.74% |   5.94 ms |
|                 | low group delay     |  22.79885% |       1.76 |   24.37% |  81.63 ms |
| parametric EQ   | Budde iterative     |   0.07904% |      22.21 |    0.00% |   0.53 ms |
|                 | phase interpolation |   1.35975% |      20.94 |    0.16% |   0.16 ms |
|                 | complex minimax     |   2.58983% |      20.79 |    0.31% |   5.67 ms |
|                 | low group delay     |  21.59908% |       1.93 |    0.00% | 100.55 ms |
| crossover       | Budde iterative     |   0.00001% |      26.41 |   44.77% |   0.92 ms |
|                 | phase interpolation |   0.07248% |      23.81 |   39.34% |   0.28 ms |
|                 | complex minimax     |   0.09716% |      23.81 |   39.35% |   6.77 ms |
|                 | low group delay     |   7.11710% |       9.73 |   35.27% | 143.78 ms |
| deep notch      | Budde iterative     |   0.14710% |      25.19 |    0.01% |   1.72 ms |
|                 | phase interpolation |   0.62083% |      22.55 |    0.00% |   0.20 ms |
|                 | complex minimax     |   1.03374% |      22.50 |    0.02% |   8.19 ms |
|                 | low group delay     |   3.53804% |       6.67 |    0.00% | 191.20 ms |
| room correction | Budde iterative     |   0.13143% |      16.50 |    0.00% |   2.27 ms |
|                 | phase interpolation |   0.28140% |      16.38 |    0.06% |   0.18 ms |
|                 | complex minimax     |   0.67989% |      16.38 |    0.06% |   7.07 ms |
|                 | low group delay     |   8.63192% |       0.10 |    0.00% |  89.70 ms |

<!-- reference-results:end -->

The alternating factorisation wins linear-magnitude accuracy on all five
targets, often by more than an order of magnitude. That is not a universal
win: it pays with more delay, reaches 44.77% pre-peak energy on the crossover,
and needs up to 238 dB of coefficient range there. Phase interpolation is the
speed winner and generally reduces pre-ringing, at the price of larger
magnitude error.

Lawson minimax controls peak _complex_ error, not these magnitude-only metrics.
It helps the deep notch's RMS/maximum dB errors relative to phase interpolation
(2.55/21.14 dB against 3.08/24.94 dB), but otherwise adds runtime without
winning a magnitude, delay, or energy column. The direct low-delay optimiser
wins mean delay on every target and keeps its 2 dB constraint to within
`8.6e-4`, but its relative magnitude error grows to 3.54–22.80%. Its deep-notch
peak delay is still 26.61 samples: mean delay near a spectral null must not be
read as a peak-delay guarantee.

Run `just compare` to regenerate both committed CSVs. The reference-package
test compares every non-runtime cell with the committed artifact, so metric
changes require an explicit regeneration and review.

## Demo packaging

The design code compiles to WebAssembly, so the live demo is a “Mixed Phase Lab”
page under `web/`, deployed to GitHub Pages:

- one delay/pre-ringing control;
- an impulse plot with the latency budget marked;
- magnitude and group-delay overlays;
- a method selector; and
- an A/B audio impulse or short transient.

This originally read “a separate repository becomes worthwhile only if the lab
grows into an independent research application”. That is what happened: the work
outgrew a phase of the `algo-dsp` roadmap once it acquired a paper, a comparison
harness and a lab of its own, and it now lives here. `algo-dsp` stayed the
upstream library and is depended on, not forked — only the public API is used.

## Novelty assessment

The broad idea of nonlinear-/mixed-phase FIR design was already well covered by
the optimisation literature before 2012. The distinctive part of the DAGA
paper is the practical alternating factorisation under a fixed combined support
with a directly understandable latency split. This search did not find an
obvious publication with exactly that construction, but it is a technical
literature search, not a patent or formal novelty search.

[budde-2012]: https://pub.dega-akustik.de/DAGA_2012/data/articles/000281.pdf
[bruschi-2022]: https://dafx.de/paper-archive/2022/papers/DAFx20in22_paper_32.pdf
[damera-venkata-2000]: https://users.ece.utexas.edu/~bevans/papers/2000/minPhase/minPhase.pdf
[lai-2009]: https://doi.org/10.1109/TSP.2009.2021639
[lee-2006]: https://doi.org/10.1109/TSP.2006.872542
[olivier-2022]: https://doi.org/10.1049/sil2.12166
[openair-room]: https://github.com/Mu-Y/RoomIR-equalizer/blob/master/r8-omni-conf_b.wav
[potchinkov-1995]: https://doi.org/10.1016/0165-1684(95)00077-Q
[wu-2013]: https://doi.org/10.1016/j.sigpro.2013.01.015
[yan-ma-2004]: https://doi.org/10.1016/j.dsp.2004.08.003
