#import "charts.typ": (
  continuum-accuracy-chart, continuum-comparison-chart, continuum-impulse-chart,
  continuum-residual-chart, continuum-ripple-chart, continuum-subfloor-table,
  continuum-summary-table, continuum-window-chart, cross-target-summary-table,
  phase-regime-chart, signal-flow-diagram,
)
#import "style.typ": code-path, paper

#let revision = sys.inputs.at("revision", default: "working tree")
#let reference-results = csv("../reference-results.csv", row-type: dictionary)
#let reference-continuum = csv(
  "../reference-continuum.csv",
  row-type: dictionary,
)
#let reference-continuum-impulse = csv(
  "../reference-continuum-impulse.csv",
  row-type: dictionary,
)
#let reference-regimes = csv(
  "../reference-phase-regimes.csv",
  row-type: dictionary,
)

#show: paper.with(
  title: "One Knob Across the Phase Continuum",
  subtitle: "Fixed-support FIR design from sub-minimum delay to maximum phase",
  author: "Christian-W. Budde",
  revision: revision,
  abstract-body: [
    A latency-constrained filter designer has a number of samples in hand and a
    magnitude response to realise. The mixed-phase literature offers no parameter
    of that kind: a mix, a latency budget and a magnitude tolerance each move
    group delay, none of them measures it, and one of them does not shape phase at
    all. This paper presents a design parameterised directly by the requested
    group delay, and shows that most of the range needs no search. Prescribing a
    phase between the minimum-phase and linear-phase responses makes the realised
    mean group delay an affine function of the mix, so the mix that meets a
    request follows in closed form; across six reference targets the realised
    delay tracks the request to within 0.28 samples, and to within 0.08 on five of
    them. The same law bounds what phase alone can reach to a window
    $[tau_"min", N - 1 - tau_"min"]$ set by the magnitude, not by the tap count:
    127.0 samples wide for a room correction, 29.3 for an eighth-order crossover
    in the same 129 taps. Outside that window the paper swaps the roles of Wu,
    Gao and Teo's objective and constraint, minimising magnitude error subject to
    the requested delay rather than delay subject to a magnitude band, and reaches
    1.46 samples on a low-pass whose floor is 5.86 at 16.0% relative magnitude
    error, where the original formulation stops at 1.76 samples and 22.8%. Three
    properties of the resulting knob are measured and none is the monotone
    latency-accuracy trade the parameter's name suggests: group-delay ripple falls
    to exactly zero at the centre of the window, magnitude error is worst in the
    interior and best at the phase-pure ends by up to seven orders of magnitude,
    and both structures vanish when the requested magnitude does not fit the
    support. All implementations and measurements are maintained in a public Go
    repository so that each quantitative claim traces to a test or a committed
    comparison artifact.
  ],
  status-body: [
    Reviewed English author manuscript. Charts and tables are generated from
    committed reference CSVs; the tagged PDF is built by the repository release
    workflow.
  ],
)

= Introduction

A designer working under a latency budget knows one number: how many samples of
delay the signal path can afford. The magnitude response is given, the tap count
follows from the processing budget, and the question is what phase to ask for.

The tools available do not take that question in that form. A linear-phase FIR of
length $N$ preserves waveform alignment and costs exactly $(N-1)/2$ samples,
whether or not the budget allows it. A minimum-phase design concentrates energy as
early as the magnitude permits and reports whatever group delay results. Between
them the mixed-phase literature offers parameters — an interpolation mix, an
alternating factorisation's delay budget @budde2012, a magnitude tolerance
@wu2013 — each of which moves group delay without measuring it. Two of them
require a sweep to find out what a given latency costs, and the third does not
shape phase at all.

This paper reorganises fixed-support mixed-phase design around the requested
group delay itself. @continuum-and-law establishes the two results that make such
a parameter practical: the delay realised by a prescribed phase is an affine
function of the prescription, so the parameter inverts in closed form, and the
same relation bounds what phase alone can reach. @algorithm gives the resulting
algorithm, including the reformulation needed outside those bounds.
@what-it-costs measures what the parameter buys and what it charges, and
@comparison reads it against the methods it replaces. @failure-modes states the
conditions under which its structure disappears.

