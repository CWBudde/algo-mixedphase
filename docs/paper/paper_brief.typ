#import "charts.typ": phase-regime-chart
#import "style.typ": code-path, paper

#let revision = sys.inputs.at("revision", default: "working tree")
#let reference-regimes = csv(
  "../reference-phase-regimes.csv",
  row-type: dictionary,
)

#show: paper.with(
  title: "Fixed-Support Mixed-Phase FIR Filter Design",
  subtitle: "The group-delay floor, and what a latency budget can buy",
  author: "Christian-W. Budde",
  revision: revision,
  abstract-body: [
    A requested magnitude response fixes a group-delay floor: every causal
    realisation of that magnitude differs from the minimum-phase one by an
    all-pass factor, whose group delay is non-negative, so none is faster. This
    note organises fixed-support mixed-phase FIR design around that floor. Above
    it the surplus latency _is_ the all-pass factor, and choosing it is the whole
    of the remaining phase freedom: a prescribed phase running from minimum
    through linear to maximum phase converts surplus into group-delay flatness
    almost proportionally, while an alternating minimum/linear-phase
    factorisation spends the same surplus as pure delay. Below the floor no phase
    choice helps and only the magnitude can give way; five of six reference
    targets buy delay that way, up to 80% of their floor, and the steepest none
    at all. What the factorisation
    does deliver is latency: at matched magnitude accuracy a linear-phase filter
    needs 8 to 22 times that of a minimum-phase-led design of the same length.
    Every quantity below traces to a test or a committed artifact in a public Go
    repository.
  ],
)

// The brief is set a little tighter than the full paper, which is what makes it
// a three-page document rather than a four-page one. The abstract above is
// rendered by the shared template and keeps that template's size.
#set text(size: 8.8pt)
#set par(leading: 0.48em)

= The group-delay floor <group-delay-floor>

Let $M(omega)$ be a non-negative target magnitude and $h[n]$ a real causal FIR
response of length $N$. The permitted latency looks like a free parameter of such
a design, on a par with the tap count. It is not.

Every causal realisation of a given magnitude differs from the minimum-phase one
by an all-pass factor, $H(z) = H_"min" (z) A(z)$ with $|A(e^(j omega))| = 1$. An
all-pass response has non-negative group delay at every frequency, so

$ tau_H (omega) >= tau_"min" (omega) quad "for all" omega. $ <delay-floor>

The minimum-phase group delay is a floor, and it is a property of the requested
magnitude rather than a design choice: a steeper transition or a deeper notch
raises it whether or not the designer meant to spend the latency. Two regimes
follow, and they are different problems.

/ Above the floor: the surplus $tau_H - tau_"min"$ _is_ the all-pass factor.
  Choosing it is the whole of the remaining phase freedom — a design may spend
  the surplus flattening the group delay, or decline to and spend it as pure
  delay. @above-the-floor measures both.

/ Below the floor: there is nothing to shape. The request is infeasible at that
  magnitude, and the only currency left is the magnitude itself: a shallower
  response lowers $tau_"min"$ with it. @below-the-floor measures that exchange.

@floor-table gives the floors of six reference targets. The spread is the point.
A room-correction curve costs almost nothing, while an eighth-order crossover
costs 49.4 samples before any phase shaping is possible at all. A 16-sample
latency allowance — a common figure, and the one this comparison publishes — is
consumed three times over by that target's magnitude request alone, and any
design returned for it is necessarily near minimum phase. Reading a latency
budget across targets without their floors alongside makes a method look
degenerate when it is merely out of room.

#figure(
  table(
    columns: 3,
    align: (left, right, right),
    table.header([Target], [$tau_"min"$], [Budget ceiling]),
    [Room correction], [0.50], [11],
    [First-order low-pass], [5.86], [39],
    [Parametric EQ], [6.21], [15],
    [Deep notch], [8.63], [14],
    [LR4 crossover], [10.41], [39],
    [LR8 crossover], [49.37], [0],
  ),
  caption: [
    Group-delay floor of each reference target at $N = 129$ on a 1024-point grid,
    in samples, and the largest budget $d$ the factorisation of
    @alternating-construction admits before its minimum-phase factor no longer
    fits the $N - 2d$ taps left to it. Both are measured: the floors are the
    minimum-phase row of #code-path("docs/reference-phase-regimes.csv"), the
    ceilings the factor's own support at a $10^(-6)$ energy tail.
  ],
) <floor-table>

A small ceiling has two causes, and the number alone does not distinguish them.
The LR8 crossover is starved: 107 of its 129 taps are needed for even a
thousandth of its energy, so no budget is admissible and its magnitude was out of
reach to begin with. Room correction is the opposite shape — 24 taps hold all but
a thousandth of its energy, which is why its floor is half a sample — and its
ceiling of 11 comes from a low-level broadband tail that the stricter threshold
still counts, not from a factor that cannot fit.

