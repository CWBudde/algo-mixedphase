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
// steer it: on every reference target the search selects the same delay with the
// guard removed entirely, because the small delays that wreck the passband — a
// one-sample budget on the eighth-order crossover raises the relative error from
// 0.0123 to 0.775, see TestSmallDelayBudgetsAreTheWorstChoice in
// internal/reference — have a worse dB error too and lose on the objective.
//
// The default is therefore deliberately generous. A tight guard does active
// harm: the deep-stopband solutions need about 2.2 times the minimum-phase
// relative error, so a ceiling below that excludes the whole basin and selects a
// design an order of magnitude worse. Three times leaves the basin comfortably
// inside the admissible set.
const defaultRelativeErrorSlack = 2.0

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
// factorisation actually buys — subject to the linear-magnitude error staying
// within RelativeErrorSlack of the zero-delay design. A zero budget is always
// evaluated and is always admissible, so the result can never be worse than
// minimum-phase truncation *by that objective*, and on a target where the delay
// would be wasted it selects zero and returns exactly that design.
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
// than one — about 25 for a 129-tap filter at the defaults — and it is only
// exact when CoarseStep is one. Result.Delay reports the budget it selected, and
// Result.LinearPhasePart is a unit impulse exactly when it selected zero.
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
	); err != nil {
		return Result{}, err
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

	search := delaySearch{prototype: prototype, cfg: cfg, length: length}

	// The zero-delay design is both the floor the search may not fall below and
	// the reference the passband guard is measured against, so it is evaluated
	// first and unconditionally.
	base, err := search.designAt(0)
	if err != nil {
		return Result{}, err
	}

	search.best = base
	search.ceiling = math.Inf(1)

	if slack >= 0 {
		search.ceiling = (1 + slack) * base.Metrics.RelativeMagnitudeError
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

// consider designs at one delay and keeps it when it improves the objective
// without breaching the passband guard.
func (s *delaySearch) consider(delay int) error {
	candidate, err := s.designAt(delay)
	if err != nil {
		return err
	}

	if candidate.Metrics.RelativeMagnitudeError > s.ceiling {
		return nil
	}

	if candidate.Metrics.RMSMagnitudeErrorDB <
		s.best.Metrics.RMSMagnitudeErrorDB {
		s.best = candidate
	}

	return nil
}
