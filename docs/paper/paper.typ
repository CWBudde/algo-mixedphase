#import "charts.typ": accuracy-delay-chart, graphiceq-chart, pre-ringing-chart
#import "style.typ": code-path, draft-note, paper

#let revision = sys.inputs.at("revision", default: "working tree")
#let reference-results = csv("../reference-results.csv", row-type: dictionary)
#let graphiceq-results = csv("../graphiceq-results.csv", row-type: dictionary)

#show: paper.with(
  title: "Fixed-Support Mixed-Phase FIR Filter Design",
  subtitle: "A reproducible revision and comparison of latency–accuracy trade-offs",
  author: "Christian-W. Budde",
  revision: revision,
  abstract-body: [
    Linear-phase finite impulse response filters offer constant group delay but
    spend half their support before the impulse-response centre. Minimum-phase
    filters minimise delay but accept frequency-dependent phase. This paper
    revisits an alternating factorisation proposed at DAGA 2012 that combines
    minimum- and linear-phase factors. It distinguishes that historical
    construction from the exact support convention, numerical safeguards,
    stopping rule, comparison methods, and evaluation protocol introduced by
    the present revision. The later comparison covers phase interpolation,
    weighted complex approximation, direct low-group-delay optimisation, and a
    structure-specific hybrid graphic equaliser. All implementations and
    measurements are maintained in a public Go repository so that each
    quantitative claim can be traced to a test or a committed comparison
    artifact.
  ],
  status-body: [
    This is a working English revision. The boundary between the 2012
    contribution and later repository work has been audited against the
    original paper. Its charts are generated from committed reference CSVs;
    the full cross-target argument and final conclusions remain open.
  ],
)

= Introduction

Audio filters rarely have an unconstrained phase response. A linear-phase FIR
filter preserves waveform alignment across frequency, but an odd filter of
length $N$ introduces a delay of $(N - 1) / 2$ samples and distributes its
ringing symmetrically around the main peak. A minimum-phase response
concentrates energy as early as its magnitude response permits, at the cost of
frequency-dependent group delay. In applications with a finite latency budget,
the useful design space lies between these endpoints.

The DAGA 2012 contribution “Gemischtphasige Filter” proposed a practical
factorisation for that space @budde2012. Its developed construction converts a
target to minimum phase, truncates it, designs a short zero- or linear-phase
residual, and alternately corrects the two factors for the spectral error
introduced by windowing. It also sketches minimum/maximum-phase and
frequency-dependent extensions. The original two-page paper supplies neither a
reference implementation nor a controlled numerical comparison.

This document is a revised English treatment, not a historical translation.
Historical claims are limited to what the 2012 paper states. Exact
fixed-support accounting, numerical conditioning, deterministic stopping,
competing methods, failure cases, and the reproducibility apparatus are later
additions developed in the companion repository.

== Historical scope and revision boundary

The 2012 contribution consists of four ideas:

- treat the available pre-ringing time as a design budget between the
  minimum- and linear-phase endpoints;
- realise the mixed-phase response as a cascade of short minimum- and
  linear-phase FIR factors;
- form each factor from the target divided by the other factor, so it can
  compensate the other's windowing error; and
- repeat that residual design alternately, using response error as a possible
  stopping criterion.

The original paper describes a minimum/maximum-phase split, frequency-dependent
phase weighting, and a three-factor decomposition only as possible extensions.
They are not developed algorithms or evaluated results. Likewise, “nearly
optimal” impulse-response length is a design objective in the original text,
not a proved optimum or a claim supported by a benchmark.

This revision adds the exact output-support convention, regularised spectral
division, concrete minimum-phase reconstructions and windows, a reproducible
stopping policy, common metrics, competing methods, and deterministic benchmark
artifacts. Those additions must not be read back as results of the 2012 paper.
The revised treatment:

- states the alternating factorisation with an exact total-support budget;
- compares designs under identical targets, weights, tap counts, and delay
  constraints;
- measures the realised impulse response rather than only a design grid;
- exposes cases where each method is unreliable; and
- maps every reported number and figure to source code and a regeneration
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

== Fixed-support convention of this revision