Two conventions hold throughout. All designs are of fixed support $N$, so a
latency comparison is never confounded by a tap-count difference; and group delay
is always a weighted mean over a stated band, because the quantity is not
meaningful across a spectral null.

= The continuum and the affine delay law <continuum-and-law>

== The group-delay floor a magnitude request implies <group-delay-floor>

The requested delay looks like a free parameter. It is not, and the constraint
that binds it organises everything that follows.

Every causal realisation of a given magnitude differs from the minimum-phase one
by an all-pass factor: $H(z) = H_"min" (z) A(z)$ with $|A(e^(j omega))| = 1$
@oppenheim1989. An all-pass response has non-negative group delay at every
frequency, so

$ tau_H (omega) >= tau_"min" (omega) quad "for all" omega. $ <delay-floor>

The minimum-phase group delay is therefore a floor, and it is a property of the
requested magnitude rather than a design choice. Asking for a steeper transition
or a deeper notch raises it whether or not the designer intended to spend the
latency. At 129 taps the six reference targets of this comparison have floors
spanning two orders of magnitude, from half a sample for a room correction to 49.4
for an eighth-order crossover, as the first column of @continuum-summary reports.

== Prescribing a phase, and what delay results

A prescribed-phase design interpolates between the two responses whose phase is
known in closed form. With $phi.alt_"min"$ the continuous minimum phase of the
requested magnitude and $mu$ the mix,

$
  phi.alt(mu, omega) = (1 - mu) phi.alt_"min" (omega) - mu omega (N-1)/2,
$ <phase-mix>

so $mu = 0$ is minimum phase, $mu = 1$ is linear phase and $mu = 2$ is maximum
phase: negating a real response's phase reverses its impulse response, and the
added full-length delay makes it causal again. The continuum closes at two rather
than at one.

The minimum phase itself is reconstructed from the requested magnitude either
cepstrally or through the discrete Hilbert transform of its log magnitude
@damera2000. The two agree to machine precision on a single reconstruction, and the
Hilbert route reproduces the target magnitude exactly because it computes only the
phase.

The phase must be the continuous branch, not one recovered from a spectrum. An
`atan2` reconstruction with a bin-to-bin unwrapper loses whole $2 pi$ turns
wherever the phase advances by more than $pi$ between neighbouring bins, which a
steep target routinely does; @phase-mix would then be wrong by $2 pi mu$ radians
at that bin. The endpoints $mu in {0, 1}$ happen to be immune because a whole turn
is invisible after exponentiation. Every value between them is not.

Group delay is $-d phi.alt \/ d omega$, and differentiation is linear, so the
group delay of the prescription is the same blend of the two endpoint delays:
$tau(mu, omega) = (1 - mu) tau_"min" (omega) + mu (N-1)/2$. Averaging over any
fixed weight preserves the blend, giving the relation this paper is built on,

$ tau(mu) = (1 - mu) tau_"min" + mu (N-1)/2. $ <affine-law>

Two consequences follow. The first is that the parameter inverts analytically:
the mix realising a requested delay $tau$ is

$ mu(tau) = (tau - tau_"min") / ((N-1)/2 - tau_"min"), $ <mix-inverse>

so meeting a request costs one design and no search. The second is that
@affine-law bounds what a prescribed phase can reach. Since $mu in [0, 2]$,

$ tau in [tau_"min", N - 1 - tau_"min"]. $ <window>

@window is centred on $(N-1)/2$ and its width is $N - 1 - 2 tau_"min"$, so it
narrows as the target's own floor rises. @window-figure draws it for the six
targets in a common 129 taps: a room correction keeps 127.0 of the 128 available
samples, while the eighth-order crossover keeps 29.3. That target's magnitude is
barely realisable in the support it has been given, and a magnitude that hard to
realise dictates its own latency.

#figure(
  continuum-window-chart(reference-continuum),
  caption: [
    The reachable window @window of each reference target at 129 taps on a
    1024-point grid. The coloured bar is the range of mean group delay a
    prescribed phase reaches; the grey ends are the regions only a magnitude
    concession opens. Every window is centred on linear phase at 64 samples, and
    its width is set by the requested magnitude rather than by the tap count —
    which is why the same 129 taps leave 127.0 samples of freedom for a room
    correction and 29.3 for the LR8 crossover.
  ],
) <window-figure>

