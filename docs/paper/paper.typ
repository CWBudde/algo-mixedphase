#import "charts.typ": (
  accuracy-delay-chart, cross-target-summary-table, cross-target-win-count,
  latency-accuracy-chart, magnitude-response-chart, phase-regime-chart,
  pre-ringing-chart, signal-flow-diagram,
)
#import "style.typ": code-path, paper

#let revision = sys.inputs.at("revision", default: "working tree")
#let reference-results = csv("../reference-results.csv", row-type: dictionary)
#let reference-response = csv("../reference-response.csv", row-type: dictionary)
#let reference-sweep = csv("../reference-delay-sweep.csv", row-type: dictionary)
#let reference-regimes = csv(
  "../reference-phase-regimes.csv",
  row-type: dictionary,
)

#show: paper.with(
  title: "Fixed-Support Mixed-Phase FIR Filter Design",
  subtitle: "A reproducible revision and comparison of latency–accuracy trade-offs",
  author: "Christian-W. Budde",
  revision: revision,
  abstract-body: [
    A requested magnitude response fixes a group-delay floor: every causal
    realisation differs from the minimum-phase one by an all-pass factor, whose
    group delay is non-negative, so no design of that magnitude is faster. This
    paper organises fixed-support mixed-phase FIR design around that floor,
    building on the alternating minimum/linear-phase factorisation proposed at
    DAGA 2012. Above the floor the surplus latency _is_ the all-pass factor, and
    choosing it is the whole of the remaining phase freedom; below it no phase
    choice helps and only the magnitude can give way. Three measured results
    follow. The 2012 construction makes the above-floor choice implicitly and
    takes the trivial one: its budget inserts a pure delay, so the cascade's
    group-delay deviation equals its minimum-phase factor's and does not respond
    to the budget at all, where a prescribed phase over the same latency converts
    it into flatness almost proportionally. Its budget is bounded by the output
    support rather than by the available latency, and a sweep over 129 to 1025
    taps finds it worth anything on one of six reference targets and only below
    about 513 taps. Below the floor, five of six targets buy delay by conceding
    magnitude, the steepest giving up 70% of its floor for 1.90 dB. What the
    construction does deliver is latency: at matched magnitude accuracy a
    linear-phase filter needs 8 to 22 times that of a minimum-phase-led design.
    All implementations and measurements are maintained in a public Go repository
    so that each quantitative claim traces to a test or a committed comparison
    artifact.
  ],
  status-body: [
    Reviewed English author manuscript. Charts are generated from committed
    reference CSVs; the tagged PDF is built by the repository release workflow.
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

This document is a revised English treatment, not a historical translation. The
2012 contribution supplies four ideas: treat the available pre-ringing time as a
budget between the minimum- and linear-phase endpoints; realise the response as a
cascade of short minimum- and linear-phase factors; form each factor from the
target divided by the other, so it compensates the other's windowing error; and
repeat that residual design alternately. Its minimum/maximum-phase split,
frequency-dependent weighting, and three-factor decomposition are sketched as
possible extensions rather than developed algorithms, and are not evaluated here.

Everything else below is work of the present revision: the exact support
convention, regularised spectral division, concrete reconstructions and stopping
policy, the delay-selection rule, the comparison methods, the failure analysis, and
the benchmark artifacts. The delay budget in particular is an _input_ to the 2012
construction; treating it as an output, as @delay-selection does, is new, and is
what the measurements below show the construction needs. A fuller attribution audit
is kept with the repository sources rather than here.

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

== The group-delay floor a magnitude request implies <group-delay-floor>

Item 3 above is usually written as though delay were free to choose. It is not,
and the constraint that binds it is the reason the rest of this paper is
organised the way it is.

Every causal realisation of a given magnitude differs from the minimum-phase one
by an all-pass factor: $H(z) = H_"min" (z) A(z)$ with $|A(e^(j omega))| = 1$. An
all-pass response has non-negative group delay at every frequency, so

$ tau_H (omega) >= tau_"min" (omega) quad "for all" omega. $ <delay-floor>

The minimum-phase group delay is therefore a floor, and it is a property of the
requested magnitude rather than a design choice. Asking for a steeper transition
or a deeper notch raises it whether or not the designer intended to spend the
latency.

Two regimes follow, and they are not variations of one problem:

/ Above the floor: the surplus $tau_H - tau_"min"$ _is_ the all-pass factor, and
  choosing it is the whole of the remaining phase freedom. A design can spend the
  surplus on flattening the group delay, running from minimum phase through
  linear phase to maximum phase, or it can decline to and spend it as pure delay.
  @phase-regimes shows both being done.

/ Below the floor: there is nothing to shape. The request is infeasible at that
  magnitude, and the only currency left is the magnitude itself: a shallower
  transition lowers $tau_"min"$ along with it. @below-the-floor measures that
  exchange.

@floor-table gives the floors of the six reference targets, measured as the
weighted mean group delay of the minimum-phase design at the published operating
point. The spread is the point. A room-correction curve costs almost nothing,
while the eighth-order crossover costs 49.4 samples before any phase shaping is
possible at all --- so the 16-sample budget this comparison publishes is consumed
three times over by that target's magnitude request alone, and the design it
returns is necessarily a near-minimum-phase one. Reading a fixed budget across
targets without their floors alongside is what makes the construction look
degenerate when it is merely out of room.

#figure(
  table(
    columns: 3,
    align: (left, right, right),
    table.header([Target], [$tau_"min"$], [Budget ceiling]),
    [Room correction], [0.50], [0],
    [First-order low-pass], [5.86], [39],
    [Parametric EQ], [6.21], [15],
    [Deep notch], [8.63], [14],
    [LR4 crossover], [10.41], [39],
    [LR8 crossover], [49.37], [0],
  ),
  caption: [
    The group-delay floor of each reference target at 129 taps on a 1024-point
    grid, in samples, and the largest delay budget the factorisation admits
    before its minimum-phase factor no longer fits the $N - 2 d$ taps the split
    leaves it. Both are measured, not assumed: the floors are the minimum-phase
    row of #code-path("docs/reference-phase-regimes.csv"), and the ceilings come
    from the factor's own support at a $10^(-6)$ energy tail. Two targets admit
    no budget at all, because their minimum-phase response already needs the
    whole support.
  ],
) <floor-table>

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
    Signal-flow comparison redrawn in the block style of @budde2012, Figures 2
    and 3. The upper structure is the mixed FIR/IIR arrangement the original
    reproduces in order to motivate departing from it; the construction replaces
    its minimum-phase IIR bank with the finite minimum-phase factor $a[n]$ and
    cascades that with the linear-phase factor $b[n]$. The two FIR blocks may be
    convolved into the single response $h[n]$ that runs.
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

