package mixedphase

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/window"
)

// defaultDelaySearchStep is the stride of the coarse delay scan.
//
// The error is not unimodal in the delay budget: it falls smoothly towards a
// basin and then jitters between neighbouring delays inside it, because the
// parity of the linear factor's length changes which zeros it can place. A
// stride of four reaches the basin in a sixth of the designs an exhaustive scan
// would need, and the refinement pass below recovers the jitter.
const defaultDelaySearchStep = 4

// defaultRelativeErrorSlack is how much linear-magnitude accuracy a candidate
// may give up relative to the minimum-phase design, as a multiplier.
//
// The guard exists to bound what the objective is allowed to trade away, not to
// steer it. At the repository's published operating point — 129 output taps from
// a 257-tap prototype, see internal/reference — it never binds: the search
// selects the same delay on all six targets with the guard removed entirely,
// because the small delays that wreck the passband (a one-sample budget on the
// eighth-order crossover raises the relative error from 0.0123 to 0.775, see
// TestSmallDelayBudgetsAreTheWorstChoice) have a worse dB error too and lose on
// the objective anyway.
//
// The default is therefore deliberately generous. A tight guard does active
// harm: the deep-stopband solutions need about 2.2 times the minimum-phase
// relative error, so a ceiling below that excludes the whole basin and selects a
// design an order of magnitude worse. Three times leaves the basin comfortably
// inside the admissible set.
const defaultRelativeErrorSlack = 2.0

// defaultRelativeErrorFloor is the smallest relative magnitude error the guard
// will ever treat as a breach.
//
// A purely multiplicative ceiling is meaningless once the zero-delay design is
// already excellent, and actively harmful. Designing the same eighth-order
// crossover into 257 taps puts the zero-delay relative error at 4.7e-4, so a
// three-times ceiling is 1.4e-3 — and that rejects the 8-sample budget, whose
// relative error is 9.0e-3 but whose RMS dB error is 1.78 against the zero-delay
// design's 24.83. The guard would trade 23 dB of stopband for a passband
// regression of under a hundredth of a decibel, which is not the trade it exists
// to prevent.
//
// The floor makes the guard absolute where the relative form stops carrying
// meaning. A relative magnitude error of 1e-2 is roughly 0.09 dB, so anything
// under it is inaudible and not worth protecting; the guard resumes its
// multiplicative behaviour as soon as the zero-delay design is worse than that.
// See TestDesignIterativeAutoGuardDoesNotRejectAMuchBetterDesign.
const defaultRelativeErrorFloor = 1e-2

// defaultMinimumImprovementDB is how much RMS dB magnitude error a non-zero
// delay budget has to save before it is worth its latency.
//
// Without it the search buys latency for arbitrarily small gains. A 129-tap
// low-pass improves from 0.800 dB to 0.498 dB at a one-sample budget, so a bare
// "lower is better" rule spends a sample of delay, abandons the exact
// minimum-phase design, and reports a non-degenerate factorisation for three
// tenths of a decibel. Latency is a cost the objective does not otherwise price,
// so near-ties must go to the shorter delay.
//
// One decibel separates the two regimes cleanly. The budgets that genuinely pay
// save far more — 23 dB at 257 taps on the eighth-order crossover, 51 dB at 129 —
// while every gain this rule discards is a fraction of a decibel.
const defaultMinimumImprovementDB = 1.0

// AutoIterativeConfig configures [DesignIterativeAuto]. Every field it shares
// with [IterativeConfig] has the same meaning and is forwarded unchanged.
type AutoIterativeConfig struct {
	// Length is the number of taps in the resulting FIR. Zero uses the
	// prototype length.
	Length int

	// Iterations is the maximum number of alternating correction passes spent
	// on each candidate delay. Zero uses the [DesignIterative] default.
	Iterations int

	// FFTSize controls the dense frequency grid. Zero selects a power of two
	// at least eight times the filter length.
	FFTSize int

	// Epsilon is the magnitude floor used by logarithms and regularised
	// spectral division. Zero selects a scale-relative default. Negative
	// values are rejected.
	Epsilon float64

	// Window selects the truncation window for both factors.
	Window window.Type

	// WindowAlpha supplies the alpha or beta parameter for parametric windows.
	WindowAlpha float64

	// Method selects the minimum-phase reconstruction.
	Method MinimumPhaseMethod

	// ToleranceDB stops each candidate's correction loop. It is forwarded to
	// [IterativeConfig.ToleranceDB] and does not affect the delay search.
	ToleranceDB float64

	// MaxDelay bounds the search. Zero uses (Length-1)/2, the largest budget
	// the split admits. Negative values are rejected.
	MaxDelay int

	// CoarseStep is the stride of the first scan. Zero uses four. One makes
	// the search exhaustive and therefore exact, at the cost of one design per
	// admissible delay.
	CoarseStep int

	// RelativeErrorSlack bounds how much linear-magnitude accuracy a candidate
	// may give up against the zero-delay design, as a multiple of that design's
	// relative error. Zero uses 2, so a candidate may be three times worse. A
	// negative value removes the guard.
	//
	// Raising it is close to free and lowering it is not: a ceiling that
	// excludes the deep-stopband delays selects a much worse design rather than
	// a more conservative one.
	RelativeErrorSlack float64

	// RelativeErrorFloor is the smallest relative magnitude error the guard
	// treats as a breach, so that a candidate is rejected only when it exceeds
	// both this floor and the RelativeErrorSlack multiple of the zero-delay
	// design. Zero uses 1e-2, about 0.09 dB. Negative values are rejected.
	//
	// It exists because the multiplicative form loses its meaning once the
	// zero-delay design is already accurate: three times an error of 4.7e-4 is
	// still inaudible, yet enforcing it can cost 23 dB of stopband depth. Set it
	// to a very small positive value to recover the purely multiplicative
	// behaviour.
	RelativeErrorFloor float64

	// MinimumImprovementDB is how much RMS dB magnitude error a non-zero delay
	// budget must save against the zero-delay design before the search will
	// spend the latency. Zero uses 1 dB. Negative values are rejected.
	//
	// It prices the latency the objective would otherwise ignore, so that a
	// budget is taken only when it buys a real accuracy gain rather than a
	// fraction of a decibel. Setting it to a very small positive value recovers
	// a pure "lowest error wins" search.
	MinimumImprovementDB float64
}

