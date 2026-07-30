# Mixed-phase FIR filter design

This document turns Christian-W. Budde's DAGA 2012 paper,
[“Gemischtphasige Filter”][budde-2012], into an implementation and research
roadmap for this repository. The [`revised English paper`][paper-pdf] presents
the reviewed argument and data-backed figures. The implementation builds on
`algo-dsp` for the underlying DSP primitives and on `algo-fft` for transforms.

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
`TestIterativeDelaySweep` pins every row above; `TestIterativeCrossBuildDeterminism`
pins the delay-8 row in more detail, and `scripts/test-cross-build.sh` runs both it
and `TestIterativeConditioning` in native and `GOOS=js GOARCH=wasm` builds.

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
reproduced while the peak error moves only from 0.580 dB to 0.603 dB, and it keeps
working for several further bands. Against an all-FIR design cut to the same tap
count the hybrid is three to five times more accurate in RMS and roughly two to six
times in peak error (1.85x at one offloaded band, 5.78x at three), which is what
makes the split worth taking.

Two limits are worth stating with it. The shelves step monotonically between
neighbouring band gains, so a target alternating between +12 and -12 dB per
octave breaks the method: the peak error is 3.60 dB for the all-FIR design, 4.00 dB
with one band offloaded and 19.58 dB with two, pinned by `TestZigzagPeakErrorsBySplit`.
Degradation is not monotone in the split — three bands recovers to 10.56 dB — so the
band count is not a quality dial here. And the shelf gains are taken directly from
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
dynamic range; iterations; and constraint violation. Coefficient range is the
peak divided by the smallest coefficient no more than 240 dB below it.
Group-delay bins below `1e-6` absolute magnitude are excluded because phase at
a null is not meaningful.

It records no timing. Wall-clock measurements are machine-dependent, so a
runtime column would make the file differ on every run and destroy its value as
the committed regression golden. Timings live in
[`reference-timings.csv`](reference-timings.csv), which records the machine and
toolchain alongside each measurement and is regenerated only by
`just compare-timings` — never by `just compare`, and never checked for
reproducibility.

### Results

The compact table shows the principal trade-offs; the committed
[`reference-results.csv`](reference-results.csv) contains every metric. Relative
error and pre-peak energy are percentages. Every column here is golden-tested:
`just compare-check` regenerates the table and fails if a single byte moves.

For the cost side of the trade-off see
[`reference-timings.csv`](reference-timings.csv), which reports the fastest of
five complete design calls together with the machine and toolchain that
produced them. Those numbers are a machine-local comparison between methods and
are not reproducible across machines.

<!-- reference-results:start -->