Two options of the original are absent above because neither is implemented
here: forcing the residual to minimum rather than zero phase, and time-reversing
one factor to obtain a minimum/maximum-phase split. Both are extensions the
original only sketches.

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

In the 2012 construction the delay $d$ is given. It is worth being precise about
what it measures: $d$ is not the filter's latency but its _excess_ over the floor
of @group-delay-floor, since the design's mean group delay is $tau_"min" + d$
while the minimum-phase factor still fits. A controller that can afford $L$
samples end to end therefore has $L - tau_"min"$ to offer here, not $L$ — which
for the eighth-order crossover at 129 taps is negative for every budget this
comparison publishes.

@failure-mode-table and @cross-target-summary show that the given budget is the
construction's principal practical weakness rather than a detail of its
interface, and that it fails in both directions. Too large a budget on a target whose minimum-phase factor already
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
  d^* = arg min_(d in D) E_"dB" (d) quad "subject to" \
  E_"rel" (d) <= max((1 + sigma) E_"rel" (0), epsilon_"rel"), \
  E_"dB" (d) <= E_"dB" (0) - delta quad "for" d > 0,
$ <delay-objective>

where $E_"dB"$ is the realised RMS dB magnitude error, $E_"rel"$ the relative
linear-magnitude error, $sigma$ a caller-stated slack, and $D$ a set of candidate
budgets that always contains zero. The dB error is the objective because it is
the measure sensitive to stopband depth, which @contrast-magnitude shows is what
the factorisation actually buys; the relative error is the constraint because it
is dominated by the passband, which is what a deeper stopband can otherwise be
paid for with. Because $d = 0$ is always a candidate and always satisfies both
constraints, the selected design is never worse than minimum-phase truncation in
$E_"dB"$.

The two bounds on the right are not decoration; each corrects a way the plain
form misbehaves. The floor $epsilon_"rel"$ is needed because a purely
multiplicative passband constraint stops meaning anything once the $d = 0$ design
is already accurate. Designing the eighth-order crossover into 193 taps puts
$E_"rel" (0)$ near $2 dot 10^(-3)$, so a three-times ceiling rejects the
eleven-sample budget at $7.5 dot 10^(-3)$ — trading 16 dB of stopband depth for a
passband regression under a hundredth of a decibel. The margin $delta$ is needed
because the objective does not price latency at all: on a 129-tap low-pass a
one-sample budget lowers $E_"dB"$ from 0.800 dB to 0.498 dB, and without a margin
the rule spends a sample of latency and abandons the exact minimum-phase design
for three tenths of a decibel. The implementation uses $epsilon_"rel" = 10^(-2)$,
about 0.09 dB, and $delta = 1$ dB, and breaks ties towards the shorter delay.