// DesignIterativeAuto chooses the delay budget for the alternating
// factorisation instead of taking it as an input.
//
// [DesignIterative] spends a delay budget whether or not that budget buys
// anything. When the target's minimum-phase impulse response already fits the
// Length-2*Delay taps the split leaves it, the residual quotient is
// unit-magnitude, the linear factor collapses to a unit impulse, and the design
// is a delayed minimum-phase filter: the latency is paid and nothing is
// received. Across the repository's reference targets that is the common case,
// and there plain minimum-phase truncation beats the factorisation on accuracy
// and latency simultaneously.
//
// This entry point removes that failure mode. It minimises the RMS dB magnitude
// error — the measure sensitive to stopband depth, which is what the
// factorisation actually buys — subject to two constraints: the linear-magnitude
// error must stay within RelativeErrorSlack of the zero-delay design or under
// RelativeErrorFloor, whichever is the looser bound, and a non-zero budget must
// beat the zero-delay design's dB error by at least MinimumImprovementDB, which
// prices the latency the objective would otherwise ignore. Ties go to the shorter
// delay.
//
// A zero budget is always evaluated and is always admissible, so the result can
// never be worse than minimum-phase truncation *by that objective*, and on a
// target where the delay would be wasted it selects zero and returns exactly that
// design.
//
// What the delay budget buys, and what it cannot. The budget recovers magnitude
// accuracy when the output support is too short to host the target's
// minimum-phase response, and nothing else. It cannot buy phase linearity: the
// linear factor is symmetric, so it contributes exactly linear phase, and the
// cascade's group-delay deviation from linear equals its minimum-phase factor's
// regardless of Delay. Raising the budget to flatten group delay does not work
// and is not what this search is for — see the group-delay notes in the package
// documentation.
//
// The useful range of budgets is therefore narrow and depends on the output
// length rather than on the available latency. Designing an eighth-order 800 Hz
// crossover from a 2049-tap prototype, the selected budget is 28 samples at 129
// output taps, 8 at 257, and zero from 513 upwards, because by then the
// minimum-phase factor fits and the correction has nothing left to do.
//
// What it does not promise: the selected design may be worse in relative
// magnitude error, by up to the slack, and may carry more group delay. Both are
// the trade being made rather than a defect. On the repository's reference
// targets the cost is 2.6 samples of mean group delay and a doubled relative
// error on the one target where a non-zero budget wins, in exchange for 71 dB of
// stopband rejection; on the other five it selects zero and costs nothing.
//
// The search is a coarse scan followed by a local refinement, so it is a
// heuristic: it costs roughly MaxDelay/CoarseStep + 2*CoarseStep designs rather
// than one, and it is only exact when CoarseStep is one. Because MaxDelay
// defaults to (Length-1)/2, that count grows with the output length — about 24
// designs at 129 taps, 40 at 257, 72 at 513 and 136 at 1025 — while each design
// also costs more, so the search is roughly quadratic in Length. Measured on an
// 8192-point grid it runs in 0.29 s at 129 taps and 1.9 s at 1025. Where the
// application knows the delay it can actually spend, setting MaxDelay bounds the
// count directly and is the cheapest knob available.
//
// Result.Delay reports the budget it selected, and Result.LinearPhasePart is a
// unit impulse exactly when it selected zero.
func DesignIterativeAuto(
	prototype []float64,
	cfg AutoIterativeConfig,
) (Result, error) {
	if err := validatePrototype(prototype); err != nil {
		return Result{}, err
	}

	length := cfg.Length
	if length == 0 {
		length = len(prototype)
	}

	if length <= 0 {
		return Result{}, ErrInvalidLength
	}

	if err := validateFiniteFields(
		field{"relative error slack", cfg.RelativeErrorSlack},
		field{"relative error floor", cfg.RelativeErrorFloor},
		field{"minimum improvement", cfg.MinimumImprovementDB},
	); err != nil {
		return Result{}, err
	}

	if cfg.RelativeErrorFloor < 0 {
		return Result{}, fmt.Errorf(
			"%w: relative error floor %g is negative",
			ErrInvalidTolerance,
			cfg.RelativeErrorFloor,
		)
	}

	if cfg.MinimumImprovementDB < 0 {
		return Result{}, fmt.Errorf(
			"%w: minimum improvement %g dB is negative",
			ErrInvalidTolerance,
			cfg.MinimumImprovementDB,
		)
	}

	maxDelay := (length - 1) / 2

	if cfg.MaxDelay < 0 {
		return Result{}, fmt.Errorf(
			"%w: maximum delay %d is negative",
			ErrInvalidDelay,
			cfg.MaxDelay,
		)
	}

	if cfg.MaxDelay > 0 {
		maxDelay = min(cfg.MaxDelay, maxDelay)
	}

	step := cfg.CoarseStep
	if step == 0 {
		step = defaultDelaySearchStep
	}

	if step < 1 {
		return Result{}, fmt.Errorf(
			"%w: coarse step %d is not positive",
			ErrInvalidLength,
			step,
		)
	}

	slack := cfg.RelativeErrorSlack
	if slack == 0 {
		slack = defaultRelativeErrorSlack
	}

	floor := cfg.RelativeErrorFloor
	if floor == 0 {
		floor = defaultRelativeErrorFloor
	}

	improvement := cfg.MinimumImprovementDB
	if improvement == 0 {
		improvement = defaultMinimumImprovementDB
	}

	search := delaySearch{prototype: prototype, cfg: cfg, length: length}

	// The zero-delay design is the floor the search may not fall below, the
	// reference the passband guard is measured against, and the baseline a
	// non-zero budget has to beat by the improvement margin. It is therefore
	// evaluated first and unconditionally.
	base, err := search.designAt(0)
	if err != nil {
		return Result{}, err
	}

	search.best = base
	search.ceiling = math.Inf(1)
	search.admissible = base.Metrics.RMSMagnitudeErrorDB - improvement

	if slack >= 0 {
		search.ceiling = max(
			(1+slack)*base.Metrics.RelativeMagnitudeError,
			floor,
		)
	}

	for delay := step; delay <= maxDelay; delay += step {
		if err := search.consider(delay); err != nil {
			return Result{}, err
		}
	}

	// The coarse scan lands in the basin but not on its floor, so rescan the
	// stride either side of the winner. Delays already visited are cheap to
	// repeat relative to the bookkeeping needed to skip them.
	for delay := max(search.best.Delay-step+1, 1); delay <= min(search.best.Delay+step-1, maxDelay); delay++ {
		if err := search.consider(delay); err != nil {
			return Result{}, err
		}
	}

	return search.best, nil
}

