package mixedphase

import (
	"errors"

	"github.com/cwbudde/algo-dsp/dsp/window"
)

var (
	// ErrEmptyPrototype is returned when a design receives no prototype taps.
	ErrEmptyPrototype = errors.New("mixedphase: empty prototype")
	// ErrNonFinitePrototype is returned when a prototype contains a NaN or an
	// infinity. Such a value would otherwise poison the magnitude floor and
	// propagate silently into every returned tap.
	ErrNonFinitePrototype = errors.New(
		"mixedphase: prototype contains a non-finite value",
	)
	// ErrNonFiniteConfig is returned when a configuration field that is only
	// range-checked receives a NaN, which compares false against every bound.
	ErrNonFiniteConfig = errors.New(
		"mixedphase: configuration value is not finite",
	)
	// ErrInvalidLength is returned when the requested FIR length is invalid.
	ErrInvalidLength = errors.New("mixedphase: invalid filter length")
	// ErrInvalidDelay is returned when the requested pre-delay does not fit.
	ErrInvalidDelay = errors.New("mixedphase: invalid delay")
	// ErrInvalidPhaseMix is returned when phase mix is outside [0, 2].
	ErrInvalidPhaseMix = errors.New("mixedphase: phase mix must be in [0, 2]")
	// ErrDelayOutOfReach is returned when a requested group delay lies outside
	// [0, Length-1], which no causal FIR of that support realises.
	ErrDelayOutOfReach = errors.New(
		"mixedphase: requested group delay is outside [0, Length-1]",
	)
	// ErrInvalidEpsilon is returned when a negative magnitude floor is given.
	ErrInvalidEpsilon = errors.New("mixedphase: epsilon must not be negative")
	// ErrInvalidWindowAlpha is returned when a negative window alpha is given.
	ErrInvalidWindowAlpha = errors.New(
		"mixedphase: window alpha must not be negative",
	)
	// ErrInvalidMethod is returned when an unknown reconstruction method is
	// requested.
	ErrInvalidMethod = errors.New(
		"mixedphase: unknown minimum-phase reconstruction method",
	)
	// ErrInvalidWeight is returned when a frequency weight vector has the wrong
	// length, contains a negative or non-finite entry, or is identically zero.
	ErrInvalidWeight = errors.New("mixedphase: invalid frequency weight")
	// ErrSingularSystem is returned when the weighted normal equations cannot be
	// factored even after regularisation.
	ErrSingularSystem = errors.New(
		"mixedphase: weighted normal equations are singular",
	)
	// ErrInvalidTolerance is returned when a negative magnitude tolerance is
	// given.
	ErrInvalidTolerance = errors.New(
		"mixedphase: magnitude tolerance must not be negative",
	)
	// ErrInvalidIterations is returned when a negative iteration or stage count
	// is given for a budget that has no "run until convergence" meaning.
	ErrInvalidIterations = errors.New(
		"mixedphase: iteration count must not be negative",
	)
	// ErrInvalidPenalty is returned when a negative constraint penalty is given.
	ErrInvalidPenalty = errors.New(
		"mixedphase: penalty must not be negative",
	)
	// ErrZeroResponse is returned when a reference or prescribed response has no
	// energy at all, which leaves every relative error undefined.
	ErrZeroResponse = errors.New(
		"mixedphase: response is identically zero",
	)
)

// MinimumPhaseMethod selects how a magnitude response is factored into a
// causal minimum-phase spectrum.
//
// Both methods evaluate the same Hilbert relation between log magnitude and
// phase on the same dense grid and therefore agree to within rounding on
// well-conditioned targets. They differ in how the result is assembled, which
// matters once the target spans a large dynamic range.
type MinimumPhaseMethod int

const (
	// MethodCepstrum folds the real cepstrum onto the causal half and
	// exponentiates the resulting complex log spectrum. Magnitude and phase
	// are both produced by the exponential, so the reconstructed magnitude
	// only approximates the target. This is the zero value.
	MethodCepstrum MinimumPhaseMethod = iota

	// MethodHilbert evaluates the discrete Hilbert transform of the log
	// magnitude to obtain the phase alone and pairs it with the (floored)
	// target magnitude. The magnitude is therefore reproduced exactly on the
	// design grid and never passes through an exponential.
	MethodHilbert
)

// String implements [fmt.Stringer].
func (m MinimumPhaseMethod) String() string {
	switch m {
	case MethodCepstrum:
		return "cepstrum"
	case MethodHilbert:
		return "hilbert"
	default:
		return "unknown"
	}
}

func (m MinimumPhaseMethod) valid() bool {
	return m == MethodCepstrum || m == MethodHilbert
}

