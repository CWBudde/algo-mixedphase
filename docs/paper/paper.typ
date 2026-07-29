#import "style.typ": abstract, code-path, draft-note, paper

#let revision = sys.inputs.at("revision", default: "working tree")

#show: paper.with(
  title: "Fixed-Support Mixed-Phase FIR Filter Design",
  subtitle: "A reproducible revision and comparison of latency–accuracy trade-offs",
  author: "Christian-W. Budde",
  revision: revision,
)

#abstract[
  Linear-phase finite impulse response filters offer constant group delay but
  spend half their support before the impulse-response centre. Minimum-phase
  filters minimise delay but accept frequency-dependent phase. This paper
  revisits an alternating factorisation proposed at DAGA 2012 that divides a
  fixed support between minimum- and linear-phase factors. The revised
  treatment makes the support and stopping rules explicit and places the
  method beside phase interpolation, weighted complex approximation, direct
  low-group-delay optimisation, and a structure-specific hybrid graphic
  equaliser. All implementations and measurements are maintained in a public
  Go repository so that each quantitative claim can be traced to a test or a
  committed comparison artifact.
]

#draft-note[
  This is the first compilable English draft. The method description and
  evidence map follow the implementation; the common benchmark figures,
  result tables, and final conclusions remain open until Phase 3 of the
  repository roadmap is complete.
]

= Introduction

Audio filters rarely have an unconstrained phase response. A linear-phase FIR
filter preserves waveform alignment across frequency, but an odd filter of
length $N$ introduces a delay of $(N - 1) / 2$ samples and distributes its
ringing symmetrically around the main peak. A minimum-phase response
concentrates energy as early as its magnitude response permits, at the cost of
frequency-dependent group delay. In applications with a finite latency budget,
the useful design space lies between these endpoints.

The DAGA 2012 contribution “Gemischtphasige Filter” proposed a practical
factorisation for that space @budde2012. It converts a target to minimum phase,
truncates it, designs a short zero- or linear-phase residual, and alternately
corrects both factors for the spectral error introduced by truncation. The
original two-page paper stated the construction and its intended extensions,
but did not provide a reference implementation or a controlled comparison
against later alternatives.

This document is a revised English treatment, not a historical translation.
Sections that describe the 2012 construction are identified as such. Numerical
conditioning, competing methods, failure cases, and the reproducibility
apparatus are later additions developed in the companion repository.

== Contributions and scope

The completed revision will:

- state the alternating factorisation with an exact total-support budget;
- compare designs under identical targets, weights, tap counts, and delay
  constraints;
- measure the realised impulse response rather than only a design grid;
- expose cases where each method is unreliable; and
- map every reported number and figure to source code and a regeneration
  command.

The `mixedphase` package contains general fixed-length FIR methods. The
`graphiceq` package is reported separately because its hybrid IIR/FIR structure
only applies to octave graphic-equaliser targets.

= Problem statement

Let $M(omega)$ be a non-negative target magnitude sampled for
$omega in [0, pi]$, and let $h[n]$ be a real causal FIR impulse response of
length $N$. A comparison is only meaningful after fixing:

1. the target samples and their frequency weights;
2. the support $N$;
3. the permitted delay or pre-ringing;
4. the error norm; and
5. whether phase is prescribed or selected by the optimiser.

These choices separate two problems that are often conflated. Approximating a
specified complex response can be convex for useful norms. Selecting a phase
that jointly balances magnitude error, delay, and temporal concentration is
generally non-convex.

== Fixed-support factorisation

The 2012 construction represents the final filter as the convolution

$ h[n] = (a ast b)[n], quad N_A + N_B - 1 = N, $

where $a[n]$ is causal and minimum phase and $b[n]$ is a symmetric
linear-phase residual. If the requested delay is $d$ samples, the current
implementation assigns

$ N_B = 2 d + 1, quad N_A = N - N_B + 1. $ <support-split>

Thus $d = 0$ yields the minimum-phase endpoint and
$d = (N - 1) / 2$ yields the linear-phase endpoint without changing the final
tap count.

= Alternating minimum/linear-phase design

The construction below restates the algorithm in the notation of
@budde2012 and the implementation in
#code-path("mixedphase/iterative.go").

1. Transform the prototype and retain its target magnitude $M$.
2. Reconstruct a dense minimum-phase spectrum from $M$.
3. Transform to time, truncate and window it to $N_A$ taps, producing $a$.
4. Divide the target spectrum by the spectrum of $a$ using a regularised
  quotient.
5. Force the residual to zero phase, transform to time, centre it, and
  truncate and window it to $N_B$ taps, producing $b$.
6. Alternately redesign $a$ from the residual of $b$ and $b$ from the
  residual of $a$.
7. Convolve the accepted factors and stop before the first pass that raises
  the realised RMS magnitude error, or when the improvement falls below the
  configured tolerance.

Truncation is not a neutral operation: it convolves the response with the
spectrum of the selected window. The quotient in the next half-pass asks one
factor to compensate for the other factor's truncation error. This alternating
correction is the distinguishing step; direct phase interpolation does not
perform it.

== Minimum-phase reconstruction

The implementation provides two equivalent dense-grid reconstructions:

- a real-cepstrum method that folds the cepstrum onto its causal half; and
- a discrete-Hilbert method that derives phase from log magnitude, following
  Damera-Venkata, Evans, and McCaslin @damera2000.