| Target          | Method                   | Rel. error | RMS dB | Mean delay | Delay ripple | Pre-peak |
| :-------------- | :----------------------- | ---------: | -----: | ---------: | -----------: | -------: |
| low-pass        | Budde iterative          |   0.00010% |  0.000 |      21.86 |        1.117 |   17.74% |
|                 | phase interpolation      |   0.22262% |  0.026 |      20.40 |        0.889 |    1.74% |
|                 | complex minimax          |   0.40295% |  0.123 |      20.39 |        0.873 |    1.74% |
|                 | minimum-phase truncation |   0.00000% |  0.000 |       5.86 |        1.117 |   17.74% |
|                 | budde-adaptive           |   0.00000% |  0.000 |       5.86 |        1.117 |   17.74% |
|                 | low group delay          |  22.79885% |  1.898 |       1.76 |        6.676 |   24.37% |
| parametric EQ   | Budde iterative          |   0.07904% |  0.006 |      22.21 |        6.994 |    0.00% |
|                 | phase interpolation      |   1.35975% |  0.100 |      20.94 |        5.835 |    0.16% |
|                 | complex minimax          |   2.58974% |  0.240 |      20.79 |        5.642 |    0.31% |
|                 | minimum-phase truncation |   0.00428% |  0.000 |       6.21 |        6.996 |    0.00% |
|                 | budde-adaptive           |   0.00428% |  0.000 |       6.21 |        6.996 |    0.00% |
|                 | low group delay          |  21.59908% |  1.708 |       1.93 |        7.595 |    0.00% |
| crossover       | Budde iterative          |   0.00001% |  0.001 |      26.41 |        0.766 |   44.77% |
|                 | phase interpolation      |   0.07248% |  3.616 |      23.81 |        0.582 |   39.34% |
|                 | complex minimax          |   0.09716% |  7.194 |      23.81 |        0.580 |   39.35% |
|                 | minimum-phase truncation |   0.00000% |  0.000 |      10.41 |        0.766 |   44.77% |
|                 | budde-adaptive           |   0.00000% |  0.000 |      10.41 |        0.766 |   44.77% |
|                 | low group delay          |   7.11710% |  6.115 |       9.73 |        2.906 |   35.27% |
| deep notch      | Budde iterative          |   0.14710% |  0.739 |      25.19 |        5.812 |    0.01% |
|                 | phase interpolation      |   0.62083% |  3.083 |      22.55 |        4.673 |    0.00% |
|                 | complex minimax          |   0.99023% |  2.547 |      22.51 |        4.787 |    0.02% |
|                 | minimum-phase truncation |   0.00719% |  0.084 |       8.63 |        5.969 |    0.00% |
|                 | budde-adaptive           |   0.00719% |  0.084 |       8.63 |        5.969 |    0.00% |
|                 | low group delay          |   3.53804% |  0.514 |       6.67 |        7.182 |    0.00% |
| room correction | Budde iterative          |   0.13143% |  0.080 |      16.50 |        1.227 |    0.00% |
|                 | phase interpolation      |   0.28140% |  0.117 |      16.38 |        0.984 |    0.06% |
|                 | complex minimax          |   0.70134% |  0.130 |      16.38 |        1.197 |    0.06% |
|                 | minimum-phase truncation |   0.03625% |  0.023 |       0.50 |        1.218 |    0.00% |
|                 | budde-adaptive           |   0.03625% |  0.023 |       0.50 |        1.218 |    0.00% |
|                 | low group delay          |   8.63192% |  0.901 |       0.10 |        2.453 |    0.00% |
| steep crossover | Budde iterative          |   2.50945% |  6.901 |      49.61 |        4.892 |   49.27% |
|                 | phase interpolation      |   1.35880% | 54.483 |      53.04 |        3.235 |   48.11% |
|                 | complex minimax          |   4.35239% | 72.233 |      53.02 |        3.289 |   47.98% |
|                 | minimum-phase truncation |   1.22689% | 54.934 |      49.37 |        4.269 |   49.45% |
|                 | budde-adaptive           |   2.63941% |  3.310 |      52.00 |        4.905 |   47.80% |
|                 | low group delay          |   1.96151% | 42.838 |      49.44 |        4.482 |   49.41% |

<!-- reference-results:end -->

Read this table with the degeneracy check in mind. On the five smooth targets the
minimum-phase factor already fits its share of the taps, so the alternating
correction converges to the identity and `Budde iterative` is a delayed
minimum-phase filter. That is why `minimum-phase truncation` — the same design with
the delay budget removed — matches or beats it on relative error and RMS dB on every
one of them, at a third to a twentieth of the delay. Those five rows measure the
minimum-phase reconstruction, not the factorisation.

The published impulse responses show this directly. On the `crossover` target the
two designs' peak-aligned coefficients agree to `3.0e-07` over all 113 shared
samples, and their peak indices differ by exactly the 16-sample budget — they are
the same filter, one delayed. On `steep-crossover` the same comparison differs by
`4.3e-02`, five orders of magnitude more. Note also that both targets give the two
designs the _same_ pre-peak energy ratio to four decimals, so a high pre-peak
figure is not on its own evidence of a mixed-phase result.

`steep-crossover` is the row that measures the method. An eighth-order crossover at
800 Hz does not fit the budget, the linear factor carries 92.4% of its energy off
centre, and the correction loop accepts five passes. There the alternating
factorisation reaches 6.901 dB RMS magnitude error against 54.483 dB for phase
interpolation, 54.934 dB for minimum-phase truncation and 42.838 dB for the
low-delay optimiser, at a mean group delay of 49.61 samples — _lower_ than phase
interpolation's 53.04, so on this target it wins on accuracy and delay at once.
It buys that with
linear-magnitude accuracy (2.509% against 1.227%) and with the worst group-delay
ripple of the fixed-delay methods. `TestSteepTargetActuallyExercisesTheFactorisation`
guards both halves of this, so the suite cannot silently drift back to measuring
only the degenerate case.