// MinimumPhaseConfig configures [MinimumPhaseWith].
type MinimumPhaseConfig struct {
	// FFTSize controls the dense design grid. Zero selects a power of two at
	// least eight times the prototype length.
	FFTSize int

	// Epsilon is the magnitude floor applied before taking logarithms. Zero
	// selects a scale-relative default. Negative values are rejected.
	Epsilon float64

	// Method selects the reconstruction algorithm. The zero value is
	// [MethodCepstrum].
	Method MinimumPhaseMethod
}

// IterativeConfig configures the DAGA 2012 alternating factorisation.
type IterativeConfig struct {
	// Length is the number of taps in the resulting FIR. Zero uses the
	// prototype length.
	Length int

	// Delay is the group delay, in samples, contributed by the linear-phase
	// factor. The linear factor has length 2*Delay+1. Consequently Delay must
	// be in [0, (Length-1)/2].
	Delay int

	// Iterations is the maximum number of alternating correction passes.
	// Zero uses a default of 12. A negative value returns the initial
	// uncorrected factorisation, which is useful for comparisons.
	Iterations int

	// FFTSize controls the dense frequency grid. Zero selects a power of two
	// at least eight times the filter length.
	FFTSize int

	// Epsilon is the magnitude floor used by logarithms and regularised
	// spectral division. Zero selects a scale-relative default. Negative
	// values are rejected.
	Epsilon float64

	// Window selects the truncation window for both factors. The
	// minimum-phase part receives its right-hand slope; the linear-phase
	// residual receives the symmetric form. The zero value is rectangular.
	Window window.Type

	// WindowAlpha supplies the alpha or beta parameter for parametric
	// windows. Zero uses the window package default. Negative values are
	// rejected.
	WindowAlpha float64

	// Method selects the minimum-phase reconstruction used for the
	// minimum-phase factor. The zero value is [MethodCepstrum].
	Method MinimumPhaseMethod

	// ToleranceDB stops the iteration once the change in RMS magnitude error
	// falls below this value or the error starts rising. A rising pass is
	// discarded. Zero uses 1e-7 dB. A negative value disables early stopping.
	ToleranceDB float64
}

// PhaseInterpolationConfig configures the direct frequency-domain baseline.
type PhaseInterpolationConfig struct {
	// Length is the number of output taps. Zero uses the prototype length.
	Length int

	// Mix interpolates the unwrapped target phase across the whole phase
	// continuum for a fixed magnitude: zero is minimum phase, one is linear
	// phase with delay (Length-1)/2, and two is maximum phase. Values outside
	// [0, 2] are rejected with [ErrInvalidPhaseMix].
	//
	// Mix prescribes phase, so the realised group delay is a consequence of it
	// rather than an input. It rises from the target's minimum-phase group
	// delay at zero to (Length-1)/2 at one and on to its reflection at two.
	Mix float64

	// FFTSize controls the dense design grid. Zero selects a power of two at
	// least eight times the output length.
	FFTSize int

	// Epsilon is the magnitude floor used by minimum-phase reconstruction.
	// Zero selects a scale-relative default. Negative values are rejected.
	Epsilon float64

	// Method selects the minimum-phase reconstruction whose phase is
	// interpolated. The zero value is [MethodCepstrum].
	Method MinimumPhaseMethod
}

// ContinuumConfig configures [DesignContinuum].
type ContinuumConfig struct {
	// Length is the number of output taps. Zero uses the prototype length.
	Length int

	// TargetGroupDelay is the requested weighted mean group delay in samples,
	// and is the only knob of the method. It must lie in [0, Length-1]:
	// negative delay is not causal and Length-1 is maximum phase, beyond which
	// a filter of this support has no phase left to spend.
	//
	// Which of the four regimes serves the request depends on the target, not
	// on this field alone. The requested magnitude implies a minimum-phase
	// group delay tau_min, and prescribing phase can reach any delay in
	// [tau_min, Length-1-tau_min]. Requests inside that window cost latency
	// only; requests outside it are met by conceding magnitude accuracy. The
	// realised value is reported as [Result.AchievedGroupDelay] and the branch
	// taken as [Result.Regime].
	TargetGroupDelay float64

	// FFTSize controls the dense design grid. Zero selects a power of two at
	// least eight times the output length.
	FFTSize int

	// Epsilon is the magnitude floor used by minimum-phase reconstruction.
	// Zero selects a scale-relative default. Negative values are rejected.
	Epsilon float64

	// Method selects the minimum-phase reconstruction whose phase is
	// interpolated. The zero value is [MethodCepstrum].
	Method MinimumPhaseMethod

	// Iterations and PenaltyStages budget the optimiser that serves requests
	// outside the reachable window. Zero values select the same defaults as
	// [DesignLowGroupDelay]. As there, the iteration count is a second
	// delay-versus-accuracy dial rather than a convergence threshold, so
	// results should be quoted with the budget that produced them.
	Iterations    int
	PenaltyStages int

	// DelayWeight holds one non-negative weight per bin on [0, Nyquist], so
	// its length must be FFTSize/2+1. It selects the band whose group delay
	// TargetGroupDelay refers to. Nil uses the squared target magnitude, which
	// concentrates the request on the passband.
	//
	// The knob is only as meaningful as this band. Group delay is not a
	// sensible whole-band quantity when the response has deep stopbands or
	// spectral nulls, so a weight that does not mask them makes the requested
	// delay an average over bins where phase is not audible and not numerically
	// stable either.
	DelayWeight []float64
}