Both require a positive magnitude floor before taking a logarithm. They differ
numerically because the cepstral route reconstructs magnitude through an
exponential while the Hilbert route retains the floored target magnitude
directly. Finite support and windowing subsequently dominate the error for a
single reconstruction, but repeated quotient updates can amplify their
rounding differences.

== Conditioning and stopping

The alternating update is not a contraction. A factor can contain a deep
spectral null, so even regularised division may amplify small platform
differences. The implementation therefore treats the iteration count as a
maximum budget, evaluates the convolved candidate after every full pass, and
discards the first rising pass.

This stopping rule is deliberately empirical: it returns the first
reproducible local minimum rather than claiming convergence. A negative
stopping tolerance disables both rise detection and convergence checks for
experiments that require an exact pass count. The final paper will report the
pass budget and the number of accepted passes for every alternating-design
result.

= Comparison methods

The repository implements three general comparison paths and one
structure-specific design.

== Prescribed phase

Phase interpolation constructs a target whose unwrapped phase moves between
the minimum- and linear-phase endpoints and then projects its inverse transform
onto the finite support. Weighted complex least squares approximates the same
target directly in coefficient space; Lawson reweighting trades RMS complex
error for a lower peak error.

Zero-weight bins are genuinely unconstrained. They can diverge even when the
weighted band is fitted accurately, so weak weights are safer than removing
bins. The absolute complex objective also tends to prioritise the passband of
a low-pass target unless attenuation is reflected in the weights.

== Phase-free low-group-delay optimisation

Wu, Gao, and Teo formulate FIR design as magnitude-constrained group-delay
optimisation without prescribing the phase @wu2013. The local implementation
uses a penalty ladder and limited-memory BFGS with an analytic gradient. Its
result depends on the starting point because changing between local basins can
require moving a zero across the unit circle. Iteration count and penalty
stages are therefore experimental budgets, not evidence of convergence.

== Structure-specific graphic equalisation

For octave graphic equalisation, Bruschi, Välimäki, Liski, and Cecchi replace
the lowest linear-phase FIR band with an IIR shelving filter and retain the FIR
structure above it @bruschi2022. That approach changes the implementation
structure rather than selecting a general FIR phase. It belongs in the
comparison because it buys latency for an important target class, but it is
not part of the general mixed-phase API.

= Evaluation protocol

The common benchmark suite will evaluate low-pass, parametric-EQ, crossover,
deep-notch, and measured room-correction targets. Each method will receive the
same target samples, frequency weights, tap budget, and applicable delay or
magnitude constraint.

The reported response will always be recomputed from the realised taps. Planned
metrics are:

- RMS and peak magnitude error;
- mean and peak group delay in meaningful magnitude bands;
- peak location, energy centroid, and energy before the peak;
- coefficient range and constraint violation; and
- iteration budget, accepted iterations, and runtime.

Group delay is masked in deep stopbands, where phase is numerically unstable
and perceptually irrelevant. Runtime comparisons will state the machine and
toolchain; the paper build consumes committed benchmark artifacts and never
reruns timing measurements.

#draft-note[
  The Phase 3 CSVs and generated figures will be inserted here. Until those
  artifacts are committed, this section defines the protocol but makes no
  cross-method ranking.
]

= Reproducibility map

#table(
  columns: (1.25fr, 1.2fr, 1.55fr),
  table.header([Paper item], [Implementation], [Evidence or regeneration]),
  [Support split @support-split],
  [#code-path("mixedphase/iterative.go")],
  [#code-path("mixedphase/mixedphase_test.go")],

  [Alternating factorisation],
  [#code-path("mixedphase.DesignIterative")],
  [`go test ./mixedphase`],

  [Minimum-phase reconstruction],
  [#code-path("mixedphase.MinimumPhaseWith")],
  [`go test ./mixedphase`],

  [Prescribed complex response],
  [#code-path("mixedphase.DesignComplexLeastSquares")],
  [`go test ./mixedphase`],

  [Low-group-delay optimisation],
  [#code-path("mixedphase.DesignLowGroupDelay")],
  [`go test ./mixedphase`],

  [Hybrid graphic equaliser],
  [#code-path("graphiceq.Design")],
  [`go test ./graphiceq`],

  [Cross-method CSV], [#code-path("examples/mixedphase")], [`just compare`],
  [Native/WASM agreement],
  [#code-path("scripts/test-cross-build.sh")],
  [`just test-cross-build`],
)

The build embeds the repository revision shown on the title page. The Typst
source, bibliography, generated figure inputs, and build workflow live beside
the implementation; the PDF is a build artifact.

= Limitations and open work

The current draft intentionally leaves three conclusions open. First, the
common benchmark suite is not yet complete, so no method is ranked across
target classes. Second, the original 2012 figures have not been reconstructed
from machine-readable data. Third, perceptual evaluation is limited to
objective pre-ringing and delay proxies; controlled listening tests are outside
the present scope.

= Conclusion

Mixed-phase FIR design is not one optimisation problem but a family of choices
about which phase information to preserve, which error to minimise, and how to
spend finite support. The revised paper makes those choices explicit and binds
its evidence to executable designs. Final conclusions will be added only after
the common benchmark artifacts can reproduce them.

#bibliography("references.bib", style: "ieee", title: "References")