= Two ways to spend the surplus <above-the-floor>

== Alternating minimum/linear-phase factorisation <alternating-construction>

The construction of @budde2012 realises the response as a cascade of a short
causal minimum-phase factor and a short symmetric linear-phase factor, each
repeatedly redesigned to compensate the other's windowing error. For ordinary
finite convolution the present revision fixes the support exactly as

$ h[n] = (a ast b)[n], quad N_A + N_B - 1 = N, $ <factor-convolution>

and splits it by the requested budget $d$ as

$ N_B = 2d + 1, quad N_A = N - N_B + 1, $ <support-split>

so $d = 0$ is the minimum-phase endpoint and $d = (N-1)\/2$ the linear-phase one
at unchanged tap count. The executable steps are:

1. Transform the prototype and retain its magnitude $M$; reconstruct a dense
  minimum-phase spectrum from it.
2. Transform to time, truncate and window to $N_A$ taps, giving $a$.
3. Divide the target spectrum by that of $a$, using a scale-relative magnitude
  floor $epsilon$; force the quotient to zero phase; transform, centre, truncate
  and window to $N_B$ taps, giving $b$.
4. Repeat step 3 alternately, dividing by $b$ and then by $a$, so each factor
  compensates the other's truncation.
5. After every complete pass, convolve the candidate factors, recompute the
  metrics from the realised taps, and reject the pass if RMS magnitude error
  rises; otherwise accept and stop once the change falls below $tau$.

Truncation is not neutral — it convolves the response with the window spectrum —
and the quotient in the next half-pass is what asks one factor to undo the
other's error. That alternating correction is the distinguishing step, and it is
not a contraction, so the pass count is a maximum budget rather than a
convergence claim and the stopping rule returns the first reproducible local
minimum.

The budget $d$ is an input to the construction, and a poor one in both
directions. Too large a budget on a target whose minimum-phase factor already
fits buys nothing and pays $d$ samples for a delayed minimum-phase filter. Too
small a non-zero budget is worse: on the steep-crossover fixture $d = 1$ raises
relative magnitude error to 77.5%, against 1.227% at $d = 0$, because a three-tap
symmetric factor cannot approximate the residual quotient at all. The error is
neither monotone nor unimodal in $d$ and no default is defensible across targets,
so this revision treats $d$ as an output of a constrained selection,

$
  d^* = arg min_(d in D) E_"dB" (d) quad "subject to" \
  E_"rel" (d) <= max((1 + sigma) E_"rel" (0), epsilon_"rel"), \
  E_"dB" (d) <= E_"dB" (0) - delta quad "for" d > 0,
$ <delay-objective>

with $E_"dB"$ the realised RMS dB magnitude error, $E_"rel"$ the relative
linear-magnitude error, $sigma$ a caller-stated slack, and $D$ a candidate set
always containing zero. The dB error is the objective because stopband depth is
what the factorisation actually buys; the relative error is the constraint
because it is dominated by the passband, which a deeper stopband can otherwise be
paid for with. Because $d = 0$ is always admissible the selected design is never
worse than minimum-phase truncation in $E_"dB"$. The floor
$epsilon_"rel" = 10^(-2)$ keeps the passband constraint meaningful once
$E_"rel" (0)$ is already small, and the margin $delta = 1$ dB stops the rule
spending latency for tenths of a decibel. $D$ is a strided scan, hence a
heuristic: one recorded fixture falls 0.76 dB short of an exhaustive search.

== The factorisation spends its surplus as pure delay <ripple-invariance>

The symmetric factor has exactly linear phase and therefore exactly constant
group delay. Group delays add along a cascade, so the deviation of $h$ from
constant group delay is the minimum-phase factor's deviation alone,

$ rho(h) = rho(a), $ <ripple-identity>

with $rho$ the weighted RMS group-delay deviation. In the terms of
@group-delay-floor, the all-pass factor this budget inserts is $z^(-d)$: it
translates the group-delay curve and does not flatten it. Across all six targets
the measured ripple is identical to nine decimal places at every budget the split
admits. This is a property of the construction, not of mixed-phase design.

== Prescribed phase <prescribed-phase>

Phase interpolation instead builds a target whose unwrapped phase moves along the
continuum and projects its inverse transform onto the finite support. With
$phi.alt_"lin" (omega) = -omega (N-1) \/ 2$,

$
  phi.alt_mu (omega) = (1 - mu) phi.alt_"min" (omega) + mu phi.alt_"lin" (omega),
  quad mu in [0, 2],
$ <phase-mix>