=== The construction spends its surplus as pure delay <ripple-invariance>

@group-delay-floor says that latency above the floor is an all-pass factor and
that choosing it is the whole of the remaining phase freedom. This construction
makes that choice implicitly, and the choice it makes is the trivial one.

The linear-phase factor is symmetric, so its phase is exactly linear and its
group delay exactly constant. Group delays add along a cascade, so the deviation
of $h$ from constant group delay is the minimum-phase factor's deviation alone,

$ rho(h) = rho(a), $ <ripple-identity>

where $rho$ denotes weighted RMS group-delay deviation. The all-pass factor the
budget inserts is $z^(-d)$: it translates the group-delay curve by $d$ and does
not flatten it. Across all six reference targets the measured ripple is identical
to nine decimal places at every budget the split admits, whether or not the
linear factor carries energy away from its centre tap.

This is a statement about the construction, not about mixed-phase design. A
prescribed-phase design over the same latency converts the surplus into flatness
in proportion, and @phase-regimes reads the two against each other on one axis.
Its dashed curves are flat while the factorisation still has a minimum-phase
factor to hold; they fall only past each target's budget ceiling from
@floor-table, and there they fall because the factor has been starved away rather
than because the phase has been shaped. At the ceiling itself the low-pass shows
the gap plainly: 44.9 samples of latency leave the factorisation at its full
minimum-phase ripple of 1.117 samples, where the prescribed continuum is already
down to 0.419. The LR4 crossover is the same story, 0.766 against 0.287.

A caller who wants flat group delay must therefore buy it with a prescribed
phase, not with this budget. What this construction offers instead is
@latency-accuracy.

#figure(
  phase-regime-chart(reference-regimes),
  caption: [
    How each family spends latency above the group-delay floor, at 129 taps on a
    1024-point grid. Both axes are normalised per target so six targets share one
    frame: latency is the fraction of the way from that target's own floor to
    linear phase, and ripple is relative to its own minimum-phase ripple, so
    every curve starts at $(0, 1)$ and linear phase is $(1, 0)$. Solid curves
    prescribe a phase between the endpoints and convert latency into flatness
    almost proportionally. Dashed curves are the alternating factorisation, which
    holds its ripple until the budget starves its minimum-phase factor and only
    then descends. The continuum is drawn to linear phase only; beyond it the
    family mirrors back to maximum phase, which costs latency without recovering
    accuracy.
  ],
) <phase-regimes>

Two properties of the selection rule should be stated plainly. It does not promise a
better design on every axis: the selected budget may carry more group delay and
a larger $E_"rel"$, bounded by $sigma$, and on the steep-crossover fixture it
accepts both. And $D$ is a strided scan with local refinement rather than every
admissible budget, so the selection is a heuristic; it is exact only when $D$ is
exhaustive, and the repository's tests record one fixture where the strided scan
is 0.76 dB short of the exhaustive result. The implementation uses $sigma = 2$
and a stride of four, which the reproducibility appendix records.

= Post-2012 comparison methods

None of the methods in this section was evaluated in the 2012 contribution.
The repository adds three general comparison paths to test the historical
construction against alternatives under common budgets.

== Prescribed phase <prescribed-phase>

Phase interpolation constructs a target whose unwrapped phase moves along the
continuum and then projects its inverse transform onto the finite support. With
$phi.alt_"lin" (omega) = -omega (N-1) \/ 2$ the prescription is

$
  phi.alt_mu (omega) = (1 - mu) phi.alt_"min" (omega) + mu phi.alt_"lin" (omega),
  quad mu in [0, 2],
$ <phase-mix>

so $mu = 0$ is minimum phase and $mu = 1$ is linear phase. The upper half is not
an extrapolation into nothing: at $mu = 2$ the prescription is
$-omega (N-1) - phi.alt_"min"$, which is exactly the maximum-phase response of the
same magnitude, because negating the phase reverses the impulse response and the
added full-length delay restores causality. The family is therefore symmetric
about linear phase, $h_(2 - mu) [n] = h_mu [N - 1 - n]$, which the repository pins
to $10^(-12)$ against a peak tap of 0.371. Every magnitude measure is
correspondingly symmetric in $mu$, so the upper half costs latency and returns no
accuracy; it is implemented because the continuum is only complete with it, not
because it is a useful place to sit.

Weighted complex least squares approximates the same target directly in
coefficient space; Lawson reweighting @lawson1961 trades RMS complex error for a
lower peak error. That reweighting is multiplicative and converges towards its
own equilibrium, so a supplied weight has little influence left after many
passes.

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
mixed-phase design from a delayed minimum-phase one. The committed impulse
artifact carries the coefficient-level evidence that does.

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