@affine-law is exact for the prescription and approximate for the filter, because
the realised design is a least-squares projection of a prescribed complex response
onto $N$ causal taps. @residual isolates that residual. It stays under 0.08
samples on five targets and reaches 0.28 on the parametric EQ near the
maximum-phase end, where the minimum phase being interpolated is steepest. Against
windows over a hundred samples wide, a closed-form inverse is an adequate way to
meet a request, and the repository commits the predicted and achieved columns side
by side so the residual is a measured quantity rather than an assurance.

#figure(
  continuum-residual-chart(reference-continuum),
  caption: [
    Error of the affine delay law @affine-law: realised mean group delay minus the
    value @affine-law predicts, against the requested delay. The law is exact for
    the prescribed phase, so the whole of this figure is the projection onto 129
    taps. The curves are antisymmetric about linear phase because reversal negates
    phase, and the parametric EQ is the worst case at 0.28 samples.
  ],
) <residual>

= The algorithm <algorithm>

The design takes a requested delay $tau$ and dispatches on @window. The floor is
measured, not assumed: it is the weighted mean group delay of the realised $mu = 0$
design, so a request of exactly $tau_"min"$ is met exactly and the residual of
@residual is attributable to the projection alone.

/ Inside the window: @mix-inverse gives the mix, and one weighted least-squares
  projection of @phase-mix gives the filter. Only latency is spent.

/ Below the floor: @delay-floor says no phase choice reaches the request, so the
  magnitude gives way. @swapped-objective is solved instead.

/ Beyond maximum phase: the reflected request $N - 1 - tau$ lies below the floor,
  so the sub-floor solve runs on it and the result is reversed. Reversal maps a
  delay of $tau$ to $N - 1 - tau$ exactly, so one solver covers both tails.

== Below the floor: swapping the objective and the constraint <swapped>

Wu, Gao and Teo minimise weighted group delay subject to a magnitude band
@wu2013. Their parameter is therefore the tolerance, and the delay is an outcome —
the wrong assignment for a design parameterised by delay. This paper solves the
exchanged problem,

$
  min_h sum_k m_k (|H_k| - T_k)^2 quad "subject to" quad sum_k w_k tau_k = tau,
$ <swapped-objective>

with $T$ the target magnitude, $w$ the delay band and $m$ uniform over the grid.
Weighting the magnitude uniformly while the delay band masks stopbands is
deliberate: a magnitude objective that also masked them would let them drift
unpenalised.

Both forms are smooth and non-convex, and the same machinery serves them — a
limited-memory BFGS minimiser with Armijo backtracking inside a penalty ladder
that multiplies the constraint penalty by ten per stage. One structural difference
matters. The magnitude band of the original is one inequality per bin, whereas the
delay constraint of @swapped-objective is a single scalar over the whole grid, so
its residual is unknown until every bin has been visited. The gradient of the
weighted mean delay is therefore accumulated during the sweep and scaled by the
residual afterwards, which costs one vector the length of the filter and no second
sweep.

Convergence is to a local minimum, and the starting point chooses which one. The
minimum-phase design is used, since the linear-phase alternative is a
substantially worse basin that no descent direction leaves: escaping it would mean
moving a zero across the unit circle. The iteration count is likewise a second
delay-versus-accuracy dial rather than a convergence threshold, so every number
below is quoted with the budget that produced it.

== What the alternating factorisation's budget is, and is not <ripple-invariance>

One existing parameter looks like it belongs on this axis and does not. The
alternating minimum/linear-phase factorisation of @budde2012 splits a fixed
support between a minimum-phase factor and a symmetric factor of length $2d + 1$,
as @factor-signal-flow shows, and $d$ is widely read as a phase control.

#figure(
  signal-flow-diagram(),
  caption: [
    The alternating construction of @budde2012: a minimum-phase factor and a
    symmetric linear-phase factor share one fixed support, redrawn from Figures 2
    and 3 of that paper.
  ],
) <factor-signal-flow>

The symmetric factor has exactly linear phase and therefore exactly constant group
delay. Group delays add along a cascade, so the deviation of the product from
constant group delay is the minimum-phase factor's deviation alone,

$ rho(h) = rho(a), $ <ripple-identity>