// ComplexLeastSquaresConfig configures [DesignComplexLeastSquares].
type ComplexLeastSquaresConfig struct {
	// Length is the number of output taps. Zero uses the prototype length.
	Length int

	// Mix interpolates the prescribed phase: zero is minimum phase, one is
	// linear phase with delay (Length-1)/2, and two is maximum phase. It shares
	// its meaning with [PhaseInterpolationConfig] so both designs approximate
	// the same target.
	Mix float64

	// FFTSize controls the dense design grid. Zero selects a power of two at
	// least eight times the output length.
	FFTSize int

	// Epsilon is the magnitude floor used by minimum-phase reconstruction.
	// Zero selects a scale-relative default. Negative values are rejected.
	Epsilon float64

	// Method selects the minimum-phase reconstruction whose phase is
	// interpolated. The zero value is [MethodCepstrum].
	Method MinimumPhaseMethod

	// Weight holds one non-negative weight per bin on [0, Nyquist], so its
	// length must be FFTSize/2+1. Nil selects uniform weighting, for which the
	// least-squares solution coincides with [DesignPhaseInterpolation].
	Weight []float64

	// MinimaxIterations is the number of Lawson reweighting passes applied
	// after the initial weighted least-squares solve. Zero performs none and
	// returns the pure least-squares design; positive values trade mean-square
	// error for a lower peak error.
	MinimaxIterations int

	// MinimaxTolerance stops the reweighting once the relative change of the
	// peak complex error falls below this value. Zero uses 1e-4. A negative
	// value disables early stopping.
	MinimaxTolerance float64
}

// LowGroupDelayConfig configures [DesignLowGroupDelay].
//
// The design minimises passband group delay subject to a magnitude constraint,
// so the tolerance is the primary control: a tighter band buys accuracy at the
// price of delay, a looser one the other way round.
type LowGroupDelayConfig struct {
	// Length is the number of output taps. Zero uses the prototype length.
	Length int

	// FFTSize controls the dense design grid. Zero selects a power of two at
	// least eight times the output length. The optimiser cost grows with the
	// product of grid size and Length, so a large grid is expensive here.
	FFTSize int

	// Epsilon is the magnitude floor used by the minimum-phase starting
	// point. Zero selects a scale-relative default. Negative values are
	// rejected.
	Epsilon float64

	// Method selects the minimum-phase reconstruction used for the default
	// starting point. The zero value is [MethodCepstrum].
	Method MinimumPhaseMethod

	// ToleranceDB is the permitted magnitude deviation from the prototype, in
	// dB. Zero uses 1 dB. Negative values are rejected. The permitted band
	// never narrows below a small fraction of the target peak, so bins where
	// the target is numerically zero stay feasible.
	ToleranceDB float64

	// DelayWeight holds one non-negative weight per bin on [0, Nyquist], so
	// its length must be FFTSize/2+1. It selects the band whose group delay is
	// minimised. Nil uses the squared target magnitude, which concentrates the
	// objective on the passband.
	DelayWeight []float64

	// InitialTaps is the starting point of the optimisation and must have
	// Length entries. Nil uses the truncated minimum-phase design. The problem
	// is non-convex, so this choice can change the result.
	InitialTaps []float64

	// Iterations is the maximum number of quasi-Newton iterations per penalty
	// stage. Zero uses 200. A negative value returns the starting point
	// unchanged, which is useful for comparisons.
	Iterations int

	// PenaltyStages is the number of times the constraint penalty is raised,
	// each stage multiplying it by ten. Zero uses six stages.
	PenaltyStages int

	// InitialPenalty scales the constraint term in the first stage. Zero uses
	// one. Negative values are rejected.
	InitialPenalty float64
}

// GroupDelayMetrics summarises the group delay of a design and how well it
// respects its magnitude constraint.
type GroupDelayMetrics struct {
	// Mean is the weight-averaged group delay in samples.
	Mean float64

	// Peak is the largest group delay, in samples, over the bins that carry at
	// least one percent of the largest delay weight.
	Peak float64

	// ConstraintViolation is the largest magnitude deviation that exceeds the
	// permitted band, expressed as a multiple of that band. Zero means the
	// design is feasible; 0.5 means the worst bin overshoots its tolerance by
	// half of it.
	ConstraintViolation float64
}