The fixed-budget alternating construction leads no column on any target: it has
the lowest relative magnitude error on
#cross-target-win-count(
  reference-results,
  "budde-iterative",
  "relative_magnitude_error",
) of six targets and the lowest RMS magnitude error on
#cross-target-win-count(
  reference-results,
  "budde-iterative",
  "rms_magnitude_error_db",
) of six. That is the clearest statement of the problem @delay-selection
addresses.
Selecting the budget reverses it: the selected-delay variant leads RMS magnitude
error on
#cross-target-win-count(
  reference-results,
  "budde-adaptive",
  "rms_magnitude_error_db",
) of six, the only method here to lead a column on every target. It shares the
relative-error column with minimum-phase truncation, because it reproduces that
design exactly wherever it declines a budget, and gives the lead up only on
steep-crossover where @delay-objective spends the slack it is allowed.

The low-group-delay optimiser leads mean delay and coefficient range, but under a
different problem with a 2 dB magnitude constraint; its pre-peak energy lead has
low-pass and steep-crossover exceptions, showing that the lowest mean delay need
not minimise ringing before the largest coefficient. Complex minimax leads none of
these magnitude-only columns, which is not a failure of its stated objective —
Lawson reweighting controls peak complex error, and on the deep notch it improves
both RMS and maximum dB error over direct phase interpolation.

The resulting selection rule is conditional. Use the alternating construction with
a selected budget when fixed-support magnitude fidelity in the dB sense is
primary; it is never worse than minimum-phase truncation on that measure and is
decisively better when the target starves the minimum-phase factor. Use
minimum-phase truncation when a linear-magnitude norm is the specification and
latency is critical. Use direct low-delay optimisation when delay and coefficient
range justify an explicit magnitude tolerance, and complex weighting when the
complex-response norm or band priorities are the specification. The fixed-budget
alternating design is not recommended for new work; it is retained here as the
2012 reference point and as the evidence for that conclusion.

== What the delay budget buys

Only one reference target is support-starved, and it is the only one on which
the factorisation shapes anything. @contrast-magnitude is therefore the single
frequency-domain figure worth showing: the degenerate case is by construction
the $d = 0$ row of @cross-target-summary, and plotting it adds a curve that
coincides with a curve already there.

The design budget is 48 kHz, realised 129-tap filters on the 1024-point grid.
The three prescribed-phase designs use $d = 16$; the alternating loop permits at
most 12 passes and stops before rising error or below a $10^(-7)$ dB change;
complex minimax permits 16 Lawson passes at a $10^(-4)$ tolerance; and
low-group-delay optimisation uses a 2 dB magnitude tolerance over four stages of
at most 80 L-BFGS steps.

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
    budgets stated above. Values below −100 dB are clamped to the plot floor.
    Line dash, as well as colour, identifies each method. Source:
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

== Below the floor: trading magnitude for delay <below-the-floor>

Everything so far spends latency above the floor. A controller with a hard
latency budget below it has the opposite problem, and @delay-floor says the only
way out is to stop asking for the same magnitude.

That exchange is measurable directly. The low-group-delay optimiser of
@low-delay-design minimises weighted group delay subject to a magnitude
tolerance, so widening the tolerance is exactly the question "what does a
magnitude concession buy in delay". @floor-probe-table walks it from a quarter
of a decibel to two.

#figure(
  table(
    columns: 6,
    align: (left, right, right, right, right, right),
    table.header([Target], [$tau_"min"$], [0.25 dB], [0.5 dB], [1 dB], [2 dB]),
    [Low-pass], [5.86], [5.40], [4.93], [3.93], [1.76],
    [Parametric EQ], [6.21], [5.75], [5.27], [4.25], [1.93],
    [LR4 crossover], [10.41], [10.37], [10.28], [10.09], [9.73],
    [Deep notch], [8.63], [8.59], [8.49], [8.17], [6.67],
    [Room correction], [0.50], [0.47], [0.42], [0.32], [0.10],
    [LR8 crossover], [49.37], [49.60], [49.61], [49.52], [49.44],
  ),
  caption: [
    Weighted mean group delay in samples reached by the low-group-delay
    optimiser as its magnitude tolerance widens, against each target's floor.
    129 taps, 1024-point grid, four penalty stages of at most 80 L-BFGS steps.
    Five targets buy delay below their floor, the low-pass most steeply at 70%
    for 1.90 dB of RMS magnitude error. The LR8 crossover buys nothing: at 129
    taps its magnitude is already unrealisable, so there is no accuracy left to
    concede. Source: #code-path("docs/reference-phase-regimes.csv").
  ],
) <floor-probe-table>