with $rho$ the weighted RMS group-delay deviation. The all-pass the budget inserts
is $z^(-d)$: it translates the group-delay curve and does not flatten it. Across
all six targets the measured ripple is identical to nine decimal places at every
budget the split admits. The budget is a magnitude-recovery control, and
@phase-regimes reads the two families against each other: at 44.9 samples of
latency on the low-pass the factorisation still carries its full minimum-phase
ripple of 1.117 samples where the prescribed continuum is down to 0.419, and the
LR4 crossover tells the same story at 0.766 against 0.287.

#figure(
  phase-regime-chart(reference-regimes),
  caption: [
    How each family spends latency above the floor, at 129 taps on a 1024-point
    grid, with both axes normalised per target so six targets share one frame:
    latency is the fraction of the way from that target's floor to linear phase,
    and ripple is relative to its own minimum-phase ripple, so every curve starts
    at $(0, 1)$ and linear phase is $(1, 0)$. Solid curves prescribe a phase and
    convert latency into flatness almost proportionally. Dashed curves are the
    alternating factorisation, which holds its ripple until the budget starves its
    minimum-phase factor and only then descends.
  ],
) <phase-regimes>

= What the knob buys and what it charges <what-it-costs>

== Group-delay ripple falls monotonically to zero

The quantity the parameter actually shapes is group-delay flatness.
@ripple-figure measures it across the window. Every target descends monotonically
from its minimum-phase ripple to exactly zero at the centre — the linear-phase
design, symmetric by construction — and rises again by reflection. The measured
value at the centre is below $10^(-13)$ samples on all six targets, so this is an
identity of the construction rather than a numerical coincidence.

The descent is close to proportional, which is what makes the parameter useful: a
caller who can afford half the distance from the floor to linear phase gets
roughly half the flatness. It is also the axis on which the parameter differs from
the budget of @ripple-invariance, whose ripple does not respond at all.

#figure(
  continuum-ripple-chart(reference-continuum),
  caption: [
    Weighted RMS group-delay deviation against requested delay, in-window rows
    only. Each curve starts at its target's minimum-phase ripple, falls
    monotonically to zero at linear phase and mirrors back. Sub-floor rows are
    excluded: their ripple runs to 222 samples on the LR8 crossover and would
    flatten everything else against the axis. The short steep curve is that same
    LR8 crossover, whose window is only 29.3 samples wide.
  ],
) <ripple-figure>

== Magnitude accuracy is worst in the interior

The accuracy curve does not follow the ripple curve, and this is the result most
likely to surprise a caller. @accuracy-figure measures relative magnitude error
across the whole causal range.

#figure(
  continuum-accuracy-chart(reference-continuum),
  caption: [
    Relative magnitude error against requested delay, over ten decades. Curves are
    in-window rows; filled circles are the two optimised branches. The phase-pure
    ends of each window are its most accurate points, the interior is more
    expensive by up to seven orders of magnitude, and both tails cost far more
    still. The near-flat curve across the middle is the LR8 crossover, whose
    magnitude does not fit 129 taps and which is therefore almost indifferent to
    phase.
  ],
) <accuracy-figure>

Three statements hold on every target, and @continuum-summary quantifies them.

First, *the ends of the window are its most accurate points*, not merely its
fastest and slowest. A minimum-phase or maximum-phase design is a spectral factor
of the requested magnitude and needs no compromise to realise it; every
intermediate phase must be approximated on the same support. The penalty for
leaving an end is a factor of $1.6 times 10^7$ on the LR4 crossover and
$9.9 times 10^5$ on the low-pass, whose minimum-phase responses fit their support
almost exactly, and a factor of 12 on the room correction, whose does not.

Second, *the error is exactly symmetric about linear phase*, since reversal cannot
change a magnitude. Linear phase is therefore always a stationary point of the
accuracy curve — but it is a local minimum for some targets and a local maximum
for others, so it cannot be recommended or dismissed on accuracy grounds in
general. What can be said is that it is the only point where ripple vanishes, and
that it always costs the full $(N-1)/2$ of latency.

Third, *a target that does not fit its support has neither structure*. The LR8
crossover's error spans 1.2% to 1.5% across its entire continuum, a ratio of 1.3
where the other five span factors of 12 to $1.6 times 10^7$. Truncation error
dominates every phase choice. This is the same condition that collapses its window
to 29.3 samples, so a narrow window is a usable warning that the accuracy
structure is absent too, and that the phase parameter is not the constraint worth
attending to.