`budde-adaptive` is the same construction with the delay budget chosen rather than
supplied: it minimises RMS dB error over a candidate set of budgets that always
contains zero, subject to relative error staying within three times the zero-delay
value or under an absolute floor of 1e-2, and to a non-zero budget saving at least
1 dB against the zero-delay design. Read its rows against `Budde iterative`. On all five smooth targets it selects
zero and its row is therefore identical to `minimum-phase truncation` — the delay
that bought nothing is simply not spent, and the 16.5-to-26.4-sample mean delays of
the fixed budget become 0.5 to 10.4. On `steep-crossover` it selects 22 and reaches
3.310 dB against the fixed budget's 6.901 dB. Across the six targets it holds the
lowest RMS dB error on all six, the only method here to lead a column outright, and
the lowest relative error on five; it concedes relative error only on
`steep-crossover`, where it spends the slack the constraint allows (2.639% against
1.227%). Measured against `minimum-phase truncation` on that target, the trade is
70.8 dB more stopband rejection over the bins where the target is already below
−80 dB, for 2.63 samples of mean group delay.

Both of those extra bounds earn their place. Without the absolute floor, a purely
multiplicative passband ceiling rejects designs that are far better on the
objective: designing `steep-crossover` into 193 taps, the three-times ceiling
declines an 11-sample budget worth 16 dB in exchange for a passband regression
under a hundredth of a decibel. Without the 1 dB margin, the search buys latency
for nothing measurable — a 129-tap low-pass improves from 0.800 dB to 0.498 dB at a
one-sample budget, and taking it abandons the exact minimum-phase design for three
tenths of a decibel. `TestAdaptiveGuardDoesNotRejectAMuchBetterDesign` and
`TestDesignIterativeAutoDeclinesBudgetsThatBarelyHelp` pin the two.

Two cautions on that method. The candidate set is a strided scan with local
refinement, not every admissible budget, so the selection is a heuristic — one
fixture in the `mixedphase` tests is 0.76 dB short of the exhaustive result. And a
hand-picked budget is hazardous in the other direction too: on `steep-crossover` a
one-sample budget reaches 77.5% relative error, far worse than either endpoint,
because a three-tap linear factor cannot approximate the residual quotient at all.
`TestSmallDelayBudgetsAreTheWorstChoice` pins that, and it is the reason the budget
should be selected rather than defaulted.

The delay-ripple column is the one axis on which the alternating factorisation is
consistently last among the fixed-delay methods: it inherits the minimum-phase
factor's ripple almost exactly wherever the correction is inert.

Lawson minimax controls peak _complex_ error, not these magnitude-only metrics, and
its reweighting is multiplicative, so after sixteen passes the supplied
inverse-magnitude weight has almost no influence left. The weight is worth having on
the plain least-squares solution — 3.616 dB to 0.568 dB on the crossover — and
almost nothing after a full minimax run. The direct low-delay optimiser wins mean
delay on five of the six targets and keeps its 2 dB constraint, but its relative
magnitude error grows accordingly; on `steep-crossover` minimum-phase truncation is
marginally lower (49.37 against 49.44 samples), because that target's own phase
response already forces most of the delay; that tolerance is a deliberate dial, not a converged
result. Its deep-notch peak delay is still large: mean delay near a spectral null
must not be read as a peak-delay guarantee.

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
[paper-pdf]: https://github.com/cwbudde/algo-mixedphase/releases/latest/download/mixed-phase-filter-design-en.pdf
[potchinkov-1995]: https://doi.org/10.1016/0165-1684(95)00077-Q
[wu-2013]: https://doi.org/10.1016/j.sigpro.2013.01.015
[yan-ma-2004]: https://doi.org/10.1016/j.dsp.2004.08.003

## What the delay budget is, and is not, for

Everything above is measured at one operating point: 129 output taps from 257-tap
prototypes. `docs/reference-delay-sweep.csv` covers the rest of the space — the same
six curves rebuilt as 2049-tap fixtures, designed at 129, 257, 513 and 1025 output
taps with the budget strided over the admissible range, plus a linear-phase family
for comparison at equal latency. It is a separate artifact because a 257-tap
prototype is shorter than a 513-tap filter, so at the published fixture length every
method reproduces the target exactly and the comparison measures rounding. Each
sweep row carries the prototype length it used; the two artifacts must not be read
against each other.

Two results come out of it, and both narrow what the budget should be used for.

