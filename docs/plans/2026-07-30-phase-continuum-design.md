# One knob across the phase continuum

Design note, 2026-07-30. Companion to the rewrite of `docs/paper/paper.typ`.

## Why

The package already spans the whole phase continuum, but on two unrelated knobs, so no
document can present it as one algorithm:

| regime                           | today's knob                    | entry point                |
| -------------------------------- | ------------------------------- | -------------------------- |
| below the minimum group delay    | `ToleranceDB` (dB of magnitude) | `DesignLowGroupDelay`      |
| minimum → linear → maximum phase | `Mix` (dimensionless, [0, 2])   | `DesignPhaseInterpolation` |

Worse, a third knob looks like it belongs on the axis and does not: `IterativeConfig.Delay`
inserts a pure `z^-d`, so it translates the group-delay curve rather than shaping it. It is a
magnitude-recovery control. Anyone reading the API can be forgiven for assuming otherwise,
and the current paper spends most of its length establishing that it is not a phase knob.

This note defines one knob, in samples of group delay, that covers all four regimes, and
records the analytical result that makes it exact rather than a search.

## The affine delay law

`prescribedResponse` (`mixedphase/target.go`) builds

    phi(mix, omega) = (1 - mix) * phi_min(omega) + mix * (-omega * D),  D = (N-1)/2

Group delay is `-dphi/domega`, and the operator is linear in `phi`, so the group delay of the
prescription is the same blend of the two endpoint delays:

    tau(mix, omega) = (1 - mix) * tau_min(omega) + mix * D

Averaging over any fixed weight leaves the blend intact, so the weighted mean obeys

    tau(mix) = (1 - mix) * tau_min + mix * (N-1)/2                                       (1)

The realised filter is the least-squares projection of that prescription onto `N` causal
taps, so (1) is exact for the prescription and approximate for the filter. Measured residual
across the six reference targets at 129 taps, from `docs/reference-phase-regimes.csv`:

| target          | mix   | predicted | measured |
| --------------- | ----- | --------- | -------- |
| low-pass        | 0.125 | 13.13     | 13.17    |
| crossover       | 0.125 | 17.11     | 17.117   |
| steep-crossover | 0.125 | 51.20     | 51.204   |
| low-pass        | 2     | 122.14    | 122.14   |

Two consequences the design rests on.

**The knob inverts in closed form.** Solving (1) for the mix that realises a requested delay:

    mix(tau) = (tau - tau_min) / ((N-1)/2 - tau_min)                                     (2)

No search, no iteration, and the cost of the whole in-window regime is one
`DesignPhaseInterpolation` call.

**The reachable window is set by the target, not by the length.** `mix` is confined to
`[0, 2]`, so (1) bounds the achievable delay to

    tau in [tau_min, N-1-tau_min]                                                        (3)

which is centred on `(N-1)/2` and shrinks as the target's own minimum-phase delay grows. At
129 taps the six reference targets give windows of width `N-1-2*tau_min`:

| target          | tau_min | window       |
| --------------- | ------- | ------------ |
| room correction | 0.50    | 0.5 … 127.5  |
| low-pass        | 5.86    | 5.9 … 122.1  |
| parametric EQ   | 6.21    | 6.2 … 121.8  |
| deep notch      | 8.63    | 8.6 … 119.4  |
| LR4 crossover   | 10.41   | 10.4 … 117.6 |
| LR8 crossover   | 49.37   | 49.4 … 78.6  |

The steep crossover has almost no continuum left: its magnitude is barely realisable in 129
taps, and a magnitude that hard to realise dictates its own latency.

## The four regimes

Outside the window, (3) says phase choice is exhausted and only the magnitude can move. That
is the whole content of regime (a), and its mirror above the window.

| requested tau            | mechanism                                               | what is spent            |
| ------------------------ | ------------------------------------------------------- | ------------------------ |
| `< tau_min`              | constrained optimisation, §"Below the floor"            | magnitude accuracy       |
| `tau_min`                | `mix = 0`, minimum phase                                | nothing                  |
| `(tau_min, (N-1)/2)`     | `mix` from (2)                                          | latency                  |
| `(N-1)/2`                | `mix = 1`, linear phase, zero ripple                    | full half-length latency |
| `((N-1)/2, N-1-tau_min)` | `mix` from (2), maximum-phase side                      | latency and pre-ringing  |
| `N-1-tau_min`            | `mix = 2`, maximum phase, the time reverse of `mix = 0` | worst of both            |
| `> N-1-tau_min`          | mirror of the sub-floor solve on the reversed filter    | magnitude accuracy       |

Note what the knob does **not** buy monotonically. Group-delay ripple falls monotonically to
exactly zero at `mix = 1` and rises again by reflection. Magnitude error does not follow it.
Measured relative magnitude error, 129 taps:

| target                  | mix 0   | interior peak  | mix 1   | mix 2   |
| ----------------------- | ------- | -------------- | ------- | ------- |
| reference low-pass      | 4.6e-9  | 4.6e-3 @ 0.125 | 2.1e-5  | 4.6e-9  |
| parametric EQ           | 4.3e-5  | 1.4e-2 @ 0.25  | 2.9e-3  | 4.3e-5  |
| 65-tap low-pass → 129   | 1.5e-5  | 7.6e-4 @ 0.09  | 4.6e-4  | 1.5e-5  |
| LR8 crossover (starved) | 1.23e-2 | 1.54e-2 @ 1    | 1.54e-2 | 1.23e-2 |

Three claims survive on every target, and are the ones the paper makes:

1. **The endpoints are the most accurate points of the continuum**, not just the fastest and
   the slowest. A spectral factor of the target needs no compromise; every intermediate phase
   has to be approximated on the same support. The error rises steeply the moment the knob
   leaves either end — by three orders of magnitude on the reference low-pass.
