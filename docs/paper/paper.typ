#import "charts.typ": (
  accuracy-delay-chart, cross-target-summary-table, cross-target-win-count,
  graphiceq-chart, group-delay-response-chart, magnitude-response-chart, number,
  peak-aligned-impulse-chart, pre-ringing-chart, representative-results-table,
  signal-flow-diagram,
)
#import "style.typ": code-path, paper

#let revision = sys.inputs.at("revision", default: "working tree")
#let reference-results = csv("../reference-results.csv", row-type: dictionary)
#let reference-response = csv("../reference-response.csv", row-type: dictionary)
#let reference-impulse = csv("../reference-impulse.csv", row-type: dictionary)
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
    stopping rule, delay-selection rule, comparison methods, and evaluation
    protocol introduced by the present revision. The later comparison covers
    phase interpolation, weighted complex approximation, minimum-phase
    truncation, direct low-group-delay optimisation, and a structure-specific
    hybrid graphic equaliser. The benchmark's principal finding is that the
    construction's delay budget, an input in 2012, is what limits it: held fixed
    it leads no metric on any reference target, because on targets whose
    minimum-phase factor fits the allocated support the design reduces to a
    delayed minimum-phase filter. Selecting the budget against a stated
    objective instead makes it lead realised RMS dB magnitude error on every
    target while declining the budget wherever it would be wasted. All
    implementations and measurements are maintained in a public Go repository so
    that each quantitative claim can be traced to a test or a committed
    comparison artifact.
  ],
  status-body: [
    This is a reviewed English author manuscript. The boundary between the 2012
    contribution and later repository work has been audited against the
    original paper. Its charts are generated from committed reference CSVs;
    the tagged PDF is generated and attached by the repository release
    workflow.
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

== Historical scope and revision boundary <revision-boundary>

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
stopping policy, common metrics, competing methods, deterministic benchmark
artifacts, and a rule for choosing the delay budget. Those additions must not be
read back as results of the 2012 paper. In particular, the delay budget is an
_input_ to the 2012 construction; treating it as an output, as
@delay-selection does, is work of this revision and is what the benchmark
below shows the construction needs. The revised treatment:

- states the alternating factorisation with an exact total-support budget;
- compares designs under identical targets, weights, tap counts, and delay
  constraints;
- measures the realised impulse response rather than only a design grid;
- exposes cases where each method is unreliable;
- derives the delay budget from the target instead of accepting it; and
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

$ h[n] = (a ast b)[n], quad N_A + N_B - 1 = N, $ <factor-convolution>

where $a[n]$ is causal and minimum phase and $b[n]$ is a symmetric
linear-phase residual. If the requested pre-ringing budget is $d$ samples, the
repository implementation assigns

$ N_B = 2 d + 1, quad N_A = N - N_B + 1. $ <support-split>

Thus $d = 0$ yields the minimum-phase endpoint and
$d = (N - 1) / 2$ yields the linear-phase endpoint without changing the final
tap count.

#figure(
  signal-flow-diagram(),
  caption: [
    Signal-flow comparison redrawn in the simple block style of the original
    paper's Figures 2 and 3 @budde2012. The upper structure is not Budde's: it
    is the mixed FIR/IIR arrangement of Goertz, Kleber, Makarski and Thaden
    @goertz2011, which @budde2012 reproduces as its Figure 2 in order to
    motivate the departure from it. The 2012 construction replaces that
    minimum-phase IIR bank with the finite minimum-phase factor $a[n]$ and
    cascades it with the linear-phase factor $b[n]$. The two FIR
    blocks may be convolved into the single mixed-phase response $h[n]$ that
    runs.
  ],
) <factor-signal-flow>

@factor-signal-flow makes the support and delay roles explicit: only
the symmetric factor $b[n]$ spends samples before its centre, while $a[n]$
concentrates its energy causally. The diagram is structural; the realised
response and all reported metrics are computed from the convolved $N$-tap
filter.

