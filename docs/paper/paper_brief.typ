#import "charts.typ": (
  continuum-accuracy-chart, continuum-ripple-chart, continuum-subfloor-table,
  continuum-summary-table, continuum-window-chart,
)
#import "style.typ": code-path, paper

#let revision = sys.inputs.at("revision", default: "working tree")
#let reference-continuum = csv(
  "../reference-continuum.csv",
  row-type: dictionary,
)

// A short version of paper.typ, built from the same artifact so the two cannot
// disagree on a number. Body text is tightened to 8.8 pt on 0.48 em leading,
// which is what keeps this to four pages rather than six.
#show: paper.with(
  title: "One Knob Across the Phase Continuum",
  subtitle: "A short account of delay-parameterised fixed-support FIR design",
  author: "Christian-W. Budde",
  revision: revision,
  abstract-body: [
    A latency-constrained designer has a number of samples in hand, but no
    mixed-phase design parameter measures group delay: a mix, a latency budget and
    a magnitude tolerance each move it, and one of them does not shape phase at
    all. Prescribing a phase between the minimum-phase and linear-phase responses
    makes the realised mean group delay affine in the prescription, so a requested
    delay inverts in closed form and is met to within 0.28 samples across six
    reference targets. The same relation bounds what phase alone reaches to a
    window $[tau_"min", N - 1 - tau_"min"]$ that the requested magnitude sets:
    127.0 samples wide for a room correction and 29.3 for an eighth-order crossover
    in the same 129 taps. Outside it, exchanging the objective and the constraint of
    the low-group-delay formulation reaches 1.46 samples on a low-pass whose floor is
    5.86, at 16.0% relative magnitude error against 22.8% at 1.76 samples for the
    original. The resulting parameter does not trade latency for accuracy
    monotonically: its window ends are its most accurate points as well as its
    fastest and slowest.
  ],
)

#set text(size: 8.8pt)
#set par(leading: 0.48em)

= The floor, and the window it implies <window-section>

Every causal realisation of a magnitude differs from the minimum-phase one by an
all-pass factor, whose group delay is non-negative @oppenheim1989, so

$ tau_H (omega) >= tau_"min" (omega) quad "for all" omega. $ <delay-floor>

The floor is a property of the requested magnitude, not a design choice. A
prescribed-phase design interpolates the two responses whose phase is known in
closed form, using the *continuous* minimum phase @damera2000 rather than an
unwrapped one:

$
  phi.alt(mu, omega) = (1 - mu) phi.alt_"min" (omega) - mu omega (N-1)/2,
$ <phase-mix>

with $mu = 0$ minimum phase, $mu = 1$ linear phase and $mu = 2$ maximum phase, the
time reverse of $mu = 0$. Group delay is $-d phi.alt \/ d omega$ and
differentiation is linear, so the prescription's weighted mean delay is the same
blend of the endpoint delays,

$ tau(mu) = (1 - mu) tau_"min" + mu (N-1)/2, $ <affine-law>

which inverts analytically: $mu(tau) = (tau - tau_"min")\/((N-1)\/2 - tau_"min")$.
Meeting a requested delay therefore costs one design and no search. Since
$mu in [0, 2]$, the same relation bounds what phase alone reaches,

$ tau in [tau_"min", N - 1 - tau_"min"]. $ <window>

@window is centred on $(N-1)/2$, and its width $N - 1 - 2 tau_"min"$ is set by the
magnitude rather than by the tap count. @window-figure shows the spread: the same
129 taps leave 127.0 samples of freedom for a room correction and 29.3 for the LR8
crossover, whose magnitude is barely realisable in the support it was given.

@affine-law is exact for the prescription and approximate for the realised filter,
which is a least-squares projection onto $N$ taps. That residual stays under 0.08
samples on five targets and reaches 0.28 on the parametric EQ, against windows over
a hundred samples wide.

#figure(
  continuum-window-chart(reference-continuum),
  caption: [
    The reachable window @window at 129 taps on a 1024-point grid. The coloured bar
    is the range a prescribed phase reaches; the grey ends are the regions only a
    magnitude concession opens.
  ],
) <window-figure>

= What the parameter buys, and what it charges

*Ripple falls monotonically to zero.* @ripple-figure measures weighted RMS
group-delay deviation across the window. Every target descends from its
minimum-phase ripple to below $10^(-13)$ samples at the centre — the linear-phase
design — and mirrors back. The descent is close to proportional, so half the
latency from the floor to linear phase buys roughly half the flatness. This is the
axis on which the parameter differs from the alternating factorisation's delay
budget @budde2012, whose all-pass factor is $z^(-d)$: it translates the group-delay
curve without flattening it, and the measured ripple is identical to nine decimal
places at every budget the split admits. That budget is a magnitude-recovery
control, not a phase control.

#figure(
  continuum-ripple-chart(reference-continuum),
  caption: [
    Weighted RMS group-delay deviation against requested delay, in-window rows
    only. Sub-floor rows reach 222 samples on the LR8 crossover and are excluded.
    The short steep curve is that same target, whose window is 29.3 samples wide.
  ],
) <ripple-figure>