#figure(
  continuum-summary-table(reference-continuum),
  caption: [
    Per target at 129 taps on a 1024-point grid: the group-delay floor
    $tau_"min"$ in samples, the width of the reachable window @window in samples,
    the relative magnitude error at the window ends $E_"end"$, the worst error
    sampled in its interior $E_"peak"$, and their ratio. Every cell is computed at
    build time from #code-path("docs/reference-continuum.csv"). The ratio measures
    how much of the magnitude the phase choice costs, and it collapses to 1 on the
    one target whose magnitude does not fit the support.
  ],
) <continuum-summary>

== Below the floor, the exchange is expensive and worth stating

@subfloor-table gives the most aggressive request the artifact samples — a quarter
of each target's floor — and what it cost. Two things should be read from it.

The magnitude concession is modest in relative terms on three targets — 5.3% for the
room correction, 5.7% for the deep notch and 6.6% for the parametric EQ — larger at
16.0% for the low-pass, and severe on the two crossovers at 19.6% and 57.5%. The
stopband-sensitive dB figure moves further and in a different order, reaching 21.7 dB
on the LR4 crossover and 69.2 dB on the LR8, because the optimiser spends stopband
depth first. Reporting only one of the two would misstate the trade in either
direction.

The group-delay ripple is where the real price sits. Driving the low-pass from
5.86 samples to 1.46 raises its ripple from 1.117 to 27.2 samples, and the
parametric EQ reaches 69.3. Sub-floor designs are fast in the mean and badly
non-flat, which is worth saying plainly: below the floor the design is not
approaching minimum phase, it is leaving the class of well-behaved phase responses
altogether.

#figure(
  continuum-subfloor-table(reference-continuum),
  caption: [
    The exchange below the floor at the most aggressive sampled request, a quarter
    of each target's $tau_"min"$: realised mean group delay $tau$, relative
    magnitude error, stopband-sensitive RMS dB error, and weighted group-delay
    ripple, all in samples or as marked. Computed at build time from
    #code-path("docs/reference-continuum.csv") with the published optimiser budget
    of 80 iterations over four penalty stages.
  ],
) <subfloor-table>

== The time domain

@impulse-figure shows what the parameter does to the impulse response of one
target. The energy migrates from the head of the filter to its tail as the
requested delay rises, and the last lane is the second lane read backwards — the
maximum-phase design is the time reverse of the minimum-phase one, which is the
statement of @phase-mix at $mu = 2$ made visible. The sub-floor lane at the top is
the one that is not a reversal of anything: it realises a different magnitude.

#figure(
  continuum-impulse-chart(
    reference-continuum-impulse,
    target: "low-pass",
  ),
  caption: [
    Impulse responses of the low-pass at six requested delays, normalised to unit
    peak and sharing one amplitude scale. Reading down, the energy moves from the
    head of the support to its tail; the 122.1-sample lane is the 5.9-sample lane
    reversed. The 2.9-sample lane at the top is sub-floor and realises a magnitude
    16.0% away from the request.
  ],
) <impulse-figure>

= Comparison <comparison>

@comparison-lowpass and @comparison-steep put the parameter's own curve against
the fixed points the other methods reach on the same target. The asymmetry of the
comparison is structural and should be stated: the delay is an input to this
design and an outcome of every other, so one is a curve and the rest are points.

#figure(
  continuum-comparison-chart(
    reference-results,
    reference-continuum,
    target: "low-pass",
  ),
  caption: [
    Low-pass at 129 taps: relative magnitude error against mean group delay. The
    solid curve is the requested-delay parameter across its window, the filled
    circles its two optimised branches, and the markers are the six published
    methods. The alternating construction at $d = 16$ sits three orders of
    magnitude below the curve at the same latency, because it spends its budget as
    pure delay and keeps the minimum-phase magnitude; what it does not do is
    flatten group delay.
  ],
) <comparison-lowpass>

The six comparison methods are those the repository publishes: the alternating
factorisation at a fixed and at a selected budget @budde2012, the prescribed-phase
projection, a weighted complex least-squares fit refined towards minimax by
Lawson's reweighting @lawson1961, minimum-phase truncation, and the
low-group-delay optimisation of @wu2013.