#figure(
  text(size: 7.5pt)[
    #table(
      columns: (0.38fr, 1.05fr, 1.55fr),
      align: (center, left, left),
      table.header([Symbol], [Meaning], [Public Go API]),
      [$M(omega)$],
      [magnitude derived from prototype],
      [`prototype` argument; input phase discarded],

      [$N$],
      [output tap count],
      [#code-path("IterativeConfig.Length"); `len(Result.Taps)`],

      [$d$], [linear-factor delay], [#code-path("IterativeConfig.Delay")],

      [$N_A$], [minimum-factor taps], [`len(Result.MinimumPhasePart)`],

      [$N_B$], [linear-factor taps], [`len(Result.LinearPhasePart)`],
      [$K$], [dense FFT grid], [#code-path("IterativeConfig.FFTSize")],

      [$epsilon$], [magnitude floor], [#code-path("IterativeConfig.Epsilon")],

      [$P_"max"$], [pass budget], [#code-path("IterativeConfig.Iterations")],

      [$P$], [accepted passes], [#code-path("Result.Iterations")],
      [$tau$],
      [stopping tolerance],
      [#code-path("IterativeConfig.ToleranceDB")],
    )
  ],
  caption: [
    Mathematical notation and its public API representation for the
    alternating design. All exported identifiers are in package
    #code-path("mixedphase").
  ],
) <api-notation>

= The 2012 alternating construction <alternating-construction>

The following restates the implemented algorithm in English, with notation
normalised to the present paper. It follows @budde2012 step for step except
where noted after the list:

1. Transform the prototype and retain its target magnitude $M$.
2. Reconstruct a dense minimum-phase spectrum from $M$.
3. Transform to time, truncate and window it to $N_A$ taps, producing $a$.
4. Divide the target spectrum by the spectrum of $a$ to obtain the residual.
5. Force the residual to zero phase, transform to time, centre it, and
  truncate and window it to $N_B$ taps, producing $b$.
6. Convolve the two factors to obtain the mixed-phase response.
7. Return to the residual step, alternately dividing by $b$ and $a$, so each
  factor compensates the windowing influence of the other.

Two statements of the original are deliberately absent above, because neither
is implemented here. Step 6 of @budde2012 offers forcing *minimum* phase as an
alternative to forcing zero phase, and the sentence immediately following its
step list then requires one of the two factors to be reversed in time, which
yields a minimum/maximum-phase split rather than the minimum/linear-phase split
used throughout this paper. Both belong to the extensions listed in
@revision-boundary and neither was evaluated.

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

For the public entry point, the executable steps are:

1. Validate `prototype` and #code-path("IterativeConfig"), resolve zero-valued
  fields to documented defaults, and transform the prototype on the $K$-point
  grid.
2. Compute $N_B = 2d + 1$ and $N_A = N - N_B + 1$. Return the appropriate
  single-factor endpoint immediately when $d = 0$ or $N_A = 1$.
3. Reconstruct and window `MinimumPhasePart`, divide by its spectrum with floor
  $epsilon$, then reconstruct and window `LinearPhasePart`.
4. For at most $P_"max"$ complete passes, redesign the minimum factor from the
  current linear factor and then the linear factor from the candidate minimum
  factor.
5. Convolve the candidate factors, recompute #code-path("Result.Metrics") from
  the realised taps, and reject the pass if RMS magnitude error rises.
6. Otherwise accept the factors, increment $P$, and stop when the RMS-error
  change is below $tau$.
7. Return #code-path("Result.Taps"), both factors, the accepted-pass count $P$,
  and the realised metrics.

Negative #code-path("IterativeConfig.Iterations") returns the initial,
uncorrected factorisation. Negative
#code-path("IterativeConfig.ToleranceDB") retains the configured pass count by
disabling both the rising-error and settling checks. These two experimental
controls are intentionally distinct.

=== Minimum-phase reconstruction <minimum-reconstruction>

The implementation provides two equivalent dense-grid reconstructions:

- a real-cepstrum method that folds the cepstrum onto its causal half, the
  classical construction of Oppenheim and Schafer @oppenheim1989 that
  @budde2012 cites for this step; and
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
experiments that require an exact pass count. The reproducibility appendix
reports the configured pass budget, while every alternating-design CSV row
records the number of accepted passes.

=== Selecting the delay budget <delay-selection>

In the 2012 construction the delay $d$ is given. @failure-mode-table and
@cross-target-summary show that this is the construction's principal practical
weakness rather than a detail of its interface, and that it fails in both
directions. Too large a budget on a target whose minimum-phase factor already
fits $N - 2 d$ taps buys nothing: the design degenerates as described in
@evaluation-protocol and pays $d$ samples of latency for a delayed
minimum-phase filter. Too small a non-zero budget is worse still. On the steep-crossover
fixture a budget of one sample raises the relative magnitude error to 77.5%,
against 1.227% at $d = 0$ and 2.639% at the budget selected below, because a
three-tap linear factor cannot approximate the residual quotient at all. The
error is therefore neither monotone nor unimodal in $d$, and no fixed default
is defensible across targets.

This revision therefore treats $d$ as an output. The rule is stated as a
constrained selection over the same construction:

$
  d^* = arg min_(d in D) E_"dB" (d) \
  "subject to" quad E_"rel" (d) <= (1 + sigma) E_"rel" (0),
$ <delay-objective>

where $E_"dB"$ is the realised RMS dB magnitude error, $E_"rel"$ the relative
linear-magnitude error, $sigma$ a caller-stated slack, and $D$ a set of
candidate budgets that always contains zero. The dB error is the objective
because it is the measure sensitive to stopband depth, which @contrast-magnitude
shows is what the factorisation actually buys; the relative error is the
constraint because it is dominated by the passband, which is what a deeper
stopband can otherwise be paid for with. Because $d = 0$ is always a candidate
and always satisfies the constraint, the selected design is never worse than
minimum-phase truncation in $E_"dB"$.

Two properties of this rule should be stated plainly. It does not promise a
better design on every axis: the selected budget may carry more group delay and
a larger $E_"rel"$, bounded by $sigma$, and on the steep-crossover fixture it
accepts both. And $D$ is a strided scan with local refinement rather than every
admissible budget, so the selection is a heuristic; it is exact only when $D$ is
exhaustive, and the repository's tests record one fixture where the strided scan
is 0.76 dB short of the exhaustive result. The implementation uses $sigma = 2$
and a stride of four, which the reproducibility appendix records.

= Post-2012 comparison methods

None of the methods in this section was evaluated in the 2012 contribution.
The repository adds three general comparison paths and one structure-specific
design to test the historical construction against alternatives under common
budgets.

== Prescribed phase <prescribed-phase>

Phase interpolation constructs a target whose unwrapped phase moves between
the minimum- and linear-phase endpoints and then projects its inverse transform
onto the finite support. Weighted complex least squares approximates the same
target directly in coefficient space; Lawson reweighting @lawson1961 trades RMS
complex error for a lower peak error. That reweighting is multiplicative and
converges towards its own equilibrium, so a supplied weight has little influence
left after many passes.

Zero-weight bins are genuinely unconstrained. They can diverge even when the
weighted band is fitted accurately, so weak weights are safer than removing
bins. The absolute complex objective also tends to prioritise the passband of
a low-pass target unless attenuation is reflected in the weights.

== Phase-free low-group-delay optimisation <low-delay-design>

Wu, Gao, and Teo formulate FIR design as magnitude-constrained group-delay
optimisation without prescribing the phase @wu2013. The local implementation
uses a penalty ladder and limited-memory BFGS with an analytic gradient. Its
result depends on the starting point because changing between local basins can
require moving a zero across the unit circle. Iteration count and penalty
stages are therefore experimental budgets, not evidence of convergence.

== Structure-specific graphic equalisation <graphic-eq-design>

For octave graphic equalisation, Bruschi, Välimäki, Liski, and Cecchi replace
the lowest linear-phase FIR band with an IIR shelving filter and retain the FIR
structure above it @bruschi2022. That approach changes the implementation
structure rather than selecting a general FIR phase. It belongs in the
comparison because it buys latency for an important target class, but it is
not part of the general mixed-phase API.

= Repository evaluation protocol <evaluation-protocol>

The 2012 paper contains illustrative response and signal-flow figures but no
machine-readable result set. The present revision's common benchmark suite
evaluates low-pass, parametric-EQ, crossover, deep-notch, measured
room-correction, and steep-crossover targets. Each method receives the same
target samples, frequency weights, tap budget, and applicable delay or
magnitude constraint.

The first five are smooth curves whose minimum-phase factor fits inside the
$N_A$ taps the split allocates to it. That has a consequence which must be
stated before any of their numbers are read: when the minimum-phase factor
alone already reproduces the target, the residual quotient is unit-magnitude,
its zero-phase inverse transform is a unit impulse, and the alternating
correction converges to the identity. The construction then degenerates to
$z^(-d)$ times a minimum-phase filter, and the reported magnitude error
describes the reconstruction rather than the factorisation. On all five the
linear factor carries no measurable energy away from its centre tap.

The steep-crossover target — an eighth-order crossover at 800 Hz — is included
precisely because it does not fit. Its linear factor carries 92.4% of its
energy off centre and the correction loop accepts five passes, so it is the
only target in the suite on which the alternating construction is doing the
work the method claims. The minphase-truncation method, which is the same
design with the delay budget set to zero, is included for the same reason: it
is the baseline that separates the factorisation from the reconstruction.

The reported response is always recomputed from the realised taps. The suite
records:

- RMS and peak magnitude error;
- mean and peak group delay in meaningful magnitude bands;
- peak location, energy centroid, and energy before the peak;
- coefficient range and constraint violation; and
- accepted iteration count.

Group delay is masked in deep stopbands, where phase is numerically unstable
and perceptually irrelevant. The deterministic quality CSV contains no runtime
column. Machine-local timings, including the machine, Go version, and trial
count, live in #code-path("docs/reference-timings.csv") and are regenerated
only with `just compare-timings`. The paper build consumes committed quality
artifacts and never reruns timing measurements.

== Data-backed trade-offs

The plots in @accuracy-delay and @pre-ringing are rendered directly from
#code-path("docs/reference-results.csv"). No plotted value is copied into the
Typst source. Colour is backed by marker shape or hatch pattern so the figures
remain legible when printed in greyscale.

#figure(
  accuracy-delay-chart(reference-results),
  caption: [
    Magnitude-accuracy versus mean-delay trade-off over all six reference
    targets at 48 kHz. Each point is a realised 129-tap design on the common
    1024-point grid; the three fixed-budget phase-controlled methods use a
    16-sample budget, minimum-phase truncation none, and the selected-delay
    method chooses its own per target. The
    error axis is logarithmic. Shape and colour identify the method; complete
    optimiser budgets are recorded in the reproducibility appendix.
  ],
) <accuracy-delay>

The phase-free designs occupy the low-delay side of @accuracy-delay by allowing
substantially more magnitude error. The plot separates into two regimes, and
they must be read separately. On the five smooth targets, which cluster below
25 samples, the fixed-budget alternating design occupies the high-accuracy
region but at a greater mean delay than direct phase interpolation — the delay
is spent without being earned, for the reason given in @evaluation-protocol.
The steep-crossover cluster near 50 samples inverts that relation: there the
alternating design attains both a lower mean delay than phase interpolation and
a far lower dB error, which is the behaviour the construction is meant to
produce.

The selected-delay variant of @delay-selection resolves the two regimes into
one. On the five smooth targets it chooses $d = 0$ and its marker therefore
coincides with minimum-phase truncation, at 0.5 to 10.4 samples of mean delay
instead of the 16.5 to 26.4 the fixed budget pays; on steep-crossover it chooses
$d = 22$ and moves further into the accurate region than the fixed budget
reaches. Where the two alternating markers coincide the cross is drawn over the
inverted triangle, which is the visible signature of a declined budget.

Note that the relative-error axis, being a linear-magnitude norm, understates
the steep-crossover gap and is the one axis on which the selected budget is
worse than $d = 0$; @cross-target-summary and @contrast-magnitude give the dB
view. This plot is descriptive rather than a universal ranking: targets differ
in difficulty, and the methods do not optimise the same norm.

#figure(
  pre-ringing-chart(reference-results),
  caption: [
    Energy before the realised impulse-response peak for the six reference
    targets at 48 kHz and 129 output taps (LP: low-pass; PEQ: parametric EQ;
    XO: crossover; Notch: deep notch; Room: room correction; Steep XO:
    eighth-order 800 Hz crossover). Colour and hatch pattern identify the
    method; the configurations are identical to @accuracy-delay.
  ],
) <pre-ringing>

@pre-ringing shows why mean delay alone is not a sufficient temporal metric.
The alternating crossover result has substantial energy ahead of its peak,
while several equaliser and notch cases concentrate almost all energy at or
after the peak. The paper therefore reports both delay and energy distribution.
The chart also shows the limit of this metric on its own: within each target
group the alternating and minimum-phase-truncation bars are near-indistinguishable,
including on steep-crossover, so pre-peak energy alone does not separate a
mixed-phase design from a delayed minimum-phase one. @representative-impulse
and @contrast-impulse make that distinction.

== Cross-target interpretation

@cross-target-summary counts the lowest value in each metric across the six
fixed targets. The counts are computed from the committed CSV during the paper
build; they are descriptive comparisons under the stated common budget, not
proof that metrics with different objectives are interchangeable.

#figure(
  text(size: 7.4pt)[
    #cross-target-summary-table(reference-results)
  ],
  caption: [
    Number of the six reference targets on which each method has the lowest
    value for the named realised-response metric. Ties, if present, count for
    every tied method. All cells are calculated directly from
    #code-path("docs/reference-results.csv") under the budget of
    @accuracy-delay.
  ],
) <cross-target-summary>

The fixed-budget alternating construction has the lowest relative magnitude
error on
#cross-target-win-count(
  reference-results,
  "budde-iterative",
  "relative_magnitude_error",
) of six targets and the lowest RMS magnitude error on
#cross-target-win-count(
  reference-results,
  "budde-iterative",
  "rms_magnitude_error_db",
) of six — it wins no column on any target, which is the clearest statement of
the problem @delay-selection addresses. Selecting the budget instead changes
that outcome completely: the selected-delay variant has the lowest RMS magnitude
error on
#cross-target-win-count(
  reference-results,
  "budde-adaptive",
  "rms_magnitude_error_db",
) of six targets, the only method to lead a column on every target, and the
lowest relative magnitude error on
#cross-target-win-count(
  reference-results,
  "budde-adaptive",
  "relative_magnitude_error",
) of six. It gives up the relative-error lead only on steep-crossover, where
@delay-objective spends the slack it is allowed. Minimum-phase truncation leads
relative error on
#cross-target-win-count(
  reference-results,
  "minphase-truncation",
  "relative_magnitude_error",
) of six and RMS error on
#cross-target-win-count(
  reference-results,
  "minphase-truncation",
  "rms_magnitude_error_db",
) of six; the selected-delay variant reproduces it exactly wherever it declines
a budget, so those columns are shared by construction rather than contested.