// delaySearch carries the state of one delay search.
type delaySearch struct {
	prototype []float64
	cfg       AutoIterativeConfig
	length    int

	best    Result
	ceiling float64

	// admissible is the largest RMS dB error a non-zero budget may have and
	// still be worth its latency: the zero-delay design's error less
	// MinimumImprovementDB. It is derived from the zero-delay design alone
	// rather than from the running incumbent, so the outcome does not depend on
	// the order candidates are visited in.
	admissible float64
}

func (s *delaySearch) designAt(delay int) (Result, error) {
	result, err := DesignIterative(s.prototype, IterativeConfig{
		Length:      s.length,
		Delay:       delay,
		Iterations:  s.cfg.Iterations,
		FFTSize:     s.cfg.FFTSize,
		Epsilon:     s.cfg.Epsilon,
		Window:      s.cfg.Window,
		WindowAlpha: s.cfg.WindowAlpha,
		Method:      s.cfg.Method,
		ToleranceDB: s.cfg.ToleranceDB,
	})
	if err != nil {
		return Result{}, err
	}

	// DesignIterative reports the budget it was given, so the candidate already
	// carries the delay that produced it.
	return result, nil
}

// consider designs at one delay and keeps it when it earns its latency without
// breaching the passband guard.
func (s *delaySearch) consider(delay int) error {
	candidate, err := s.designAt(delay)
	if err != nil {
		return err
	}

	if candidate.Metrics.RelativeMagnitudeError > s.ceiling {
		return nil
	}

	// A non-zero budget has to clear the zero-delay design by the improvement
	// margin, not merely beat the running incumbent, or the search spends
	// latency on gains too small to matter.
	if candidate.Metrics.RMSMagnitudeErrorDB > s.admissible {
		return nil
	}

	// Ties go to the shorter delay, which is why this is a strict comparison
	// against an incumbent that is only ever replaced by a genuine improvement.
	if candidate.Metrics.RMSMagnitudeErrorDB <
		s.best.Metrics.RMSMagnitudeErrorDB {
		s.best = candidate
	}

	return nil
}