The 2012 paper requires the two factor lengths to fit within a total budget and
describes the total length informally as their sum. For ordinary finite
convolution, however, lengths $N_A$ and $N_B$ produce
$N_A + N_B - 1$ samples. The present revision removes that off-by-one ambiguity
by defining

$ h[n] = (a ast b)[n], quad N_A + N_B - 1 = N, $

where $a[n]$ is causal and minimum phase and $b[n]$ is a symmetric
linear-phase residual. If the requested pre-ringing budget is $d$ samples, the
repository implementation assigns

$ N_B = 2 d + 1, quad N_A = N - N_B + 1. $ <support-split>

Thus $d = 0$ yields the minimum-phase endpoint and
$d = (N - 1) / 2$ yields the linear-phase endpoint without changing the final
tap count.

= The 2012 alternating construction

The following is a faithful English restatement of the algorithm in
@budde2012, with notation normalised to the present paper:

1. Transform the prototype and retain its target magnitude $M$.
2. Reconstruct a dense minimum-phase spectrum from $M$.
3. Transform to time, truncate and window it to $N_A$ taps, producing $a$.
4. Divide the target spectrum by the spectrum of $a$ to obtain the residual.
5. Force the residual to zero phase, transform to time, centre it, and
  truncate and window it to $N_B$ taps, producing $b$.
6. Convolve the two factors to obtain the mixed-phase response.
7. Return to the residual step, alternately dividing by $b$ and $a$, so each
  factor compensates the windowing influence of the other.

Truncation is not a neutral operation: it convolves the response with the
spectrum of the selected window. The quotient in the next half-pass asks one
factor to compensate for the other factor's truncation error. This alternating
correction is the distinguishing step; direct phase interpolation does not
perform it. The 2012 text proposes response difference as one possible stopping
criterion and suggests varying window parameters during iteration, but fixes
neither a numerical rule nor a particular window.

== Repository realisation

The executable version is #code-path("mixedphase.DesignIterative"). It adopts
the exact support split in @support-split, uses a scale-relative floor in every
spectral quotient, applies deterministic windows, evaluates the realised
convolution after each complete alternating pass, and returns the last accepted
pair of factors. These choices make the historical construction testable; they
are not claims about the unspecified implementation used in 2012.

=== Minimum-phase reconstruction

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

=== Conditioning and stopping

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

= Post-2012 comparison methods

None of the methods in this section was evaluated in the 2012 contribution.
The repository adds three general comparison paths and one structure-specific
design to test the historical construction against alternatives under common
budgets.

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

= Repository evaluation protocol

The 2012 paper contains illustrative response and signal-flow figures but no
machine-readable result set. The present revision's common benchmark suite
evaluates low-pass, parametric-EQ, crossover, deep-notch, and measured
room-correction targets. Each method receives the same target samples,
frequency weights, tap budget, and applicable delay or magnitude constraint.

The reported response is always recomputed from the realised taps. The suite
records:

- RMS and peak magnitude error;
- mean and peak group delay in meaningful magnitude bands;
- peak location, energy centroid, and energy before the peak;
- coefficient range and constraint violation; and
- iteration budget, accepted iterations, and runtime.

Group delay is masked in deep stopbands, where phase is numerically unstable
and perceptually irrelevant. Runtime comparisons will state the machine and
toolchain; the paper build consumes committed benchmark artifacts and never
reruns timing measurements.

== Data-backed trade-offs

The plots in @accuracy-delay and @pre-ringing are rendered directly from
#code-path("docs/reference-results.csv"). No plotted value is copied into the
Typst source. Colour is backed by marker shape or hatch pattern so the figures
remain legible when printed in greyscale.

#figure(
  accuracy-delay-chart(reference-results),
  caption: [
    Magnitude-accuracy versus mean-delay trade-off over all five reference
    targets. Each point is a realised 129-tap design on the common 1024-point
    grid; the error axis is logarithmic. Shape and colour identify the method.
  ],
) <accuracy-delay>

The phase-free designs occupy the low-delay side of @accuracy-delay by allowing
substantially more magnitude error. The alternating method occupies the
high-accuracy region, but usually at a greater mean delay than direct phase
interpolation. This plot is descriptive rather than a universal ranking:
targets differ in difficulty, and the methods do not optimise the same norm.