The ladder stops at 2 dB because wider tolerances stop measuring anything. At
4 dB the optimiser has not converged and its answer tracks the iteration budget
rather than the tolerance --- the low-pass reports 33.1, 59.8 or 2.5 samples
under three different budgets. At 8 dB the constraint admits a spectral null,
where group delay is undefined; every budget then returns the same meaningless
figure with the maximum magnitude error saturated at its 60 dB clamp. Both are
pinned by a test so that the ladder's endpoint is a recorded limit rather than a
choice of presentation.

The shape of the exchange is worth noting. It is strongly target-dependent and
strongly non-linear: the low-pass gives up 70% of its floor across the ladder and
more than half of that in its last step alone, while the LR4 crossover moves 7%
over the whole range.
A latency budget below the floor is therefore not a small perturbation of the
design problem but a different one, and the honest answer to a controller that
cannot afford $tau_"min"$ is usually a shallower filter rather than a cleverer
phase.

== Longer filters: the budget is a function of the support <length-sweep>

Everything above sits at one operating point, 129 output taps from a 257-tap
prototype. A loudspeaker controller is not there. It runs filters of several
hundred to a few thousand taps behind an alignment delay line that is already
paid for, so it is natural to expect a larger budget to be both affordable and
useful. Measurement contradicts the second half of that expectation.

#code-path("docs/reference-delay-sweep.csv") sweeps the same six analytic curves
over output lengths 129, 257, 513 and 1025 with $d$ strided over the admissible
$[0, (N-1)\/2]$, on a fixed 8192-point grid. It uses 2049-tap fixtures rather
than the 257-tap ones behind every other figure here, because a 257-tap prototype
is shorter than a 513-tap filter and every method would reproduce it exactly. The
two artifacts must not be read against each other, and each sweep row carries the
prototype length it used.

The result is one-sided enough to state without a figure. Measured as the largest
RMS dB error any non-zero budget saves against the $d = 0$ design, one of the six
targets benefits materially and only below about 513 output taps: the
eighth-order crossover gains 57.19 dB at 129 taps and 23.05 dB at 257, then
nothing from 513 upwards. Room correction gains hundredths of a decibel, and the
remaining four gain nothing at any length. The budget is therefore a function of
the output support, not of the available latency, and a longer filter needs
_less_ of it, not more.
The supports explain the ordering: the minimum-phase factor needs 52 taps for the
LR4 crossover, 53 for the low-pass, 116 for the parametric EQ, 129 for the deep
notch, 238 for the LR8 crossover and 995 for room correction, measured as the
leading taps holding all but $10^(-6)$ of its energy. Once $N$ comfortably
exceeds that support the factor fits at any admissible budget and the correction
has nothing to recover. Room correction is the instructive exception: its support
exceeds three of the four lengths, yet a symmetric factor cannot supply the
broadband tail it is missing.

This also settles what the construction is for. It is not a way to spend latency
on phase linearity — @ripple-invariance shows it cannot be — but a way to avoid
spending latency at all.

#figure(
  latency-accuracy-chart(reference-sweep),
  caption: [
    Magnitude error against latency, latency being weighted mean group delay in
    each target's own analysis band. Each dashed curve is one target's
    linear-phase family, a symmetric FIR of $2L+1$ taps at latency $L$ sampled
    every 32 samples. Each filled circle is the same target's 1025-tap
    minimum-phase-led design at $d = 0$; those designs reach $10^(-15)$ dB, so all
    but room correction clamp to the plot floor, which is set at the $10^(-3)$ dB
    tolerance the comparison matches on. Both axes are logarithmic. Source:
    #code-path("docs/reference-delay-sweep.csv").
  ],
) <latency-accuracy>

The horizontal distance from each circle to its own curve is the comparison a
latency-bounded application faces, and it is where the construction earns its
place. A linear-phase filter of latency $L$ has only $2L+1$ taps to spend on the
magnitude; a minimum-phase-led design of $N$ taps spends all $N$ and carries only
the latency its own phase implies. Matching the circle's accuracy to within a
thousandth of a decibel costs linear phase 22 times the latency on the deep
notch, 19 on the parametric EQ, 18 on the LR4 crossover, 16 on the low-pass and
8 on the LR8 crossover. On room correction no sampled linear-phase latency
matches it at all: the circle sits at 0.046 dB and 0.6 samples, while the
linear-phase family reaches only 0.250 dB at 512 samples.