*Accuracy is worst in the interior.* @accuracy-figure and @continuum-summary give
the result most likely to surprise a caller. The ends of each window are its most
accurate points, not merely its fastest and slowest: a minimum-phase or
maximum-phase design is a spectral factor of the requested magnitude and needs no
compromise, where every intermediate phase must be approximated on the same
support. Leaving an end costs a factor of $1.6 times 10^7$ on the LR4 crossover and
12 on the room correction. The error is exactly symmetric about linear phase, since
reversal cannot change a magnitude, so linear phase is always a stationary point of
the accuracy curve — a local minimum for some targets and a local maximum for
others, and therefore not recommendable on accuracy grounds in general.

Both structures disappear when the magnitude does not fit the support. The LR8
crossover spans a factor of 1.3 across its whole continuum where the others span 12
to $1.6 times 10^7$, because truncation dominates every phase choice. It is the
same condition that collapses its window, so a narrow window in @window-figure is
advance warning that the phase parameter is not the binding constraint.

#figure(
  continuum-accuracy-chart(reference-continuum),
  caption: [
    Relative magnitude error against requested delay, over ten decades. Curves are
    in-window rows; filled circles are the two optimised branches.
  ],
) <accuracy-figure>

#figure(
  continuum-summary-table(reference-continuum),
  caption: [
    Per target at 129 taps: the floor $tau_"min"$, the window width, the relative
    magnitude error at the window ends and the worst sampled interior value, and
    their ratio. Computed at build time from
    #code-path("docs/reference-continuum.csv").
  ],
) <continuum-summary>

= Outside the window, magnitude gives way

@delay-floor says no phase choice reaches below the floor. Wu, Gao and Teo minimise
weighted group delay subject to a magnitude band @wu2013, which makes the tolerance
the parameter and the delay an outcome — the wrong assignment here. Exchanging the
two,

$
  min_h sum_k m_k (|H_k| - T_k)^2 quad "subject to" quad sum_k w_k tau_k = tau,
$ <swapped-objective>

meets the request exactly and makes the magnitude the quantity kept low. The same
penalty ladder and limited-memory BFGS minimiser serve both forms; the one
structural difference is that the delay constraint is a single scalar over the grid,
so its residual is accumulated during the sweep and applied afterwards. Requests
beyond maximum phase are served by solving the reflected request and reversing the
result, since reversal maps $tau$ to $N - 1 - tau$ exactly.

On the low-pass this reaches 1.46 samples at 16.0% relative error and 0.58 dB RMS,
where the original formulation at a 2 dB tolerance stops at 1.76 samples with 22.8%
and 1.90 dB — faster and more accurate from the same optimiser and starting point.
On the LR8 crossover, a request of 37.0 samples gives 0.48% relative error and 48.2
dB RMS against minimum-phase truncation's 1.23% at 49.4 samples, improving on
latency and on both magnitude measures at once: the floor there guards a magnitude
that 129 taps could not realise anyway.

@subfloor-table gives the price. The relative concession is modest on four targets
and the stopband-sensitive dB figure moves sharply, but the real cost is flatness —
driving the low-pass from 5.86 samples to 1.46 raises its ripple from 1.117 to 27.2
samples. Below the floor a design is not approaching minimum phase; it is leaving
the class of well-behaved phase responses.

#figure(
  continuum-subfloor-table(reference-continuum),
  caption: [
    The exchange below the floor at the most aggressive sampled request, a quarter
    of each target's $tau_"min"$, with the published budget of 80 iterations over
    four penalty stages.
  ],
) <subfloor-table>

= Failure modes

- *A narrow window means the magnitude binds, not the phase.* Read @window-figure
  before choosing a delay; on a support-starved target the accuracy structure above
  is absent.
- *The affine inverse is approximate.* Report the residual with any requested
  delay, and close the loop with one secant step where a request must be met more
  tightly than 0.28 samples.
- *Interior mixes need the continuous phase.* An `atan2` unwrapper loses whole
  $2 pi$ turns on a steep target, making @phase-mix wrong by $2 pi mu$ radians
  there. Only $mu in {0, 1}$ are immune.
- *Sub-floor results are local optima.* The two tails of the parametric EQ report
  6.6% and 6.1% for the same reflected request. Quote the optimiser budget and treat
  the numbers as achieved rather than optimal.
- *Group delay is not a whole-band quantity.* Near a spectral null, phase
  differentiation is fragile. Publish the evaluated band and weight; the requested
  delay is only as meaningful as that band.

= Scope and reproducibility

All measurements are at 48 kHz, 129 taps and a 1024-point grid, with 80 optimiser
iterations over four penalty stages where an optimiser is involved. Figures and
tables are generated at build time from #code-path("docs/reference-continuum.csv"),
which `just compare` regenerates and `just compare-check` byte-gates. The full
account, including the comparison against the six published methods, the Lawson
minimax path @lawson1961 and the mapping from each claim to its regression test, is
in the companion paper.

#show bibliography: set text(size: 8.1pt)
#bibliography("references.bib", style: "ieee", title: "References")