The low-group-delay optimiser records the lowest mean delay on
#cross-target-win-count(
  reference-results,
  "low-group-delay",
  "mean_group_delay",
) of six targets and the smallest coefficient range on
#cross-target-win-count(
  reference-results,
  "low-group-delay",
  "coefficient_range_db",
) of six, but it achieves this by solving a different problem with a
2 dB magnitude constraint. It also has the least pre-peak energy on
#cross-target-win-count(
  reference-results,
  "low-group-delay",
  "pre_peak_energy_ratio",
) of six; the low-pass and steep-crossover exceptions show that the lowest mean
delay need not minimise ringing before the largest coefficient.

Complex minimax has no aggregate win in these magnitude-only columns. That is
not a failure of its stated objective: Lawson reweighting controls peak complex
error, and in the deep-notch case it improves both RMS and maximum dB error over
direct phase interpolation.

These cross-target results support a conditional selection rule. Use the
alternating construction with a selected budget when fixed-support magnitude
fidelity in the dB sense is primary; it is never worse than minimum-phase
truncation on that measure and is decisively better when the target starves the
minimum-phase factor. Use minimum-phase truncation when a linear-magnitude norm
is the specification and latency is critical, since it leads that column and
carries the least delay of the magnitude-faithful designs. Use direct low-delay
optimisation when delay and coefficient range justify an explicit magnitude
tolerance, and complex weighting when the complex-response norm or band
priorities are the actual specification. The fixed-budget alternating design is
not recommended for new work: every target on which it would be chosen is one
where @delay-objective selects a different budget, and it is retained here as
the 2012 reference point and as the evidence for that conclusion.

