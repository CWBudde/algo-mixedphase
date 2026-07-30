// Package mixedphase designs finite impulse response filters whose delay and
// pre-ringing lie between the minimum- and linear-phase extremes.
//
// The package contains six complementary design methods:
//
//   - [DesignIterative] implements the alternating factorisation proposed by
//     Christian-W. Budde at DAGA 2012. It factors a target magnitude response
//     into a causal minimum-phase part and a short linear-phase residual while
//     repeatedly compensating the truncation error of each part.
//   - [DesignIterativeAuto] runs that same factorisation but selects the delay
//     budget instead of accepting one. It exists because a hand-picked budget is
//     the factorisation's main practical hazard, in both directions: too large a
//     budget on a target that does not need one buys nothing but latency, and a
//     small non-zero budget on a target that does need one is worse than either
//     extreme.
//   - [DesignPhaseInterpolation] constructs a complex target response by
//     interpolating between minimum and linear phase, then projects it onto a
//     finite causal support. It is useful as a simple comparison baseline.
//   - [DesignComplexLeastSquares] approximates that same complex target under a
//     frequency weight, optionally reweighted towards the minimax solution.
//   - [DesignLowGroupDelay] prescribes no phase at all. It minimises the
//     weighted passband group delay subject to a magnitude tolerance, which is
//     the formulation Wu, Gao and Teo use.
//   - [DesignContinuum] takes the quantity a latency-constrained caller actually
//     has — a number of samples of group delay — and selects the mechanism that
//     serves it. It is the only entry point whose parameter is measured in the
//     units the constraint is expressed in.
//
// All six operate on a real prototype impulse response. Its magnitude
// response is the design target; its original phase is not otherwise used.
//
// # One parameter across the whole continuum
//
// The other five entry points each expose a different parameter, and none of them
// is a group delay: a mix, a latency budget, a magnitude tolerance. Two of them
// need a sweep before a caller learns what a given latency costs, and the budget
// of [DesignIterative] does not shape phase at all. [DesignContinuum] exists
// because that is an awkward way to meet a latency constraint.
//
// Two facts make a delay-valued parameter practical. The first is that the
// prescribed phase of [prescribedResponse] is linear in its mix, and group delay
// is a linear functional of phase, so the realised weighted mean delay is affine
// in the mix,
//
//	tau(mix) = (1-mix)*tau_min + mix*(Length-1)/2
//
// which inverts in closed form: no search is needed inside the range this covers.
// Across the six reference targets the realised delay tracks the request to within
// 0.28 samples, which is the error of projecting the prescription onto a finite
// support and is committed as a column of docs/reference-continuum.csv.
//
// The second is that the same relation bounds what phase alone can do. A mix is
// confined to [0, maximumPhaseMix], so the reachable delays are
//
//	[tau_min, Length-1-tau_min]
//
// a window centred on linear phase whose width the requested magnitude sets rather
// than the tap count. At 129 taps the reference targets range from 127.0 samples of
// freedom for a room correction to 29.3 for an eighth-order crossover. Outside the
// window phase choice is exhausted and only the magnitude can move, which is the
// swapped problem form of [lowDelayProblem.evaluateMatchDelay].
//
// Two properties of the parameter are worth knowing before turning it. Group-delay
// ripple falls monotonically to exactly zero at the centre of the window, but
// magnitude error does not follow it: the phase-pure ends of the window are its
// most accurate points as well as its fastest and slowest, because a spectral
// factor of the target needs no compromise where an intermediate phase must be
// approximated on the same taps. And both structures vanish when the requested
// magnitude does not fit the support, so a narrow window is a warning that there
// is nothing to choose between phases either.
//
// # Prescribed phase versus optimised phase
//
// The methods differ in what they treat as given. [DesignIterative] derives the
// phase distribution from a latency budget: the caller states a delay, and the
// split between the two factors follows. [DesignIterativeAuto] moves that one
// step further and treats the latency as an output, searching the budget for the
// deepest stopband it can reach without giving up more linear-magnitude accuracy
// than the caller allows. [DesignPhaseInterpolation] and
// [DesignComplexLeastSquares] instead prescribe the entire complex response and
// then approximate it, so the caller states a phase curve — here parametrised
// by the same mix between minimum and linear phase — and the design only
// controls how the unavoidable approximation error is distributed.
// [DesignLowGroupDelay] gives up the phase entirely and constrains the
// magnitude instead, so the caller states how much magnitude accuracy the delay
// is worth.
//
// Among the two prescribing methods, phase interpolation is the unweighted
// least-squares solution: on a uniform DFT grid with uniform weights the
// normal equations reduce to the identity, so truncating the inverse transform
// already minimises the mean-square complex error. Both designs then agree
// exactly. A non-uniform weight is what makes the least-squares route
// worthwhile, and Lawson reweighting turns it into a peak-error design.
//
// Because the weighted objective says nothing about bins whose weight is zero,
// the response there is genuinely unconstrained and can diverge by orders of
// magnitude while the weighted band is matched closely. Weight bands down
// rather than out.
//
// Both objectives measure the absolute complex deviation, not a relative one.
// Where the prescribed response is small the absolute error is small too, so an
// unweighted design spends its budget on the passband and lets stopband
// accuracy slip. Supplying a weight that rises with the inverse target
// magnitude is the fix, and on the reference suite it is worth several times
// the dB error: on the crossover target a plain least-squares fit improves from
// 3.616 dB RMS to 0.568 dB, and on the deep notch from 3.083 dB to 1.048 dB
// (129 taps, a 1024-point grid, weight capped at 60 dB of range).
//
// That gain applies to the least-squares solution. Lawson reweighting is
// multiplicative and converges towards its own equilibrium, so after enough
// passes the supplied weight has almost no influence left: at sixteen passes
// the same two designs land within a thousandth of a dB of their unweighted
// counterparts. Weight the least-squares solution, or reweight towards the
// minimax one, but do not expect the weight to survive many passes of the
// latter.
//
// # Optimised delay: what the local search can and cannot do
//
// [DesignLowGroupDelay] is a local method on a non-convex problem, and it
// behaves like one. Three properties are worth knowing before relying on it.
//
// It does improve on a truncated minimum-phase design, which is the usual
// answer when low delay is wanted. On a 65-tap Hann-windowed low-pass at
// cutoff 0.08, designed on a 1024-point grid with 200 iterations per penalty
// stage, the mean passband group delay falls from 12.70 samples to 12.62 within
// a 1 dB tolerance and to 12.08 within 6 dB, and the peak passband delay falls
// from 23.58 to 22.72 and 21.18 respectively
// (TestLowGroupDelayQuotedImprovement). The
// gain is real but modest: minimum phase is already close to delay-optimal for
// a fixed magnitude, and the room the optimiser has is exactly the room the
// tolerance grants it.
//
// The starting point decides which optimum is reached. Started from the
// linear-phase prototype instead, the same 1 dB design ends at a mean delay of
// about 31.5 samples rather than 12.6, and it is a genuine local minimum: it
// improves on its own start and cannot be pushed further. Leaving that basin
// would mean moving a zero across the unit circle, and no descent direction
// does that. Start from a minimum-phase or short-delay design unless there is a
// reason not to.
//
// Convergence is slow rather than sharp. The objective keeps falling with the
// iteration budget instead of settling: at 6 dB tolerance the mean delay
// reaches 12.35 after 50 iterations per penalty stage, 12.08 after 200 and
// 10.79 after 800, while the relative magnitude error climbs from 1.2e-2 to
// 2.5e-2 to 1.4e-1. The budget is therefore a second delay-versus-accuracy
// dial, not a convergence threshold, and results should always be quoted with
// the budget that produced them.
//
// The design also costs far more than the other three. Every objective
// evaluation touches every grid bin and every tap, and there are hundreds of
// evaluations per penalty stage, so a large FFT size is expensive in a way it
// is not for the transform-based methods.
//
// # Group-delay measurement limits
//
// Group delay is not a meaningful whole-band metric when the response contains
// deep stopbands or spectral nulls. Phase is numerically fragile as magnitude
// approaches zero, so differentiating it there can produce arbitrarily large
// values that describe neither audible energy nor useful latency.
//
// [DesignLowGroupDelay] therefore uses squared target magnitude as its default
// delay weight, which suppresses stopbands and masks exact target nulls.
// [GroupDelayMetrics.Peak] likewise ignores bins below the documented weight
// threshold. Comparisons should state the evaluated frequency band and weight;
// an unmasked stopband group-delay curve is not evidence for or against a
// design.
//
// # The group-delay floor, and what a budget can be spent on
//
// A magnitude request carries a group delay whether or not the caller wanted to
// spend one. Every causal realisation of a given magnitude differs from the
// minimum-phase one by an all-pass factor, and an all-pass response has
// non-negative group delay at every frequency, so
//
//	groupDelay(h)(w) >= groupDelay(minimum phase)(w)   for all w
//
// The minimum-phase group delay is therefore a floor set by the magnitude, not
// a design choice. Measured at 129 taps on a 1024-point grid the six reference
// targets sit at 0.50, 5.86, 6.21, 8.63, 10.41 and 49.37 samples for room
// correction, the low-pass, the parametric EQ, the deep notch, the LR4 crossover
// and the LR8 crossover; TestZeroDelayDesignSitsOnTheMinimumPhaseFloor in
// internal/reference establishes that these belong to the targets rather than to
// the split. A 16-sample latency allowance is generous for the first five and
// meaningless for the last, which is why a fixed budget behaves so differently
// across them.
//
// Latency above the floor is exactly the all-pass factor, and choosing it is the
// whole of the remaining phase freedom. This construction makes that choice
// implicitly and takes the trivial one. The linear-phase factor is symmetric, so
// its phase is exactly linear and its group delay exactly constant; a cascade's
// group delay is the sum of its factors', so the cascade's deviation from
// constant group delay is the minimum-phase factor's alone:
//
//	ripple(h) = ripple(minimum-phase factor)
//
// The all-pass the budget inserts is z^-Delay. It translates the whole
// group-delay curve and does not flatten it. Measured across the six reference
// targets the weighted ripple is identical to nine decimal places at every
// budget the split admits, whether or not the linear factor carries real energy;
// see TestAdaptiveDelayBudgetCannotFlattenGroupDelay in internal/reference.
//
// This is a property of the construction, not of the latency.
// [DesignPhaseInterpolation] spends the same surplus on flatness instead, and at
// equal latency the difference is large: on the low-pass at 44.9 samples the
// factorisation still carries its full 1.117 samples of ripple where the
// prescribed continuum is down to 0.419, and on the LR4 crossover 0.766 against
// 0.287. TestFactorisationHoldsItsRippleWhileTheContinuumDescends pins both.
//
// The budget changes ripple only indirectly, by shortening the minimum-phase
// factor to Length-2*Delay taps until it can no longer realise the target's
// phase at all. That is a magnitude sacrifice, not a phase gain, and at the
// extreme it simply produces a linear-phase filter at the full (Length-1)/2 of
// latency — which [DesignPhaseInterpolation] reaches directly.
//
// So the delay budget here is a magnitude-recovery control. Its use is to
// restore accuracy when the output support is too short to host the target's
// minimum-phase response, and [DesignIterativeAuto] selects it on exactly that
// basis. A caller who wants flat group delay should prescribe the phase instead;
// what this construction offers such a caller is the opposite trade, described
// next.
//
// Below the floor no phase choice helps at all: the request is infeasible at
// that magnitude and only the magnitude can give way. [DesignLowGroupDelay] is
// the entry point for that regime, and widening its ToleranceDB is the dial. At
// 129 taps five of the six targets buy delay under their floor: room correction
// concedes the largest share of it at 80%, and the parametric EQ the most
// samples at 4.28, for at most 1.90 dB of RMS magnitude error. The LR8
// crossover buys nothing, because at that support its magnitude is already
// unrealisable and there is no accuracy left to concede.
//
// # What the construction is actually good for
//
// The comparison that matters for a latency-bounded application is not against
// a hand-picked budget but against a linear-phase FIR of the same latency. A
// linear-phase filter of latency L has only 2L+1 taps to spend on the magnitude,
// while a minimum-phase-led design of Length taps spends all of them and carries
// only the latency its own phase implies. So the question is how much latency
// linear phase needs to match it.
//
// Measured from 2049-tap fixtures at 1025 output taps with a zero budget, and
// read against the linear-phase family of docs/reference-delay-sweep.csv at the
// smallest sampled latency matching the same RMS dB error to within a thousandth
// of a decibel — latency being weighted mean group delay in the target's own
// band:
//
//   - room-correction: 0.046 dB at 0.6 samples. Linear phase never matches it,
//     reaching only 0.250 dB at 512 samples of latency.
//   - deep-notch: 0.000 dB at 9.9 samples against 224, a factor of 22.
//   - parametric-eq: 0.000 dB at 6.7 samples against 128, a factor of 19.
//   - crossover: 0.000 dB at 10.5 samples against 192, a factor of 18.
//   - low-pass: 0.000 dB at 5.9 samples against 96, a factor of 16.
//   - steep-crossover: 0.000 dB at 52.4 samples against 416, a factor of 8.
//
// The linear-phase family is sampled every 32 samples of latency, so each factor
// is accurate to within one stride. The cost is 0.8 to 8.0 samples of weighted
// group-delay ripple — the minimum-phase factor's, and not reducible by the
// budget. That is the trade to evaluate: a bounded group-delay deviation in
// exchange for the magnitude accuracy of a linear-phase filter an order of
// magnitude longer in latency.
//
// See TestSweepLinearPhaseNeedsFarMoreLatency in internal/reference, and note
// that this advantage belongs to the minimum-phase factorisation itself. The
// delay budget contributes nothing to it and is selected at zero on all six
// curves at this length.
//
// # When the alternating factorisation has nothing to do
//
// This is the first thing to check before reading anything into a
// [DesignIterative] result, and it is easy to miss because it fails silently
// and flatteringly.
//
// The split gives the minimum-phase factor Length-2*Delay taps. If the target's
// minimum-phase impulse response already fits in those taps, the factor alone
// reproduces the target, the residual quotient is unit-magnitude, and the
// zero-phase inverse transform of a flat magnitude is a unit impulse. The
// linear-phase factor converges to that impulse, the correction loop becomes a
// fixed point at the identity, and the design reduces to
//
//	h[n] = z^-Delay * minimum-phase(target)
//
// which is a delayed minimum-phase filter — a filter the method did not shape,
// carrying the delay without buying anything for it. The reported magnitude
// error is excellent, because minimum phase reproduces the magnitude; nothing
// in the metrics reveals that the factorisation was inert.
//
// Five of the six reference targets in internal/reference are in exactly this
// regime: their linear factor carries 0.000000 of its energy away from the
// centre tap. The steep-crossover target — an eighth-order crossover at 800 Hz,
// whose minimum-phase response does not fit the budget — carries 0.923977, and
// is the only one that exercises the method.
//
// The consequence for a caller is a rule of thumb: the alternating
// factorisation is worth reaching for when the target is too long or too steep
// for the minimum-phase factor's share of the taps. When it is not, the honest
// comparison is against Delay zero, which is the same design with the budget
// removed and is usually better on every axis.
//
// # The delay budget is a function of the output length
//
// Whether a budget is worth anything depends on the output support, not on how
// much latency the application can afford. Once Length is comfortably larger than
// the target's minimum-phase support, the factor fits at any budget the split
// admits and there is nothing left to recover.
//
// docs/reference-delay-sweep.csv measures this directly. The best RMS dB error
// any non-zero budget saves against the zero-budget design, per target and output
// length, from 2049-tap fixtures:
//
//	target              129 taps   257 taps   513 taps   1025 taps
//	steep-crossover      57.19 dB   23.05 dB    0.00 dB     0.00 dB
//	room-correction       0.00 dB    0.04 dB    0.03 dB     0.00 dB
//	the other four        0.00 dB    0.00 dB    0.00 dB     0.00 dB
//
// So one of the six curves benefits materially, and only below about 513 taps.
// The minimum-phase supports that explain it — leading taps holding all but 1e-6
// of the factor's energy — are 52 for crossover, 53 for low-pass, 116 for
// parametric-eq, 129 for deep-notch, 238 for steep-crossover and 995 for
// room-correction. room-correction is the instructive exception: its support
// exceeds three of the four lengths swept, yet a budget buys it only hundredths
// of a decibel, because its response is broadband rather than steep and a
// symmetric factor cannot supply the missing tail.
//
// The practical reading for a long filter with latency to spare is therefore the
// opposite of the intuitive one. Do not raise the budget because the delay line
// is there; a longer filter needs less budget, not more, and
// [DesignIterativeAuto] selects zero on all six curves at 1025 taps. See
// TestSweepDelayBudgetStopsPayingAsLengthGrows in internal/reference.
//
// [DesignIterativeAuto] applies that rule for the caller. Given the same six
// fixtures it selects Delay zero on all five that fit — returning bit-for-bit
// the minimum-phase design, so the degeneracy costs nothing instead of sixteen
// samples — and Delay 22 on steep-crossover, where it reaches 3.310 dB RMS
// magnitude error against the 6.901 dB the hand-picked budget of 16 achieves.
// It is also what protects a caller from the opposite mistake: on that same
// target a budget of 1 raises the relative magnitude error from 1.227% to 77.5%,
// far worse than either extreme, so a small delay chosen to be economical is the
// worst available choice. See TestAdaptiveDelaySelectionBeatsTheFixedBudget in
// internal/reference.
//
// Where the factor is genuinely starved, the method earns its delay. On
// steep-crossover at 129 taps and delay 16 it reaches 6.901 dB RMS magnitude
// error, against 54.483 dB for phase interpolation, 54.934 dB for
// minimum-phase truncation and 42.838 dB for the low-delay optimiser, at a
// comparable mean group delay. It gives up linear-magnitude accuracy to get
// there (2.509% against 1.227% for minimum-phase truncation) and it has the
// worst group-delay ripple of the fixed-delay methods. Those trade-offs are
// visible in every row of docs/reference-results.csv.
//
// # Conditioning of the alternating factorisation
//
// The [DesignIterative] updates are not a contraction. On the 129-tap,
// 0.08-cutoff low-pass at delay 8, the default grid and magnitude floor produce
// RMS dB errors of 5.366154, 4.609232 and 4.917232 over the first three
// correction passes. Continuing to pass twelve amplifies native and
// JavaScript/WASM rounding into 10.739658 and 15.829906 dB respectively.
//
// A larger Epsilon bounds that late growth but does not make the alternating
// truncated projections monotone. By default the design therefore stops
// before the first rising pass and returns the previous factors. This case
// accepts two passes and both builds agree on the checked coefficients within
// 1e-10. A negative [IterativeConfig.ToleranceDB] disables both the settling
// and rising-error stops, and should be used only when investigating a fixed
// iteration budget.
//
// # Minimum-phase reconstruction
//
// Both designs rest on a spectral factorisation that turns a sampled magnitude
// response into a causal minimum-phase spectrum. Two implementations are
// available through [MinimumPhaseMethod]:
//
//   - [MethodCepstrum] folds the real cepstrum onto its causal half and
//     exponentiates the complex log spectrum.
//   - [MethodHilbert] evaluates the discrete Hilbert transform of the log
//     magnitude to obtain the phase and pairs it with the target magnitude.
//
// The two are mathematically equivalent. They differ numerically because the
// cepstral route reconstructs the magnitude through the exponential, while the
// Hilbert route carries it through untouched. On a dense grid the Hilbert
// reconstruction therefore reproduces the target magnitude to machine precision
// regardless of the magnitude floor, whereas the cepstral deviation grows with
// the log-domain dynamic range (roughly 1e-10 relative at a floor of 1e-6 and
// 1e-6 at a floor of 1e-12 for a narrow low-pass).
//
// For a single reconstruction that difference is irrelevant in practice:
// truncating the dense response to the tap budget dominates, and both methods
// produce the same FIR to within 1e-10 of its peak. Inside [DesignIterative]
// the difference does matter, because every pass divides by a truncated factor
// and the ill-conditioned nulls of that division amplify it; the two methods
// then converge to designs whose errors differ by a few dB in either direction,
// with neither systematically ahead. Runtime and allocations are equivalent —
// both perform one inverse and one forward transform per reconstruction.
//
// # What the reconstruction does not guarantee
//
// Two limitations are worth stating plainly, because neither is visible in the
// returned error and both are easy to assume away.
//
// The grid is oversampled but not converged. A zero FFTSize selects a power of
// two at least eight times the prototype length, which bounds time-domain
// aliasing without eliminating it, and the residual does not shrink as
// prototypes grow — eight times a longer prototype is still eight times. The
// budget a caller accepts by leaving FFTSize at zero is measured by
// TestMinimumPhaseAliasingBudget: about 0.4% of peak for a 33-tap low-pass,
// about 0.9% for a 129-tap one, and 41% for a two-tap prototype, where the
// 16-point floor applies. Raise FFTSize when that matters;
// TestMinimumPhaseAliasingShrinksWithTheGrid pins that it helps monotonically.
//
// The result is minimum phase only up to truncation. The dense reconstruction
// is minimum phase, but it is then cut to the prototype length, and truncation
// can push zeros just outside the unit circle. The practical consequence is
// that the result is not safe to invert: for a 63-tap low-pass on a 512-point
// grid the inverse recursion diverges outright, and reaches 4.3e5 at 8192 and
// 1.2e5 at 131072 (TestMinimumPhaseInverseStability). Use it as a factor, not
// as something to divide by.
//
// # Weighted least squares: the ridge is silent
//
// [DesignComplexLeastSquares] solves weighted normal equations by Cholesky
// factorisation, and retries with diagonal loadings of 0, 1e-12, 1e-9, 1e-6 and
// 1e-3 relative until one succeeds. The ladder is what keeps wide zero-weight
// bands from returning [ErrSingularSystem], but a loading at the top of it
// changes the returned filter materially, and [Result] does not report which
// rung was used. A design that looks unexpectedly smooth in an unweighted band
// is the symptom. Weighting bands down rather than out avoids the situation
// entirely.
package mixedphase