The price is the group-delay deviation of the minimum-phase factor, 0.8 to 8.0
samples across these targets, which @ripple-invariance shows no budget can
reduce. That is the trade to evaluate: a bounded group-delay deviation in
exchange for the magnitude accuracy of a linear-phase filter an order of
magnitude longer in latency.

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
        Pre-peak energy does _not_ detect this: on the LR4 crossover fixture
        the degenerate and delay-zero designs agree on it to four decimals.
        Compare against the same design at $d=0$, or measure the linear
        factor's energy away from its centre tap, as the appendix's regression
        for this row does.
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
        @delay-objective does. A selection
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
        Mask or downweight deep stopbands, and publish the evaluated band and
        weight rather than a whole-band curve.
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
initialisation, or weighting changes the solution. The last is a measurement
limitation and must not be mistaken for filter latency.

= Reproducibility appendix

This appendix covers every public design algorithm and every numbered equation,
figure, and table in the current draft. Configuration fields not named below
retain their documented zero-value defaults, and the build embeds the repository
revision shown on the title page.

Unless an entry states otherwise, its evidence is reproduced by
`go test ./mixedphase`, its artifacts by `just compare-check`, and its figures by
`just paper`; only exceptions are named per entry.

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
    tested alternative.

  - *Phase interpolation, @prescribed-phase.* _Implementation:_
    #code-path("mixedphase.DesignPhaseInterpolation"). _Evidence:_
    #code-path("TestPhaseInterpolationMovesPeakContinuously") and
    #code-path("TestUniformWeightMatchesPhaseInterpolation"). _Reference
      budget:_ #code-path("Length=129"), #code-path("FFTSize=1024"),
    #code-path("Mix=0.25"), #code-path("MethodCepstrum"), default floor.

  - *Weighted complex approximation, @prescribed-phase.* _Implementation:_
    #code-path("mixedphase.DesignComplexLeastSquares"). _Evidence:_
    #code-path("TestUniformWeightMatchesPhaseInterpolation"),
    #code-path("TestMinimaxTradesRMSForPeak"), and
    #code-path("TestUnweightedBandsAreUnconstrained"). _Reference budget:_
    #code-path("Length=129"), #code-path("FFTSize=1024"),
    #code-path("Mix=0.25"), uniform initial weight, 16 Lawson passes, and the
    default $10^(-4)$ minimax tolerance.

  - *Low-group-delay optimisation, @low-delay-design.* _Implementation:_
    #code-path("mixedphase.DesignLowGroupDelay"). _Evidence:_
    #code-path("TestLowGroupDelayGradientMatchesFiniteDifferences"),
    #code-path("TestLowGroupDelayUndercutsMinimumPhase"), and
    #code-path("TestLowGroupDelayDependsOnInitialisation"). _Reference budget:_
    #code-path("Length=129"), #code-path("FFTSize=1024"), 2 dB magnitude
    tolerance, target-specific #code-path("DelayWeight"), four penalty stages,
    80 L-BFGS steps per stage, unit initial penalty, and the default
    minimum-phase start.

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

  - *@accuracy-delay and @pre-ringing.* _Generator:_
    #code-path("examples/mixedphase") through
    #code-path("internal/reference.Run"). _Artifact:_
    #code-path("docs/reference-results.csv"), whose schema and budgets are
    asserted by #code-path("TestRunCoversEveryMethodAndMetric"). _Budget:_ the
    general reference budgets listed above; iteration values in the CSV are
    accepted counts, not maxima.

  - *@cross-target-summary.* _Generator:_ the
    #code-path("cross-target-summary-table") Typst helper computes every count
    from #code-path("docs/reference-results.csv"). _Evidence:_ the artifact is
    byte-compared by #code-path("TestRunCoversEveryMethodAndMetric"). _Budget:_
    the six-target common reference budget listed under “Common realised-
    response analysis”; no summary value is stored in the Typst source.

  - *@contrast-magnitude.* _Generator:_
    #code-path("internal/reference.RepresentativeResponses"), invoked by
    #code-path("examples/mixedphase"). _Artifact:_
    #code-path("docs/reference-response.csv"), byte-compared by
    #code-path("TestRepresentativeResponsesCoverRealisedDesigns"). _Budget:_ the
    steep-crossover target at 48 kHz, $N=129$, $K=1024$, $d=16$ for prescribed
    phase, and the weights, tolerances, and iteration limits listed under “What
    the delay budget buys.” The artifact also carries the parametric-EQ and LR4
    crossover targets under the same budget; the published target lists are
    #code-path("reference.ResponseTargets") and
    #code-path("reference.ImpulseTargets"), and the same test fails if either
    shrinks to a single target.

  - *@latency-accuracy and the budget-gain figures quoted in @length-sweep.*
    _Generator:_
    #code-path("internal/reference.SweepRows"), invoked by
    #code-path("examples/mixedphase -sweep"). _Artifact:_
    #code-path("docs/reference-delay-sweep.csv"), byte-compared by
    #code-path("TestCommittedSweepCSVIsReproducible"). _Budget:_ the six
    reference curves rebuilt as 2049-tap fixtures on a 16384-point target grid,
    designed at 129, 257, 513, and 1025 output taps on a fixed 8192-point
    design and analysis grid with at most 12 correction passes; $d$ strided
    every $(N-1)\/32$ samples, and the linear-phase family sampled every 32
    samples of latency. _Evidence:_
    #code-path("TestSweepDelayBudgetStopsPayingAsLengthGrows") and
    #code-path("TestSweepBudgetGainTableMatchesTheDocumentation") pin the gain
    figures; #code-path("TestSweepLinearPhaseNeedsFarMoreLatency") pins the
    latency factors; #code-path("TestSweepMinimumPhaseSupportBoundsTheBudget")
    pins the minimum-phase supports; and
    #code-path("TestSweepLinearPhaseHasNoRipple") pins the reference family's
    zero ripple.

  - *@floor-table, @phase-regimes and @floor-probe-table.* _Generator:_
    #code-path("internal/reference.RegimeRows"), invoked by
    #code-path("examples/mixedphase -regimes"). _Artifact:_
    #code-path("docs/reference-phase-regimes.csv"), byte-compared by
    #code-path("TestCommittedRegimesCSVIsReproducible"). _Budget:_ the published
    fixtures and operating point — the six 257-tap curves at $N=129$ on a
    1024-point grid — so that every row is comparable with @cross-target-summary
    rather than being a separate experiment. Three families: the prescribed
    continuum over $mu in [0, 2]$ in steps of $1\/8$; the alternating
    factorisation over $d in [0, 64]$ in steps of 4 at 12 correction passes; and
    the low-group-delay optimiser at 0.25, 0.5, 1, and 2 dB over four penalty
    stages of at most 80 L-BFGS steps. _Evidence:_
    #code-path("TestZeroDelayDesignSitsOnTheMinimumPhaseFloor") establishes that
    the floors are the target's and not the split's;
    #code-path("TestFactorisationHoldsItsRippleWhileTheContinuumDescends") pins
    the contrast and the budget ceilings;
    #code-path("TestContinuumGroupDelayIsLinearInMix") and
    #code-path("TestContinuumRippleIsSymmetricAndVanishesAtLinearPhase") pin the
    continuum's shape; #code-path("TestFloorProbeTradesMagnitudeForDelayBelowTheFloor")
    pins the below-floor exchange including the starved target's exception; and
    #code-path("TestLooseToleranceLeavesTheMeasurableRegime") records why the
    ladder stops at 2 dB. The reflection identity behind @phase-mix is pinned by
    #code-path("TestPhaseContinuumReflectsAboutLinearPhase").

  - *@failure-mode-table.* _Implementation and evidence:_ correction-loop
    instability is guarded by
    #code-path("TestIterativeStopsBeforeRisingError") and
    #code-path("TestIterativeConditioning"); support utilisation by
    #code-path("TestSteepTargetActuallyExercisesTheFactorisation") and the
    alternating crossover assertions in
    #code-path("TestRepresentativeResponsesCoverRealisedDesigns");
    hand-picked budgets by
    #code-path("TestSmallDelayBudgetsAreTheWorstChoice");
    initialisation sensitivity by
    #code-path("TestLowGroupDelayDependsOnInitialisation"); zero-weight bins by
    #code-path("TestUnweightedBandsAreUnconstrained"); stopband-delay masking by
    #code-path("TestDefaultDelayWeightMasksSpectralNulls") and
    #code-path("mixedphase.delayWeights"); and the group-delay invariance of
    @ripple-identity by
    #code-path("TestAdaptiveDelayBudgetCannotFlattenGroupDelay"). _Budget:_ each
    named test owns and asserts its deterministic fixture; the table introduces
    no hand-entered numerical result. _Reproduce:_ `go test ./mixedphase`;
    `go test ./internal/reference`; rebuild with `just paper`.
]