== Representative realised responses

The parametric-EQ case exposes frequency-response shape and scalar metrics
without the ill-conditioned stopband phase of a low-pass example. The impulse
plot instead uses the fourth-order Linkwitz--Riley 2 kHz low-pass crossover
fixture from the common suite, whose realised response occupies far more of the
available support.

Both are smooth targets, so both are degenerate in the sense of
@evaluation-protocol, and neither on its own can show the alternating
construction shaping anything. Each is therefore published together with the
steep-crossover fixture on identical axes. The pairs are the evidence: the
figures below establish what the degenerate case looks like so that the
support-starved case can be read against it, and it is the difference between
the two, not either one alone, that distinguishes a mixed-phase design from a
delayed minimum-phase filter.

Both examples use realised 129-tap filters at 48 kHz on the 1024-point design
and analysis grid. The three prescribed-phase designs use $d=16$ samples. The
alternating design permits at most 12 passes and stops before rising error or
below a $10^(-7)$ dB change; phase interpolation has no iterative loop; complex
minimax permits 16 Lawson passes with a $10^(-4)$ stopping tolerance; and
low-group-delay optimisation uses a 2 dB magnitude tolerance and four stages
of at most 80 L-BFGS steps. The title-page repository revision identifies the
commit that produced the artifacts.

#figure(
  magnitude-response-chart(reference-response),
  caption: [
    Target and realised magnitude for the parametric-EQ fixture. The common
    budget is 48 kHz, $N=129$, and $K=1024$; the prescribed-phase methods use
    $d=16$, while low-group-delay optimisation has no fixed-delay constraint
    and instead uses the stated 2 dB magnitude tolerance. Line dash, as well
    as colour, identifies each method. Source:
    #code-path("docs/reference-response.csv").
  ],
) <representative-magnitude>

@representative-magnitude shows the alternating construction following this
smooth target closely — 0.0058 dB RMS error against 0.0995 dB for phase
interpolation and 0.2396 dB for complex minimax, both of which preserve the
intended bell but introduce visibly larger finite-support error. The
low-group-delay result spends its allowed magnitude tolerance to move energy
earlier. The delay-zero baseline is closer still, at 0.0003 dB and a mean delay
of 6.21 samples against 22.21: on a target this smooth the 16-sample budget buys
nothing, which is the degeneracy stated in @evaluation-protocol seen in the
frequency domain.

#figure(
  group-delay-response-chart(reference-response),
  caption: [
    Realised group delay for the same parametric-EQ designs and budgets as
    @representative-magnitude. Only the weighted 1.8--5 kHz analysis band is
    shown; each bin is weighted by the squared target magnitude. Group delay
    outside this band is deliberately excluded rather than interpreted.
    Source: #code-path("docs/reference-response.csv").
  ],
) <representative-group-delay>

The prescribed-phase methods cluster around the same frequency-dependent delay,
with the alternating response reaching a somewhat higher peak. The phase-free
optimiser produces the lowest mean delay, but its local solution also contains
negative group delay in part of the weighted analysis band. That local
behaviour is visible here and is not captured by a single mean value.