**Start from the floor, not from the latency allowance.** A magnitude request
already carries a group delay: every causal realisation of a magnitude differs from
the minimum-phase one by an all-pass factor, and all-pass group delay is
non-negative, so nothing realising that magnitude is faster than minimum phase. At
129 taps the six targets sit at 0.50, 5.86, 6.21, 8.63, 10.41 and 49.37 samples
(room correction, low-pass, parametric EQ, deep notch, LR4, LR8). A budget is what
is left of an application's latency allowance _after_ the floor has taken its
share — which for the LR8 crossover is nothing at all.
`TestZeroDelayDesignSitsOnTheMinimumPhaseFloor` establishes that these floors
belong to the targets rather than to the split.

**This construction spends its surplus as pure delay.** The linear-phase factor is
symmetric, so it contributes exactly linear phase, and the group-delay deviation of
the cascade equals that of its minimum-phase factor for every admissible budget.
Across all six targets the measured ripple is identical to nine decimal places at
every budget, whether or not the linear factor carries energy away from its centre
tap: the all-pass the budget inserts is `z^-d`, which translates the group-delay
curve and does not flatten it. `TestAdaptiveDelayBudgetCannotFlattenGroupDelay`
pins the identity.

That is a property of the construction, not of the latency. `DesignPhaseInterpolation`
spends the same surplus on flatness: at 44.9 samples on the low-pass the
factorisation still carries its full 1.117 samples of ripple where the prescribed
continuum is at 0.419, and on the LR4 crossover 0.766 against 0.287. So a caller
who wants flat group delay should prescribe the phase rather than raise this
budget. `TestFactorisationHoldsItsRippleWhileTheContinuumDescends` pins both sides,
and `docs/reference-phase-regimes.csv` carries the curves.

**Below the floor only the magnitude can give way.** No phase choice reaches a
latency under `tau_min`. Widening `DesignLowGroupDelay`'s `ToleranceDB` from 0.25 to
2 dB buys delay below the floor on five of six targets — room correction conceding
the largest share of its floor at 80%, and the parametric EQ the most samples at
4.28, for at most 1.90 dB of RMS magnitude error — while the LR8
crossover buys nothing, having no accuracy left to concede at 129 taps. Beyond 2 dB
the measurement stops being meaningful: at 4 dB the optimiser tracks its iteration
budget rather than its tolerance, and at 8 dB the constraint admits a spectral null
where group delay is undefined.

**The budget is a function of the output support, not of the available latency.**
The best RMS dB error any non-zero budget saves against the zero-budget design:

| target          | 129 taps | 257 taps | 513 taps | 1025 taps |
| --------------- | -------: | -------: | -------: | --------: |
| steep crossover | 57.19 dB | 23.05 dB |  0.00 dB |   0.00 dB |
| room correction |  0.00 dB |  0.04 dB |  0.03 dB |   0.00 dB |
| the other four  |  0.00 dB |  0.00 dB |  0.00 dB |   0.00 dB |

One target benefits materially, and only below about 513 taps. The minimum-phase
supports explain the ordering — 52 taps for the LR4 crossover, 53 for the low-pass,
116 for the parametric EQ, 129 for the deep notch, 238 for the LR8 crossover and 995
for room correction, taken as the leading taps holding all but 1e-6 of the factor's
energy. Once the output length comfortably exceeds that support the factor fits at
any admissible budget and the correction has nothing to recover. So a longer filter
behind a generous delay line wants a **smaller** budget, not a larger one.

What the construction does deliver is latency. A linear-phase filter of latency L
has only 2L+1 taps to spend on the magnitude, while a minimum-phase-led design of N
taps spends all N. Matching a 1025-tap zero-budget design's accuracy to within a
thousandth of a decibel costs a linear-phase filter 22 times the latency on the deep
notch, 19 on the parametric EQ, 18 on the LR4 crossover, 16 on the low-pass and 8 on
the LR8 crossover; on room correction no sampled linear-phase latency matches it at
all, the design sitting at 0.046 dB and 0.6 samples against the linear-phase family's
best of 0.250 dB at 512 samples. The price is the minimum-phase factor's group-delay
deviation, 0.8 to 8.0 samples here, which no budget reduces.
`TestSweepLinearPhaseNeedsFarMoreLatency` and
`TestSweepBudgetGainTableMatchesTheDocumentation` pin these figures; the
linear-phase family is sampled every 32 samples of latency, so each factor is
accurate to within one stride.