The Typst source, bibliography, generated figure inputs, and build workflow
live beside the implementation; the PDF is a build artifact. There are no
hand-entered quantitative results: every table and plot reads committed CSV
fields directly.

= Limitations and open work

The six-method comparison is limited to six fixed targets at one 129-tap output
budget; only the sweep of @length-sweep varies the support, and it compares two
families rather than all six methods. The delay selection of @delay-selection is
evaluated under one slack, one improvement margin, and one candidate stride, and
its candidate set is not exhaustive; the choice of RMS dB error as its objective
is an engineering judgement about which error a fixed-support design should
protect, not a result.

Only one of the six targets starves the minimum-phase factor, so both the case
for a non-zero budget and the case against it above 513 taps rest on a single
fixture of that class. The latency factors of @latency-accuracy are quantised by
the 32-sample sampling of the linear-phase family, and all of them are measured
as weighted mean group delay in each target's own analysis band rather than as
an implementable end-to-end latency. Every target here is zero-phase, so no
method is asked to fit a prescribed excess phase.

The floors of @floor-table and the exchange of @floor-probe-table are measured at
one support, so both are properties of these targets at 129 taps rather than of
the target curves alone; the floor in particular falls as the support grows and
the minimum-phase response is no longer truncated. The below-floor ladder is
bounded above by the optimiser's convergence rather than by the physics, so it
measures what this solver can reach, not the best achievable exchange. The
maximum-phase half of the continuum is included for completeness and is dominated
throughout: being the exact time reverse of the minimum-phase design, it costs
the most latency of any point on the continuum and recovers no accuracy for it.