#figure(
  peak-aligned-impulse-chart(reference-impulse),
  caption: [
    Peak-aligned realised impulse responses for the fourth-order
    Linkwitz--Riley 2 kHz low-pass crossover target and the same tap, grid, and
    optimiser budgets as @representative-magnitude. Each response is divided
    by its own absolute peak and displayed as coefficient magnitude in dB,
    with values below −80 dB clipped to the plot floor. This intentionally
    discards coefficient sign so low-level temporal detail remains visible.
    The horizontal origin is each response's peak, so the plot compares
    temporal distribution rather than gain. Source:
    #code-path("docs/reference-impulse.csv").
  ],
) <representative-impulse>

Peak alignment in @representative-impulse separates waveform shape from
absolute latency, and on this target the separation is total. The alternating
and minimum-phase-truncation traces coincide: over the 113 shared aligned
samples their normalised coefficients differ by at most $3.0 dot 10^(-7)$,
their peak indices differ by exactly the 16-sample budget (26 against 10), and
their energy centroids differ by exactly 16.00 samples. Substantial pre-peak
energy — 44.77% for both — is therefore not evidence of a mixed-phase design:
the delay-zero baseline has precisely as much. This figure shows the degeneracy
of @evaluation-protocol rather than contradicting it. Coefficient signs, full
unnormalised values, peak indices, and the exact pre-peak energy ratios remain
in the committed CSV and scalar result set.

#figure(
  peak-aligned-impulse-chart(reference-impulse, target: "steep-crossover"),
  caption: [
    Peak-aligned realised impulse responses for the eighth-order 800 Hz
    crossover target, on axes and budgets identical to
    @representative-impulse so the two may be read against each other. Source:
    #code-path("docs/reference-impulse.csv").
  ],
) <contrast-impulse>

@contrast-impulse is the same plot on a target whose minimum-phase factor does
not fit the $N_A$ taps the split allocates. The two traces that coincided in
@representative-impulse no longer do: their peak-aligned coefficients differ by
up to $4.3 dot 10^(-2)$, five orders of magnitude more than before, at an equal
peak index of 51. The alternating design is therefore not a delayed copy of the
baseline here, which is the regime in which the factorisation is doing work.
On a normalised dB scale that difference is a fraction of a decibel near the
peak and is deliberately not claimed to be conspicuous in the figure; the
separation it produces is a frequency-domain effect, and @contrast-magnitude is
where it becomes visible. Both bounds are pinned by
`TestPeakAlignedImpulseSeparatesTheRegimes`.

#figure(
  magnitude-response-chart(
    reference-response,
    target: "steep-crossover",
    y-bounds: (-100, 6),
    y-ticks: (
      (-100, "-100"),
      (-80, "-80"),
      (-60, "-60"),
      (-40, "-40"),
      (-20, "-20"),
      (0, "0"),
    ),
  ),
  caption: [
    Target and realised magnitude for the steep-crossover fixture, under the
    budgets of @representative-magnitude. Note the axis: this target spans
    100 dB where the parametric-EQ fixture spans 16, and values below −100 dB
    are clamped to the plot floor. Source:
    #code-path("docs/reference-response.csv").
  ],
) <contrast-magnitude>

@contrast-magnitude shows what that work buys. The alternating construction
tracks the crossover skirt far into the stopband, while phase interpolation,
complex minimax and the delay-zero baseline all depart from it well above
−70 dB. Integrated over the band this is the largest margin in the suite:
6.90 dB RMS magnitude error against 54.48 dB for phase interpolation, 54.93 dB
for minimum-phase truncation, 42.84 dB for the low-group-delay optimiser and
72.23 dB for complex minimax — at a mean group delay of 49.61 samples, which is
_lower_ than phase interpolation's 53.04. The method wins on accuracy and delay
at once here, which it does on none of the five smooth targets. Selecting the
budget by @delay-objective improves this further, to 3.31 dB at $d = 22$, and is
the trace drawn for the selected-delay method in the same figure.

The corresponding group-delay plot is omitted: this target's weight is confined
to the band below 516 Hz, where all six designs lie between 42.9 and 59.1
samples and the curves are not separable at a legible scale. The scalar
group-delay ripple for every method is in @cross-target-summary and the
committed CSV.

#let alternating-crossover = reference-results.find(row => (
  row.at("target") == "crossover" and row.at("method") == "budde-iterative"
))
#let alternating-peq = reference-results.find(row => (
  row.at("target") == "parametric-eq" and row.at("method") == "budde-iterative"
))
The committed scalar rows make the distinction explicit: the alternating
crossover places
#number(
  100 * float(alternating-crossover.at("pre_peak_energy_ratio")),
  digits: 1,
)\% of its energy before the peak, while the smooth parametric-EQ case rounds
to
#number(
  100 * float(alternating-peq.at("pre_peak_energy_ratio")),
  digits: 3,
)\%. The configured $d$ allocates support to the linear factor; it is not by
itself proof that the target needs or uses that support.

#figure(
  text(size: 7.5pt)[
    #representative-results-table(reference-results)
  ],
  caption: [
    Scalar results for the representative parametric-EQ designs, read directly
    from #code-path("docs/reference-results.csv"). Delay is the
    magnitude-squared-weighted mean over 1.8--5 kHz; $P$ is the accepted count
    (alternating passes, Lawson passes, or total L-BFGS steps). The tap, grid,
    delay, weight, tolerance, and iteration budgets are those stated above.
  ],
) <representative-results>

@representative-results makes the trade explicit: the alternating method
achieves the smallest response errors in this case, whereas the phase-free
optimiser reduces mean delay by roughly an order of magnitude under a
different, tolerance-constrained objective. The table is a representative
case, not a claim that one method dominates every target.

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
    The ten octave centres span 31.25 Hz to 16 kHz at 48 kHz; one through four
    low bands are offloaded.
  ],
) <graphiceq-tradeoff>

At each shared latency in @graphiceq-tradeoff, the hybrid structure has lower
RMS error than the shortened all-FIR alternative. This does not generalise to
arbitrary targets: the shelf cascade cannot reproduce a rapidly alternating
octave-band “zigzag” without large interaction error.

= Failure modes and validity limits <failure-modes>