2. **The error is exactly symmetric about `mix = 1`**, since reversal cannot change a
   magnitude. Linear phase is therefore always a stationary point of the accuracy curve — but
   it is a local _minimum_ for some targets (reference low-pass, LR4) and a local _maximum_
   for others (the 65-tap low-pass above, deep notch). It cannot be recommended or dismissed
   on accuracy grounds in general.
3. **A support-starved target flattens the whole structure.** When the magnitude is not
   realisable in the available taps, truncation dominates every phase choice and the error
   becomes nearly independent of the knob (LR8: 1.23e-2 to 1.54e-2 across the entire
   continuum). This is the same condition under which the window of (3) collapses, so a narrow
   window is a warning that the accuracy structure is absent too.

An earlier draft of this note claimed three local minima and an expensive middle. Claim 2
above is what the data actually supports; the middle is expensive relative to the ends, but
linear phase is not reliably a sweet spot.

## Below the floor: swap the objective and the constraint

Wu–Gao–Teo, as implemented in `DesignLowGroupDelay`, minimises weighted group delay subject
to a magnitude band:

    minimise   sum_k w_k * tau_k        subject to   | |H_k| - T_k | <= tol_k

Its knob is therefore `tol`, and the delay is an outcome. For a delay-parameterised design we
need the opposite: the delay is the request and the magnitude is what gives way.

    minimise   sum_k m_k * (|H_k| - T_k)^2      subject to   sum_k w_k * tau_k = tau_req

Both forms are smooth, non-convex, and solved by the same machinery already in the package —
`minimizeLBFGS` with Armijo backtracking (`lbfgs.go`) inside a penalty ladder that multiplies
the penalty by ten per stage. Only the roles swap, so no new optimiser is needed.

The gradient pieces are also already there. Per bin, with `H = A + jB` and
`dH/domega = -j(C + jD)`, the existing `evaluate` accumulates `A, B, C, D`, the delay
`tau = (AC + BD)/|H|^2`, and the derivative of `|H|^2/2` with respect to each tap. The swapped
form needs exactly those quantities.

One structural difference: the delay constraint is a _scalar_ over the whole grid, so its
residual `c = sum_k w_k tau_k - tau_req` is not known until every bin has been visited. The
gradient therefore needs two passes over the grid — one to accumulate `c` and the magnitude
objective with its gradient, a second to add `2 * penalty * c * d(sum w tau)/dh`. This is
implemented as a separate `evaluateMatchDelay` method on the existing `lowDelayProblem`, so
`evaluate` and hence `DesignLowGroupDelay` are untouched and provably unchanged
(`TestDesignLowGroupDelayIsUnchangedByTheNewProblemForm`).

Two hazards carry over unchanged from the existing low-delay path and must be documented on
the new one:

- **The start point picks the basin.** Started from the linear-phase prototype instead of the
  minimum-phase one, the same 1 dB design lands ~31.5 samples away and is a genuine local
  minimum: leaving it would mean moving a zero across the unit circle, which no descent
  direction does.
- **The iteration budget is a second delay-vs-accuracy dial**, not a convergence threshold.
  Results are quoted with the budget that produced them.

Expect at least one target to have no usable sub-floor regime at all. The LR8 crossover buys
nothing below its floor even at 2 dB of tolerance, because its magnitude is already
unrealisable in 129 taps. That is a result to report, not a failure to paper over.

## API

Additive. Every existing entry point keeps its behaviour and every committed artifact keeps
its bytes (AGENTS.md rule 4).

```go
type ContinuumConfig struct {
    Length           int
    TargetGroupDelay float64   // the knob, in samples
    FFTSize          int
    Epsilon          float64
    Method           MinimumPhaseMethod
    Iterations       int       // sub-floor optimiser budget
    PenaltyStages    int
    DelayWeight      []float64 // band whose delay the knob refers to
}

func DesignContinuum(prototype []float64, cfg ContinuumConfig) (Result, error)
```

`Result` gains two fields, `Regime` and `AchievedGroupDelay`, so a caller can tell which of
the four paths ran and how closely the request was met. A request outside `[0, N-1]`, or one
the sub-floor solve cannot reach, returns the new `ErrDelayOutOfReach` sentinel wrapped with
context per AGENTS.md rule 6.

Reused rather than reimplemented: `minimumPhaseSpectrum`, `nextDesignFFTSize` (`fft.go`),
`prescribedResponse` (`target.go`), `DesignPhaseInterpolation` (`interpolate.go`),
`newLowDelayProblem`, `runPenaltyLadder` (`lowdelay.go`), `Analyze` (`analyze.go`),
`validatePrototype`, `validateFiniteFields` (`validate.go`).

## Artifacts and figures

New generators in `internal/reference/continuum.go`, following `sweep.go` and `regimes.go`:

- `docs/reference-continuum.csv` — fine tau grid, both branches, six targets, carrying
  `predicted_delay` alongside `achieved_delay` so the residual of (1) is itself committed.
- `docs/reference-continuum-impulse.csv` — peak-aligned impulse snapshots at six tau, so the
  reversal into maximum phase is visible rather than asserted.
- `regime=unified` rows appended to `docs/reference-phase-regimes.csv`.

Seven figures, all native SVG in `docs/paper/charts.typ`: the three-panel continuum map; the
residual of (1); the comparison against minimum-phase truncation, linear phase, prescribed
phase, Lawson minimax and Wu–Gao–Teo; ripple at matched latency against the 2012
factorisation; the impulse gallery; the reachable window and its collapse with length; and the
sub-floor trade. The last two replace the only two hand-typed tables left in the paper.