so $mu = 0$ is minimum phase and $mu = 1$ linear phase. At $mu = 2$ the
prescription is $-omega (N-1) - phi.alt_"min"$, exactly the maximum-phase
response of the same magnitude: negating the phase reverses the impulse response
and the added full-length delay restores causality. The family is therefore
symmetric about linear phase, $h_(2-mu) [n] = h_mu [N-1-n]$, pinned to
$10^(-12)$ against a peak tap of 0.371, so every magnitude measure is symmetric
in $mu$ and the upper half costs latency without returning accuracy. It completes
the continuum rather than offering a useful operating point.

Weighted complex least squares approximates the same prescribed target directly
in coefficient space, with Lawson reweighting @lawson1961 trading RMS complex
error for peak error.

== The two families compared

@phase-regimes reads both against each other on one axis. The prescribed
continuum converts surplus latency into flatness almost proportionally; the
factorisation holds its ripple flat until the budget starves its minimum-phase
factor, and descends past each target's ceiling from @floor-table only because
the factor has been starved away, not because the phase has been shaped. At the
ceiling the gap is plain: on the low-pass, 44.9 samples of latency leave the
factorisation at its full minimum-phase ripple of 1.117 samples where the
continuum is already at 0.419; on the LR4 crossover, 0.766 against 0.287.

A caller who wants flat group delay must therefore buy it with a prescribed
phase; what the factorisation offers instead is @what-it-is-for.

#figure(
  phase-regime-chart(reference-regimes),
  caption: [
    How each family spends latency above the floor, at $N = 129$ on a 1024-point
    grid. Both axes are normalised per target so six targets share one frame:
    latency is the fraction of the way from that target's own floor to linear
    phase, ripple is relative to its own minimum-phase ripple, so every curve
    starts at $(0, 1)$ and linear phase is $(1, 0)$. Solid: prescribed phase.
    Dashed: the alternating factorisation. The continuum is drawn to linear phase
    only; beyond it the family mirrors back to maximum phase.
  ],
) <phase-regimes>

= Below the floor: magnitude for delay <below-the-floor>

A controller with a hard latency budget under $tau_"min"$ has the opposite
problem, and @delay-floor says the only way out is to stop asking for the same
magnitude. The exchange is measurable directly: the magnitude-constrained
group-delay optimiser of Wu, Gao and Teo @wu2013 minimises weighted group delay
subject to a magnitude tolerance, so widening that tolerance asks exactly what a
magnitude concession buys in delay. @floor-probe-table walks it from a quarter of
a decibel to two.

#figure(
  text(size: 8.4pt)[
    #table(
      columns: 6,
      align: (left, right, right, right, right, right),
      table.header([Target], [$tau_"min"$], [0.25], [0.5], [1], [2]),
      [Room correction], [0.50], [0.47], [0.42], [0.32], [0.10],
      [Low-pass], [5.86], [5.40], [4.93], [3.93], [1.76],
      [Parametric EQ], [6.21], [5.75], [5.27], [4.25], [1.93],
      [Deep notch], [8.63], [8.59], [8.49], [8.17], [6.67],
    )
  ],
  caption: [
    Weighted mean group delay in samples reached by the low-group-delay optimiser
    as its magnitude tolerance widens (columns in dB), against each target's
    floor. 129 taps, 1024-point grid, four penalty stages of at most 80 L-BFGS
    steps. Every target shown buys delay below its floor for at most 1.90 dB of
    RMS magnitude error. The two crossovers are measured but omitted as the cases
    where the exchange fails: the LR8's magnitude is unrealisable at 129 taps, so
    with no accuracy left to concede it never gets under its floor, and the LR4's
    group delay is flat enough that 2 dB moves it 6.6%. Source:
    #code-path("docs/reference-phase-regimes.csv").
  ],
) <floor-probe-table>

The ladder stops at 2 dB because wider tolerances stop measuring anything. At
4 dB the optimiser has not converged and its answer tracks the iteration budget
rather than the tolerance — the low-pass reports 33.1, 59.8 or 2.5 samples under
three budgets. At 8 dB the constraint admits a spectral null, where group delay
is undefined, and every budget returns the same figure with the maximum magnitude
error saturated at its 60 dB clamp. Both are pinned by tests, so the endpoint is
a recorded limit rather than a choice of presentation.

The exchange is strongly target-dependent and strongly non-linear. Room
correction concedes the largest share of its floor, 80%, though that floor is
only half a sample to begin with; the parametric EQ concedes the most samples,
4.28 of them, and more than half of that in the last step alone; the deep notch
holds 95% of its floor through the first three rungs and gives up 23% in total.
The honest answer to a controller that cannot afford $tau_"min"$ is therefore
usually a shallower filter rather than a cleverer phase.

= What the construction is for <what-it-is-for>