The successful cases above are conditional results, not guarantees outside
their stated targets and weights. @failure-mode-table gives each known failure
mode an observable symptom and a required reporting or design control. These
cases are regression-tested alongside the successful cases; a design should
not be selected from the aggregate trade-off plots without checking the
corresponding row.

#figure(
  text(size: 7.25pt)[
    #table(
      columns: (0.72fr, 1.08fr, 1.2fr),
      align: (left, left, left),
      table.header(
        [*Method or metric*],
        [*Observable failure*],
        [*Control and interpretation*],
      ),
      [Alternating correction],
      [
        A later correction pass can raise realised RMS magnitude error and
        amplify platform rounding.
      ],
      [
        Treat the pass count as a maximum; reject the first rising candidate
        and report accepted passes.
      ],

      [Alternating support use],
      [
        If the minimum-phase factor already fits the target, the linear
        residual approaches a centred impulse. The result then resembles a
        minimum-phase response plus delay despite the available support.
      ],
      [
        Pre-peak energy does _not_ detect this: on @representative-impulse the
        degenerate and delay-zero designs agree on it to four decimals.
        Compare against the same design at $d=0$, or measure the linear
        factor's energy away from its centre tap, as
        #code-path("TestSteepTargetActuallyExercisesTheFactorisation") does.
        Selecting the budget by @delay-objective removes the mode rather than
        only reporting it, because $d = 0$ then wins whenever it would occur.
      ],

      [Hand-picked delay budget],
      [
        A small non-zero budget is worse than both endpoints: on
        steep-crossover $d = 1$ reaches 77.5% relative magnitude error against
        1.227% at $d = 0$. The error is neither monotone nor unimodal in $d$.
      ],
      [
        Do not default the budget. Select it against a stated objective, as
        @delay-objective does and
        #code-path("TestSmallDelayBudgetsAreTheWorstChoice") pins. A selection
        over a strided candidate set is a heuristic; report the stride, and use
        an exhaustive set when the budget must be optimal.
      ],

      [Low-delay optimisation],
      [
        A linear-phase start can remain in a substantially worse local basin
        than the default minimum-phase start.
      ],
      [
        State #code-path("InitialTaps") and optimisation budgets; compare
        starts when the basin matters.
      ],

      [Weighted complex fit],
      [
        Bins assigned exactly zero weight can diverge while the weighted band
        remains accurately fitted.
      ],
      [
        Prefer weak positive weights to omitted bands and inspect the realised
        response over the full application band.
      ],

      [Group-delay metric],
      [
        Near a spectral null, phase differentiation is numerically fragile and
        can dominate a whole-band delay statistic.
      ],
      [
        Mask or downweight deep stopbands; publish the evaluated band and
        weight, as in @representative-group-delay.
      ],

      [Hybrid graphic EQ],
      [
        Rapidly alternating octave gains do not fit the smooth low-shelf
        cascade and leave large interaction error.
      ],
      [
        Offload only a smooth low-frequency region; retain the all-FIR
        structure for zigzag-like targets.
      ],
    )
  ],
  caption: [
    Failure modes, observable symptoms, and required controls. The entries are
    qualitative; their executable regression evidence is mapped in the
    reproducibility appendix.
  ],
) <failure-mode-table>

The first four rows are algorithmic limitations: stopping, factor utilisation,
initialisation, or weighting changes the solution. The fifth is a measurement
limitation and must not be mistaken for filter latency. The sixth is a
target-class limitation: the hybrid result in @graphiceq-tradeoff applies to
smooth low-frequency band trajectories, not arbitrary graphic-EQ gain
sequences.

= Reproducibility appendix

This appendix covers every public design algorithm and every numbered equation,
figure, and table in the current draft. Configuration fields not named below
retain their documented zero-value defaults. The build embeds the repository
revision shown on the title page.

== Algorithms and analysis