// ComplexErrorMetrics reports the approximation error against a prescribed
// complex response, relative to the peak magnitude of that response.
//
// Unlike [Metrics] these numbers include the phase error, which is what the
// least-squares and minimax objectives actually minimise.
type ComplexErrorMetrics struct {
	// RMS is the unweighted root-mean-square complex error over [0, Nyquist].
	RMS float64

	// Peak is the largest complex error over [0, Nyquist].
	Peak float64
}

// Result contains a designed FIR and method-specific intermediate data.
type Result struct {
	// Taps is the final causal FIR.
	Taps []float64

	// MinimumPhasePart and LinearPhasePart contain the two factors produced by
	// [DesignIterative]. They are nil for [DesignPhaseInterpolation].
	MinimumPhasePart []float64
	LinearPhasePart  []float64

	// Delay is the linear-phase delay budget the design used, in samples. It
	// is the configured value for [DesignIterative] and the selected one for
	// [DesignIterativeAuto], which is the field's reason for existing; the
	// designs that choose their own delay report it through GroupDelay
	// instead.
	Delay int

	// Iterations is the number of alternating correction passes accepted by
	// [DesignIterative], of reweighting passes performed by
	// [DesignComplexLeastSquares], or of accepted quasi-Newton steps performed
	// by [DesignLowGroupDelay].
	Iterations int

	// Metrics compares Taps with the prototype magnitude response.
	Metrics Metrics

	// ComplexError compares Taps with the prescribed complex response. It is
	// only populated by [DesignComplexLeastSquares]; the other designs do not
	// prescribe a phase over the whole grid.
	ComplexError ComplexErrorMetrics

	// GroupDelay summarises the optimised group delay and the feasibility of
	// the magnitude constraint. It is only populated by
	// [DesignLowGroupDelay] and [DesignContinuum].
	GroupDelay GroupDelayMetrics

	// Regime records which branch of the phase continuum produced Taps. It is
	// only populated by [DesignContinuum]; the zero value
	// [RegimeUnspecified] means no continuum dispatch took place.
	Regime ContinuumRegime

	// AchievedGroupDelay is the weighted mean group delay measured on Taps, in
	// samples. It is only populated by [DesignContinuum], where it is the
	// quantity the caller asked for and should be compared against the
	// request: inside the reachable window the two agree to the projection
	// residual, and outside it the difference is what the tap budget refused.
	AchievedGroupDelay float64
}

// ContinuumRegime names the branch of the phase continuum a [DesignContinuum]
// call took. The branches are separated by the reachable window
// [tau_min, Length-1-tau_min] that the requested magnitude implies.
type ContinuumRegime int

const (
	// RegimeUnspecified is the zero value, used by every design that does not
	// dispatch on a requested group delay.
	RegimeUnspecified ContinuumRegime = iota

	// RegimeSubMinimum is a request below the minimum-phase group delay. No
	// phase choice reaches it, so the magnitude gives way.
	RegimeSubMinimum

	// RegimeWindow is a request inside the reachable window, served by a
	// prescribed phase at the mix the affine delay law inverts to. The
	// magnitude is whatever a least-squares projection of that phase onto the
	// tap budget achieves.
	RegimeWindow

	// RegimeSuperMaximum is a request beyond maximum phase. It is the mirror
	// of [RegimeSubMinimum]: the sub-minimum solve runs on the reflected
	// request and the result is reversed.
	RegimeSuperMaximum
)

// String names the regime in the form used by the comparison artifacts.
func (r ContinuumRegime) String() string {
	switch r {
	case RegimeSubMinimum:
		return "sub-minimum"
	case RegimeWindow:
		return "window"
	case RegimeSuperMaximum:
		return "super-maximum"
	case RegimeUnspecified:
		return "unspecified"
	default:
		return "unknown"
	}
}

// Metrics describes spectral error and the time distribution of an FIR.
type Metrics struct {
	// RMSMagnitudeErrorDB and MaxMagnitudeErrorDB compare dB magnitudes on a
	// dense FFT grid with both responses floored 120 dB below the reference
	// peak.
	RMSMagnitudeErrorDB float64
	MaxMagnitudeErrorDB float64

	// RelativeMagnitudeError is the L2 norm of the linear-magnitude error
	// divided by the L2 norm of the reference magnitude. Unlike the dB
	// metrics, it is not dominated by differences deep in a stopband.
	RelativeMagnitudeError float64

	// PeakIndex is the index of the largest absolute coefficient.
	PeakIndex int

	// EnergyCentroid is the first moment of squared tap magnitude in samples.
	EnergyCentroid float64

	// PrePeakEnergyRatio is the fraction of total energy before PeakIndex.
	PrePeakEnergyRatio float64
}