The factorisation's advantage is not phase linearity, which @ripple-invariance
shows its budget cannot buy. It is latency avoided. A linear-phase filter of
latency $L$ has only $2L+1$ taps to spend on the magnitude, while a
minimum-phase-led design of $N$ taps spends all $N$ and carries only the latency
its own phase implies. Matching a 1025-tap minimum-phase-led design to within a
thousandth of a decibel costs linear phase 22 times the latency on the deep
notch, 19 on the parametric EQ, 18 on the LR4 crossover, 16 on the low-pass and
8 on the LR8 crossover; on room correction no sampled linear-phase latency
matches it at all. The price is the minimum-phase factor's group-delay deviation,
0.8 to 8.0 samples across these targets, which no budget reduces.

The budget itself is a function of the output support rather than of the
available latency. Swept over 129 to 1025 output taps, a non-zero budget saves
more than a decibel of RMS dB error on one of six targets and only below about
513 taps: the eighth-order crossover gains 57.19 dB at 129 taps and 23.05 dB at
257, then nothing. A longer filter behind a generous delay line therefore wants a
_smaller_ budget, not a larger one.

Two applications want opposite controls. An equaliser or room correction wants
the continuum of @phase-mix exposed directly — one parameter from minimum through
linear to maximum phase, with latency as the consequence rather than the
constraint. A loudspeaker controller, which already pays an alignment delay, wants
its floor first: the budget it can usefully spend is what remains after the
magnitude request has taken its share, and on a steep crossover that remainder is
often nothing.

The choice between methods is correspondingly conditional: the factorisation with
a selected budget for dB-sense magnitude fidelity at fixed support — never worse
than minimum-phase truncation there, leading RMS dB error on all six targets, and
reaching 3.310 dB on the one starved target against 6.901 dB for a fixed
16-sample budget — minimum-phase truncation for a linear-magnitude norm under
critical latency, prescribed phase for group-delay flatness at a stated latency,
and the phase-free optimiser where delay and coefficient range justify an
explicit magnitude tolerance.

= Failure modes

Each method has an input class it handles badly; all are regression-tested
alongside the successful cases.

- *Alternating correction.* A later pass can raise realised RMS magnitude error
  and amplify platform rounding. Treat the pass count as a maximum, reject the
  first rising candidate, and report accepted passes.

- *Support utilisation.* When the minimum-phase factor already fits the target
  the residual approaches a centred impulse and the result is a minimum-phase
  response plus delay. Pre-peak energy does _not_ detect this: on the LR4
  crossover the degenerate and $d = 0$ designs agree on it to four decimals.
  Compare against $d = 0$, or measure the symmetric factor's energy away from its
  centre tap. Selecting the budget by @delay-objective removes the mode.

- *Hand-picked budget.* Small non-zero budgets are worse than both endpoints, as
  above. Select it against a stated objective and report the candidate stride.

- *Low-delay optimisation.* A linear-phase start can remain in a substantially
  worse local basin than the default minimum-phase start. State the initial taps
  and the optimisation budget.

- *Zero-weight bins.* Unconstrained bins can diverge while the weighted band is
  fitted accurately. Prefer weak positive weights to omitted bands.

- *Group-delay metric.* Near a spectral null, phase differentiation is fragile
  and can dominate a whole-band statistic — a measurement limitation, not filter
  latency. Mask deep stopbands and publish the evaluated band and weight.

= Scope and reproducibility

The comparison uses six 257-tap prototypes at 48 kHz — low-pass, parametric EQ,
LR4 crossover, deep notch, measured room correction, and LR8 crossover — designed
into 129 output taps on a 1024-point grid. The length sweep instead rebuilds the
same curves as 2049-tap fixtures over 129 to 1025 output taps on an 8192-point
grid, and the two must not be read against each other. Every metric is recomputed
from the realised taps, never from the design grid.

Only one of the six targets starves the minimum-phase factor, so both the case
for a non-zero budget and the case against it above 513 taps rest on one fixture
of that class. The floors of @floor-table and the exchange of @floor-probe-table
are measured at one support and fall as the support grows, and the below-floor
ladder is bounded by the optimiser's convergence rather than by the physics.
Every target here is zero-phase, and perceptual evaluation is limited to
objective delay proxies.

Implementations, fixtures, and artifacts live in the public Go module
#code-path("github.com/cwbudde/algo-mixedphase"); every figure and table above is
rendered from a committed CSV, regenerated by `just compare` and byte-compared by
`just compare-check`. The full-length treatment adds the per-equation
reproducibility appendix, the six-method benchmark, and the attribution audit
against @budde2012.

#show bibliography: set text(size: 8.1pt)
#bibliography("references.bib", style: "ieee", title: "References")