Perceptual evaluation is limited to objective pre-ringing and delay proxies;
controlled listening tests are outside the present scope. The group-delay
deviation the construction leaves in place, 0.8 to 8.0 samples across these
targets, is exactly the quantity such a test would have to bound.

= Conclusion

The 2012 contribution is the alternating minimum/linear-phase factorisation:
two short FIR factors repeatedly compensate each other's windowing error so a
pre-ringing budget can be spent without defaulting to a full linear-phase
support. The exact support equation, regularisation, stop-before-rise policy,
delay selection, comparison methods, failure analysis, and benchmark evidence
belong to this revision.

Reading it against the floor of @group-delay-floor settles what its delay budget
is for. Latency above the floor is an all-pass factor, and this construction
takes the trivial one: the symmetric factor contributes exactly linear phase, so
@ripple-identity holds and the budget shifts the group-delay curve without
flattening it. A prescribed phase over the same latency does flatten it —
1.117 samples of ripple against 0.419 on the low-pass at equal delay — so the
limitation belongs to the construction rather than to the latency. The budget
also cannot buy accuracy once the support is sufficient: across 129 to 1025
output taps a non-zero budget is worth more than a decibel on one of six targets
and only below 513 taps. A longer filter behind a generous delay line therefore
wants a _smaller_ budget, not a larger one, which is the opposite of the natural
expectation.

Below the floor the currency changes. No phase choice recovers a latency budget
smaller than $tau_"min"$, and the six targets concede between 7% and 70% of their
floor for one to two decibels of magnitude error — or, for the eighth-order
crossover at 129 taps, nothing at all, because its magnitude is already beyond
the support.

Held fixed at 16 samples the budget leads no metric on any of the six targets,
because on the five whose minimum-phase factor fits the allocated support the
design is a delayed minimum-phase filter that the same code produces at $d = 0$
with 10 to 16 fewer samples of latency. That is a property of the fixed budget,
not of the factorisation, and it is correctable: selecting the budget by
@delay-objective makes the construction lead realised RMS dB magnitude error on
all six targets — the only method here to lead a column outright — while declining
the budget wherever it would be wasted, and reaching 3.310 dB on the one starved
target against 6.901 dB for the fixed budget.

The construction's real advantage is elsewhere, and it is larger. At matched
magnitude accuracy a linear-phase filter needs 8 to 22 times the latency of a
minimum-phase-led design of the same tap count, and on a broadband
room-correction target it does not match it at any latency measured. The price is
the minimum-phase factor's group-delay deviation, which no budget reduces. So the
design question this construction answers is not how to spend a latency budget on
phase linearity, but how to avoid spending one at all — and the delay budget is a
support-recovery control to be used sparingly, on short filters and steep
targets, not a latency dial.

The two applications that motivate the work therefore want different controls. An
equaliser or room correction wants the phase continuum itself exposed, a single
parameter running from minimum through linear to maximum phase with latency as
the consequence. A loudspeaker controller, which already pays an alignment delay,
wants to know its floor first: the budget it can usefully spend is what remains
after the magnitude request has taken its share, and on a steep crossover that
remainder is often nothing.

More broadly, mixed-phase FIR design is not one optimisation problem but a family
of choices about which phase information to preserve, which error to minimise, and
how to spend finite support. Across this reference suite, alternating correction
with a selected budget is the consistent dB-accuracy choice, minimum-phase
truncation the consistent linear-magnitude choice, and the phase-free optimiser the
consistent mean-delay choice; none of these conclusions extends beyond the stated
targets, weights, and budgets.

#bibliography("references.bib", style: "ieee", title: "References")