*Inside the window the parameter is not the accurate choice, and that is the
  trade.* On the low-pass at about 21 samples of latency, the alternating
construction reaches $1.0 times 10^(-6)$ relative error against the continuum's
$2.2 times 10^(-3)$ — better by a factor of 2200 — while carrying the full
minimum-phase ripple of 1.117 samples where the continuum has cut it to 0.889.
Neither number dominates the other. A caller who wants a magnitude realised at a
given latency should use the budget; a caller who wants group delay flattened
should prescribe phase, and pay for it. What the requested-delay parameter adds is
that the latency is specified rather than discovered, and that the flatness it buys
is proportional to what is spent.

*Below the floor the reformulation wins on its own terms.* On the low-pass,
@swapped-objective reaches 1.46 samples at 16.0% relative error and 0.58 dB RMS,
where the Wu–Gao–Teo formulation at a 2 dB tolerance stops at 1.76 samples with
22.8% and 1.90 dB. Faster and more accurate, from the same optimiser and the same
starting point, because the delay is the constraint rather than the objective. The
price is again ripple: 27.2 samples against 6.68.

#figure(
  continuum-comparison-chart(
    reference-results,
    reference-continuum,
    target: "steep-crossover",
    x-max: 96,
    y-min: 0.0001,
  ),
  caption: [
    LR8 crossover at 129 taps, the one target whose magnitude does not fit its
    support. The window is the short flat segment; the sub-floor branch on the left
    reaches 37.0 samples at 0.48% relative error, better on delay and on magnitude
    than the minimum-phase design at 49.4 samples and 1.23%. Note the axis
    difference from @comparison-lowpass: this target reaches no error below
    $10^(-4)$ by any method.
  ],
) <comparison-steep>

*On a support-starved target the floor guards a magnitude that was never
  realisable.* @comparison-steep makes the sharpest case for the reformulation.
Minimum-phase truncation on the LR8 crossover gives 1.23% relative error at 49.4
samples; @swapped-objective at a request of 37.0 samples gives 0.48% relative
error and 48.2 dB RMS, improving on all three of relative error, dB error and
latency simultaneously. @delay-floor is not violated — the realised filter has a
different, lower-floor magnitude — but it is not a useful bound here, because the
magnitude defining it could not be built in 129 taps to begin with. The published
adaptive factorisation still wins the stopband outright at 3.31 dB RMS, and a
caller who needs rejection rather than latency should use it.

#figure(
  text(size: 7.6pt)[#cross-target-summary-table(reference-results)],
  caption: [
    Targets on which each published method is best, of six, computed at build time
    from #code-path("docs/reference-results.csv"). The table is retained from the
    method comparison this paper supersedes so that the requested-delay parameter
    is not read as displacing methods it does not beat.
  ],
) <cross-target-summary>

= Failure modes and validity limits <failure-modes>

The results above are conditional on their stated targets, weights and budgets.
Each row below has an observable symptom and a control; all are regression-tested
alongside the successful cases.