#text(size: 8.15pt)[
  #set par(justify: false, leading: 0.42em)

  - *Alternating factorisation, @alternating-construction.* _Implementation:_
    #code-path("mixedphase.DesignIterative") in
    #code-path("mixedphase/iterative.go"). _Evidence:_
    #code-path("TestDesignIterativeHonoursTapBudget"),
    #code-path("TestIterativeStopsBeforeRisingError"), and
    #code-path("TestIterativeCrossBuildDeterminism"). _Reference budget:_
    $N=129$, $d=16$, $K=1024$, $P_"max"=12$;
    #code-path("MethodCepstrum"), rectangular window, scale-relative
    $epsilon=10^(-12)$ of the target peak, and default
    $tau=10^(-7)$ dB. The CSV records accepted $P$. _Reproduce:_
    `go test ./mixedphase`; `just test-cross-build`.

  - *Delay selection, @delay-selection.* _Implementation:_
    #code-path("mixedphase.DesignIterativeAuto") in
    #code-path("mixedphase/autoiterative.go"), published in the comparison as
    the #code-path("budde-adaptive") method. _Evidence:_
    #code-path("TestDesignIterativeAutoSelectsZeroWhenDelayBuysNothing") and
    #code-path("TestDesignIterativeAutoNeverLosesToMinimumPhase") pin the
    guarantee, #code-path("TestDesignIterativeAutoExhaustiveSearchIsNeverWorse")
    records that the strided candidate set is a heuristic,
    #code-path("TestAdaptiveDelaySelectionBeatsTheFixedBudget") pins the six
    selected budgets and the 3.310 dB figure,
    #code-path("TestAdaptiveSelectionBuysStopbandDepthForLatency") the 70.8 dB
    and 2.63-sample trade, and
    #code-path("TestSmallDelayBudgetsAreTheWorstChoice") the 77.5% figure quoted
    for $d = 1$. _Reference budget:_ $N=129$, $K=1024$, $P_"max"=12$ per
    candidate, slack $sigma=2$, candidate stride 4, $d$ searched over
    $[0, 64]$. The CSV records the selected $d$ in
    #code-path("phase_delay_samples"). _Reproduce:_ `go test ./mixedphase`;
    `just compare-check`.

  - *Minimum-phase reconstruction, @minimum-reconstruction.*
    _Implementation:_ #code-path("mixedphase.MinimumPhaseWith") and the method
    selected by #code-path("MinimumPhaseConfig.Method"). _Evidence:_
    #code-path("TestMinimumPhaseMethodsAgree"),
    #code-path("TestHilbertReproducesTargetMagnitude"), and
    #code-path("TestIterativeMethodsReachComparableQuality"). _Reference
      budget:_ the figures use #code-path("MethodCepstrum") on $K=1024$ with the
    scale-relative floor; #code-path("MethodHilbert") is the independently
    tested alternative. _Reproduce:_ `go test ./mixedphase`.

  - *Phase interpolation, @prescribed-phase.* _Implementation:_
    #code-path("mixedphase.DesignPhaseInterpolation"). _Evidence:_
    #code-path("TestPhaseInterpolationMovesPeakContinuously") and
    #code-path("TestUniformWeightMatchesPhaseInterpolation"). _Reference
      budget:_ #code-path("Length=129"), #code-path("FFTSize=1024"),
    #code-path("Mix=0.25"), #code-path("MethodCepstrum"), default floor.
    _Reproduce:_ `go test ./mixedphase`.

  - *Weighted complex approximation, @prescribed-phase.* _Implementation:_
    #code-path("mixedphase.DesignComplexLeastSquares"). _Evidence:_
    #code-path("TestUniformWeightMatchesPhaseInterpolation"),
    #code-path("TestMinimaxTradesRMSForPeak"), and
    #code-path("TestUnweightedBandsAreUnconstrained"). _Reference budget:_
    #code-path("Length=129"), #code-path("FFTSize=1024"),
    #code-path("Mix=0.25"), uniform initial weight, 16 Lawson passes, and the
    default $10^(-4)$ minimax tolerance. _Reproduce:_
    `go test ./mixedphase`.

  - *Low-group-delay optimisation, @low-delay-design.* _Implementation:_
    #code-path("mixedphase.DesignLowGroupDelay"). _Evidence:_
    #code-path("TestLowGroupDelayGradientMatchesFiniteDifferences"),
    #code-path("TestLowGroupDelayUndercutsMinimumPhase"), and
    #code-path("TestLowGroupDelayDependsOnInitialisation"). _Reference budget:_
    #code-path("Length=129"), #code-path("FFTSize=1024"), 2 dB magnitude
    tolerance, target-specific #code-path("DelayWeight"), four penalty stages,
    80 L-BFGS steps per stage, unit initial penalty, and the default
    minimum-phase start. _Reproduce:_ `go test ./mixedphase`.

  - *Hybrid graphic equaliser, @graphic-eq-design.* _Implementation:_
    #code-path("graphiceq.Design") and #code-path("graphiceq.DefaultLength").
    _Evidence:_ #code-path("TestDefaultLengthHalvesPerOffloadedBand"),
    #code-path("TestHybridBeatsEqualLatencyFIR"), and
    #code-path("TestImpulseResponseMatchesMetrics"). _Reference budget:_ ten
    octave bands from 31.25 Hz to 16 kHz at 48 kHz with gains
    $(6,-3,0,4,-6,2,0,-2,5,0)$ dB, #code-path("IIRBands=0..4"), default
    #code-path("Q=1"), default FFT grid, and rectangular FIR window.
    _Reproduce:_ `go test ./graphiceq`.

  - *Common realised-response analysis.* _Implementation:_
    #code-path("mixedphase.Analyze"), #code-path("internal/reference.Run"), and
    #code-path("internal/reference.analyze"). _Evidence:_
    #code-path("TestRunCoversEveryMethodAndMetric"),
    #code-path("TestTargetsShareFixedBudgets"), and the committed-CSV assertion
    in #code-path("internal/reference/reference_test.go"). _Budget:_ six
    257-tap prototypes at 48 kHz, 129 output taps, $K=1024$, target-specific
    group-delay weights, and no timing trials. _Reproduce:_
    `go test ./internal/reference`; `just compare-check`.
]

== Numbered items and artifacts