#figure(
  pre-ringing-chart(reference-results),
  caption: [
    Energy before the realised impulse-response peak for the five reference
    targets (LP: low-pass; PEQ: parametric EQ; XO: crossover). Colour and hatch
    pattern identify the method.
  ],
) <pre-ringing>

@pre-ringing shows why mean delay alone is not a sufficient temporal metric.
The alternating crossover result has substantial energy ahead of its peak,
while several equaliser and notch cases concentrate almost all energy at or
after the peak. The paper therefore reports both delay and energy distribution.

== Structure-specific latency and accuracy

The graphic-equaliser comparison is read from
#code-path("docs/graphiceq-results.csv"). It is kept separate from the general
FIR comparison because offloading low bands to IIR shelves changes the
structure and the applicable target class.

#figure(
  graphiceq-chart(graphiceq-results),
  caption: [
    RMS magnitude error versus realised latency for the hybrid octave graphic
    equaliser and an all-FIR design constrained to the same latency. Markers
    are discrete implemented configurations, not a continuous design curve.
  ],
) <graphiceq-tradeoff>

At each shared latency in @graphiceq-tradeoff, the hybrid structure has lower
RMS error than the shortened all-FIR alternative. This does not generalise to
arbitrary targets: the shelf cascade cannot reproduce a rapidly alternating
octave-band “zigzag” without large interaction error.

#draft-note[
  The final evaluation will add generated magnitude, group-delay, and
  peak-aligned impulse-response plots for representative targets, with the
  optimiser budget stated in every caption.
]

= Reproducibility map

#text(size: 8.4pt)[
  #set par(justify: false, leading: 0.45em)

  - *Support split @support-split:* #code-path("mixedphase/iterative.go");
    asserted in #code-path("mixedphase/mixedphase_test.go").
  - *Alternating factorisation:* #code-path("mixedphase.DesignIterative");
    `go test ./mixedphase`.
  - *Minimum-phase reconstruction:* #code-path("mixedphase.MinimumPhaseWith");
    `go test ./mixedphase`.
  - *Prescribed complex response:*
    #code-path("mixedphase.DesignComplexLeastSquares"); `go test ./mixedphase`.
  - *Low-group-delay optimisation:*
    #code-path("mixedphase.DesignLowGroupDelay"); `go test ./mixedphase`.
  - *Hybrid graphic equaliser:* #code-path("graphiceq.Design");
    `go test ./graphiceq`.
  - *Accuracy-delay and pre-ringing plots (@accuracy-delay; @pre-ringing):*
    #code-path("docs/reference-results.csv"); `just paper-refresh`.
  - *Graphic-EQ plot (@graphiceq-tradeoff):*
    #code-path("docs/graphiceq-results.csv");
    `just paper-refresh`.
  - *Native/WASM agreement:* #code-path("scripts/test-cross-build.sh");
    `just test-cross-build`.
]

The build embeds the repository revision shown on the title page. The Typst
source, bibliography, generated figure inputs, and build workflow live beside
the implementation; the PDF is a build artifact.

= Limitations and open work

The current draft intentionally leaves three conclusions open. First, the
committed benchmark suite has not yet been turned into the paper's full
cross-target argument. Second, the original 2012 figures have not been
reconstructed from machine-readable data. Third, perceptual evaluation is
limited to objective pre-ringing and delay proxies; controlled listening tests
are outside the present scope.

= Conclusion

The 2012 contribution is the alternating minimum/linear-phase factorisation:
two short FIR factors repeatedly compensate each other's windowing error so a
pre-ringing budget can be spent without defaulting to a full linear-phase
support. The exact support equation, regularisation, stop-before-rise policy,
comparison methods, failure analysis, and benchmark evidence belong to this
revision.

More broadly, mixed-phase FIR design is not one optimisation problem but a
family of choices about which phase information to preserve, which error to
minimise, and how to spend finite support. This revision makes those choices
and their provenance explicit and binds its new evidence to executable designs.
Final comparative conclusions will follow after the full cross-target analysis
and representative response plots have been technically reviewed.

#colbreak()

#bibliography("references.bib", style: "ieee", title: "References")