#figure(
  text(size: 7.25pt)[
    #table(
      columns: (0.72fr, 1.08fr, 1.2fr),
      align: (left, left, left),
      table.header(
        [*Mechanism*], [*Observable failure*], [*Control and interpretation*]
      ),
      [Requested-delay parameter],
      [
        On a support-starved target the accuracy structure of
        @accuracy-figure is absent: the LR8 crossover spans a factor of 1.3
        across its whole continuum.
      ],
      [
        Read the window width of @window-figure first. A window far below
        $N - 1$ means the magnitude, not the phase, is the binding constraint.
      ],

      [Affine law inverse],
      [
        The law is exact for the prescription and approximate for the filter;
        the residual reaches 0.28 samples on the parametric EQ.
      ],
      [
        Report @residual alongside any requested delay. Where a request must
        be met more tightly than the residual, correct it with one secant step
        rather than assuming @mix-inverse.
      ],

      [Prescribed phase, interior],
      [
        Interpolating a wrapped phase is wrong by $2 pi mu$ radians at any bin
        where the true phase has passed $pi$. The endpoints are immune; the
        interior is not.
      ],
      [
        Take the continuous phase from the reconstruction, never from an
        `atan2` unwrapper. Steep and high-order targets are where this bites.
      ],

      [Sub-floor solve],
      [
        A local optimum, and sensitive to the request: the two tails of the
        parametric EQ report 6.6% and 6.1% for the same reflected request when
        the target delay differs in its last bits.
      ],
      [
        Quote the optimiser budget with the result and treat sub-floor numbers
        as achieved, not optimal. State #code-path("InitialTaps") when the
        basin matters.
      ],

      [Sub-floor ripple],
      [
        Mean delay falls while flatness collapses: 27.2 samples of ripple on
        the low-pass at 1.46 samples of delay, 69.3 on the parametric EQ.
      ],
      [
        Report ripple with every sub-floor delay. A low mean group delay is
        not a well-behaved phase response.
      ],

      [Alternating budget],
      [
        Read as a phase control it does nothing: @ripple-identity holds to
        nine decimal places. A small non-zero budget is also worse than both
        endpoints, reaching 77.5% relative error at $d = 1$ on the steep
        crossover.
      ],
      [
        Treat it as a magnitude-recovery control and select it against a
        stated objective rather than defaulting it.
      ],

      [Group-delay metric],
      [
        Near a spectral null, phase differentiation is fragile and can
        dominate a whole-band statistic.
      ],
      [
        Mask or downweight deep stopbands, and publish the evaluated band and
        weight rather than a whole-band curve. The requested delay is only as
        meaningful as this band.
      ],
    )
  ],
  caption: [
    Failure modes, observable symptoms and required controls. The entries are
    qualitative; their executable regression evidence is mapped in
    @reproducibility.
  ],
) <failure-mode-table>

= Reproducibility appendix <reproducibility>

Every figure, table and numbered equation above is generated from a committed
artifact or pinned by a named test. Configuration fields not named here keep their
documented zero-value defaults, and the build embeds the repository revision.

== Operating point

All measurements are at 48 kHz, 129 taps, a 1024-point design grid, and — where an
optimiser is involved — 80 iterations over four penalty stages. The alternating
rows use 12 correction passes and the minimax rows 16 reweighting passes. The
continuum artifact samples 17 points across each window and three on each side of
it, at a quarter, a half and three quarters of the distance from the window edge to
the causal limit. The ladder stops at three quarters because driving the mean delay
to zero requires the magnitude to collapse, and what would be recorded past that
point is the iteration budget rather than the trade.

== Artifacts

#code-path("docs/reference-continuum.csv") and
#code-path("docs/reference-continuum-impulse.csv") carry the requested-delay rows
and the impulse snapshots; #code-path("docs/reference-results.csv") and
#code-path("docs/reference-phase-regimes.csv") carry the published method
comparison and the two-family regime scan. All four are regenerated by
`just compare` and byte-gated by `just compare-check`. Machine-dependent timings
are confined to #code-path("docs/reference-timings.csv"), which no figure reads.

== Claims and their regressions

Justification is disabled in @regression-table because Go test names do not
hyphenate.