#text(size: 8.15pt)[
  #set par(justify: false, leading: 0.42em)

  - *@factor-convolution and @support-split; @factor-signal-flow;
      @api-notation.* _Source:_
    #code-path("IterativeConfig"), #code-path("Result"), and
    #code-path("mixedphase/iterative.go"). The qualitative signal-flow
    comparison is redrawn by #code-path("signal-flow-diagram") from Figures 2
    and 3 of @budde2012; it contains no measured data. _Evidence:_
    #code-path("TestDesignIterativeHonoursTapBudget"),
    #code-path("TestIterativeZeroDelayIsMinimumPhaseEndpoint"), and
    #code-path("TestIterativeMaximumDelayIsLinearPhaseEndpoint"). _Budget:_ all
    valid $0 <= d <= (N - 1) / 2$; the reference row uses $N=129$, $d=16$.
    _Reproduce:_ `go test ./mixedphase`; rebuild with `just paper`.

  - *@accuracy-delay and @pre-ringing.* _Generator:_
    #code-path("examples/mixedphase") through
    #code-path("internal/reference.Run"). _Artifact:_
    #code-path("docs/reference-results.csv"), whose schema and budgets are
    asserted by #code-path("TestRunCoversEveryMethodAndMetric"). _Budget:_ the
    general reference budgets listed above; iteration values in the CSV are
    accepted counts, not maxima. _Reproduce:_ `just compare-check`; rebuild
    with `just paper`.

  - *@cross-target-summary.* _Generator:_ the
    #code-path("cross-target-summary-table") Typst helper computes every count
    from #code-path("docs/reference-results.csv"). _Evidence:_ the artifact is
    byte-compared by #code-path("TestRunCoversEveryMethodAndMetric"). _Budget:_
    the six-target common reference budget listed under “Common realised-
    response analysis”; no summary value is stored in the Typst source.
    _Reproduce:_ `just compare-check`; rebuild with `just paper`.

  - *@representative-magnitude, @representative-group-delay,
      @representative-impulse, @contrast-impulse, @contrast-magnitude, and
      @representative-results.* _Generator:_
    #code-path("internal/reference.RepresentativeResponses"), invoked by
    #code-path("examples/mixedphase"). _Artifacts:_
    #code-path("docs/reference-response.csv"),
    #code-path("docs/reference-impulse.csv"), and
    #code-path("docs/reference-results.csv"); all three are byte-compared by
    #code-path("TestRepresentativeResponsesCoverRealisedDesigns") or
    #code-path("TestRunCoversEveryMethodAndMetric"). _Budget:_ the
    parametric-EQ response target and fourth-order Linkwitz--Riley 2 kHz
    low-pass crossover impulse target at 48 kHz, $N=129$, $K=1024$, $d=16$ for
    prescribed phase, and the exact weights, tolerances, and iteration limits
    listed under “Representative realised responses.” Both artifacts
    additionally carry the steep-crossover target under the same budget, which
    is what @contrast-impulse and @contrast-magnitude plot; the published target
    lists are
    #code-path("reference.ResponseTargets") and
    #code-path("reference.ImpulseTargets"), and
    #code-path("TestRepresentativeResponsesCoverRealisedDesigns") fails if
    either shrinks to a single target. The reference test also requires the
    plotted alternating crossover to place at least 10% of its energy and at
    least eight coefficients above −40 dB before its peak, while
    #code-path("TestPeakAlignedImpulseSeparatesTheRegimes") pins the
    $3.0 dot 10^(-7)$ and $4.3 dot 10^(-2)$ deviations quoted for the two
    impulse figures. _Reproduce:_ `just compare-check`; rebuild with
    `just paper`.

  - *@graphiceq-tradeoff.* _Generator:_ #code-path("examples/graphiceq").
    _Artifact:_ #code-path("docs/graphiceq-results.csv"). Each hybrid split is
    paired with an all-FIR #code-path("graphiceq.Design") whose
    #code-path("Length") equals the hybrid tap count. _Budget:_ the graphic-EQ
    configuration listed above. _Reproduce:_ `just compare-check`; rebuild
    with `just paper`.

  - *@failure-mode-table.* _Implementation and evidence:_ correction-loop
    instability is guarded by
    #code-path("TestIterativeStopsBeforeRisingError") and
    #code-path("TestIterativeConditioning"); support utilisation by the
    alternating crossover assertions in
    #code-path("TestRepresentativeResponsesCoverRealisedDesigns");
    initialisation sensitivity by
    #code-path("TestLowGroupDelayDependsOnInitialisation"); zero-weight bins by
    #code-path("TestUnweightedBandsAreUnconstrained"); stopband-delay masking by
    #code-path("TestDefaultDelayWeightMasksSpectralNulls") and
    #code-path("mixedphase.delayWeights"); and the graphic-EQ target-class limit
    by #code-path("TestZigzagTargetDefeatsTheSplit"). _Budget:_ each named test
    owns and asserts its deterministic fixture; the table introduces no
    hand-entered numerical result. _Reproduce:_ `go test ./mixedphase`;
    `go test ./graphiceq`; rebuild with `just paper`.
]

The Typst source, bibliography, generated figure inputs, and build workflow
live beside the implementation; the PDF is a build artifact. There are no
hand-entered quantitative results: @representative-results and every plot read
committed CSV fields directly.

= Limitations and open work

The comparison is limited to six fixed targets, one 129-tap output budget,
and the stated optimiser budgets; it does not establish asymptotic convergence
or perceptual preference. The delay selection of @delay-selection is evaluated
under one slack and one candidate stride, and its candidate set is not
exhaustive; the choice of RMS dB error as its objective is an engineering
judgement about which error a fixed-support design should protect, not a result.
Only one of the six targets starves the minimum-phase factor, so the advantage
that selection produces rests on a single fixture of that class. The original signal-flow figures are redrawn only as
a qualitative structural diagram; their response example has not been
reconstructed because no machine-readable source data were published.
Perceptual evaluation is limited to objective pre-ringing and delay proxies;
controlled listening tests are outside the present scope.

= Conclusion

The 2012 contribution is the alternating minimum/linear-phase factorisation:
two short FIR factors repeatedly compensate each other's windowing error so a
pre-ringing budget can be spent without defaulting to a full linear-phase
support. The exact support equation, regularisation, stop-before-rise policy,
delay selection, comparison methods, failure analysis, and benchmark evidence
belong to this revision.

The benchmark's principal finding concerns the budget rather than the
factorisation. With the delay budget fixed at 16 samples the construction leads
no metric on any of the six targets: on the five whose minimum-phase factor fits
the allocated support it is a delayed minimum-phase filter, which the same code
produces at $d = 0$ with 10 to 16 fewer samples of latency and no more error.
That is a property of the fixed budget, not of the factorisation, and it is
correctable. Treating the budget as an output of a stated objective —
@delay-objective — makes the construction lead realised RMS dB magnitude error
on all six targets, the only method here to lead a column outright, while
declining the budget entirely wherever it would be wasted. On the one target
that starves the minimum-phase factor, selection reaches 3.310 dB against
6.901 dB for the fixed budget and 42.838 dB for the best competing method, at
2.63 samples more mean group delay than minimum-phase truncation and 71 dB more
stopband rejection. What the factorisation buys is stopband depth at fixed
support, and it is worth buying only when the target's minimum-phase response
does not fit the taps the split leaves it — a condition that can be tested
before designing.

More broadly, mixed-phase FIR design is not one optimisation problem but a
family of choices about which phase information to preserve, which error to
minimise, and how to spend finite support. This revision makes those choices
and their provenance explicit and binds its new evidence to executable designs.
Across the fixed reference suite, alternating correction with a selected budget
is the consistent dB-accuracy choice, minimum-phase truncation the consistent
linear-magnitude choice, and the phase-free optimiser the consistent mean-delay
choice; none of these conclusions extends beyond the stated targets, weights,
and budgets.

#bibliography("references.bib", style: "ieee", title: "References")