#figure(
  block(width: 100%)[
    #set par(justify: false)
    // The longest Go test names are 56 characters and cannot break, so the
    // monospace face has to be small enough for one to fit the column.
    #show raw: set text(size: 5.6pt)
    #text(size: 7.1pt)[
      #table(
        columns: (0.5fr, 1.5fr),
        align: (left, left),
        table.header([*Claim*], [*Regression*]),
        [@affine-law, @mix-inverse],
        [
          `TestContinuumDelayLawMatchesTheAffinePrediction` and
          `TestContinuumAffineLawResidualIsSmall` bound the residual at 0.3
          samples. `TestContinuumKnobHitsItsRequestedDelay` and
          `TestContinuumRequestIsMetOnEveryRow` check that the request is met,
          to 0.3 samples in-window and $10^(-3)$ outside it.
        ],

        [@window, @window-figure],
        [
          `TestContinuumReachableWindowMatchesTheFloor` pins the dispatch,
          `TestContinuumWindowEdgesAreTheFloorAndItsReflection` the edges, and
          `TestContinuumWindowNarrowsWithTheFloor` the ordering across targets.
        ],

        [Reflection symmetry],
        [
          `TestContinuumEndpointsAreTimeReversals` and
          `TestContinuumImpulseReversesAtMaximumPhase` pin the time-domain
          identity to $10^(-9)$ of peak.
          `TestContinuumWindowIsSymmetricAboutLinearPhase` pins the in-window
          magnitude symmetry, and
          `TestOutOfWindowBranchesAreOnlyApproximatelySymmetric` records that
          the two optimised tails agree only approximately.
        ],

        [@ripple-figure],
        [
          `TestContinuumRippleVanishesAtLinearPhaseOnly` pins both the monotone
          descent and that zero is reached only at the centre;
          `TestContinuumCentreIsLinearPhase` pins the symmetry of the filter
          there.
        ],

        [@accuracy-figure, @continuum-summary],
        [
          `TestContinuumEndpointsAreTheMostAccuratePoints` and
          `TestContinuumIsMostAccurateAtThePhasePureEndpoints` pin the endpoint
          result, `TestContinuumErrorIsSymmetricAboutLinearPhase` the symmetry,
          and `TestSupportStarvedTargetIsIndifferentToPhase` with
          `TestSupportStarvedTargetFlattensTheContinuum` the exception.
        ],

        [@swapped-objective, @subfloor-table],
        [
          `TestSubFloorSolveBuysDelayBelowTheFloor` and
          `TestSuperMaximumSolveBuysDelayBeyondMaximumPhase` pin that both tails
          move the delay;
          `TestDesignLowGroupDelayIsUnchangedByTheNewProblemForm` pins that the
          original formulation is untouched by the addition.
        ],

        [@ripple-identity, @delay-floor],
        [
          `TestAdaptiveDelayBudgetCannotFlattenGroupDelay`,
          `TestFactorisationHoldsItsRippleWhileTheContinuumDescends` and
          `TestZeroDelayDesignSitsOnTheMinimumPhaseFloor`.
        ],

        [Artifacts],
        [
          `TestCommittedContinuumCSVIsReproducible`,
          `TestCommittedContinuumImpulseCSVIsReproducible` and their
          counterparts for the older artifacts.
        ],
      )
    ]
  ],
  caption: [
    Each quantitative claim of this paper and the named test that pins it. Test
    names are as they appear in the repository's #code-path("mixedphase") and
    #code-path("internal/reference") packages.
  ],
) <regression-table>

= Limitations and open work

The requested delay is a weighted mean, so it says nothing about the shape of the
group-delay curve beyond the ripple figure reported alongside it. A parameter over
a peak or a band-limited maximum would be a different and probably harder design.

@mix-inverse is exact for the prescription only. The 0.28-sample worst case is
adequate for the applications this comparison targets, but a design needing tighter
control would have to close the loop with a secant step on the measured delay.

The sub-floor branch is a local optimisation, and this paper does not establish how
far its results are from optimal. The two tails disagreeing at the most aggressive
request is direct evidence that they are not. A global or multi-start treatment of
@swapped-objective would settle what a magnitude concession can actually buy.

Finally, the whole comparison is at one support and one grid. The delay-budget
behaviour of the alternating construction is known to depend strongly on the output
length, and there is no reason to assume the accuracy structure of
@accuracy-figure is length-invariant either.

= Conclusion

Parameterising a fixed-support mixed-phase design by its requested group delay
turns out to be mostly free. Prescribing a phase between the minimum-phase and
linear-phase responses makes the realised mean delay affine in the prescription, so
the request inverts in closed form and is met to within 0.28 samples across six
reference targets. The same relation says what phase alone can deliver: a window
$[tau_"min", N-1-tau_"min"]$ that the requested magnitude sets and the tap count
does not, spanning 127.0 samples for one target and 29.3 for another in the same
129 taps. Outside it, exchanging Wu, Gao and Teo's objective and constraint reaches
delays their formulation does not, at better accuracy on the targets measured here.

What the parameter does not do is trade latency for accuracy monotonically. The
phase-pure ends of the window are its most accurate points as well as its fastest
and slowest, the interior costs up to seven orders of magnitude more, and both
structures disappear on a target whose magnitude does not fit its support — where
the window width gives the caller advance warning. A latency-constrained designer
can now state the constraint directly, and read what it costs from a figure rather
than from a sweep.

// The bibliography is set a little smaller so the fifth entry stays on the same
// page as the other four.
#show bibliography: set text(size: 8.5pt)
#bibliography("references.bib", style: "ieee", title: "References")
