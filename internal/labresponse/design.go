package labresponse

import (
	"errors"
	"fmt"
	"sync"

	"github.com/cwbudde/algo-mixedphase/internal/reference"
	"github.com/cwbudde/algo-mixedphase/mixedphase"
)

// LowpassTarget names the lab's own adjustable fixture: a Hann-windowed sinc
// whose cutoff the visitor moves. It is the only target whose prototype depends
// on the requested tap budget, and the only one for which Request.Cutoff means
// anything.
const LowpassTarget = "lowpass"

// ErrUnknownTarget is returned for a target name the lab does not publish.
var ErrUnknownTarget = errors.New("labresponse: unknown target")

// ErrUnknownMethod is returned for a method name the lab does not publish.
var ErrUnknownMethod = errors.New("labresponse: unknown method")

// Request is one design the lab asks for. It mirrors the object the page sends
// across the WebAssembly boundary, so every field is a value the URL can carry.
type Request struct {
	Method      string
	Target      string
	Length      int
	Cutoff      float64
	Delay       int
	ToleranceDB float64
	Iterations  int
}

// Result is one completed lab design together with the curves the page draws.
type Result struct {
	Result mixedphase.Result

	// Realised is evaluated from the returned taps, not from the design grid,
	// so the page shows the filter that would actually run.
	Realised Response

	// Prototype is the same evaluation applied to the target, drawn as the
	// reference curve.
	Prototype Response
}

// targetFixture is a prototype the lab can design against, together with the
// frequency weights and grid it must be driven on.
type targetFixture struct {
	prototype       []float64
	delayWeight     []float64
	magnitudeWeight []float64

	// fftSize is the grid the two weights are sized for. It is zero for the
	// lab's own low-pass, which carries no weights and lets each designer pick
	// its own grid.
	fftSize int

	// penaltyStages mirrors the published low-delay budget. Zero keeps the
	// designer's own default.
	penaltyStages int
}

// referenceTargets builds the published comparison fixtures once.
//
// Each one costs a 4096-point transform to construct, and the lab redesigns on
// every slider movement, so building them per request would put six transforms
// in front of every keystroke.
var referenceTargets = sync.OnceValues(func() (map[string]targetFixture, error) {
	targets, err := reference.Targets()
	if err != nil {
		return nil, fmt.Errorf("labresponse: build reference targets: %w", err)
	}

	fixtures := make(map[string]targetFixture, len(targets))
	for _, target := range targets {
		fixtures[target.Name] = targetFixture{
			prototype:       target.Prototype,
			delayWeight:     target.DelayWeight,
			magnitudeWeight: target.MagnitudeWeight,
			fftSize:         reference.FFTSize,
			penaltyStages:   reference.LowDelayPenaltyStages,
		}
	}

	return fixtures, nil
})

// TargetNames lists every target the lab accepts, the adjustable low-pass
// first and then the published comparison fixtures in their harness order.
//
// The page keeps its own copy of this list so that it can validate a shared URL
// before the WebAssembly engine has loaded. The browser smoke test drives every
// entry of that copy through this package, which is what keeps the two in step.
func TargetNames() ([]string, error) {
	targets, err := reference.Targets()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(targets)+1)
	names = append(names, LowpassTarget)

	for _, target := range targets {
		names = append(names, target.Name)
	}

	return names, nil
}

// MethodNames lists every method name Design accepts, in the order the page
// offers them.
func MethodNames() []string {
	return []string{"iterative", "interpolation", "minimax", "lowdelay"}
}

// fixtureFor resolves a request's target to its prototype and weights.
func fixtureFor(request Request) (targetFixture, error) {
	if request.Target == "" || request.Target == LowpassTarget {
		// The lab is an untrusted boundary: a negative length would panic
		// inside make and tear the WebAssembly instance down, and a single tap
		// would divide by zero and emit NaN curves. Reject both here.
		prototype, err := LowpassPrototype(request.Length, request.Cutoff)
		if err != nil {
			return targetFixture{}, err
		}

		return targetFixture{prototype: prototype}, nil
	}

	fixtures, err := referenceTargets()
	if err != nil {
		return targetFixture{}, err
	}

	fixture, ok := fixtures[request.Target]
	if !ok {
		return targetFixture{}, fmt.Errorf(
			"%w: %q; known targets are %v",
			ErrUnknownTarget,
			request.Target,
			mustTargetNames(),
		)
	}

	return fixture, nil
}

func mustTargetNames() []string {
	names, err := TargetNames()
	if err != nil {
		return []string{LowpassTarget}
	}

	return names
}

// Design runs one lab request and evaluates the curves the page draws.
//
// A request naming a published comparison target is driven on the harness grid
// and with the harness weights, so a lab result at the published tap and delay
// budget reproduces the matching row of docs/reference-results.csv exactly. The
// adjustable low-pass keeps its historical unweighted treatment so that URLs
// shared before the comparison targets existed still recreate their design.
func Design(request Request) (Result, error) {
	fixture, err := fixtureFor(request)
	if err != nil {
		return Result{}, err
	}

	length := request.Length
	if length <= 0 {
		length = len(fixture.prototype)
	}

	maximumDelay := (length - 1) / 2
	delay := min(request.Delay, maximumDelay)

	mix := 0.0
	if maximumDelay > 0 {
		mix = min(float64(delay)/float64(maximumDelay), 1)
	}

	var result mixedphase.Result

	switch request.Method {
	case "iterative":
		result, err = mixedphase.DesignIterative(
			fixture.prototype,
			mixedphase.IterativeConfig{
				Length:     length,
				Delay:      delay,
				Iterations: request.Iterations,
				FFTSize:    fixture.fftSize,
			},
		)
	case "interpolation":
		result, err = mixedphase.DesignPhaseInterpolation(
			fixture.prototype,
			mixedphase.PhaseInterpolationConfig{
				Length:  length,
				Mix:     mix,
				FFTSize: fixture.fftSize,
			},
		)
	case "minimax":
		result, err = mixedphase.DesignComplexLeastSquares(
			fixture.prototype,
			mixedphase.ComplexLeastSquaresConfig{
				Length:            length,
				Mix:               mix,
				MinimaxIterations: request.Iterations,
				FFTSize:           fixture.fftSize,
				// Unweighted, this design coincides with phase interpolation
				// and lets stopband depth slip; the published comparison
				// weights it by the inverse target magnitude for exactly that
				// reason. Only the comparison fixtures carry that weight, so
				// the low-pass keeps the behaviour its shared URLs recorded.
				Weight: fixture.magnitudeWeight,
			},
		)
	case "lowdelay":
		result, err = mixedphase.DesignLowGroupDelay(
			fixture.prototype,
			// FFTSize is left at its default for the low-pass so that the grid
			// always oversamples the requested length; a fixed size would be
			// too coarse to measure against once the visitor asks for a long
			// filter.
			mixedphase.LowGroupDelayConfig{
				Length:        length,
				ToleranceDB:   request.ToleranceDB,
				Iterations:    request.Iterations,
				FFTSize:       fixture.fftSize,
				DelayWeight:   fixture.delayWeight,
				PenaltyStages: fixture.penaltyStages,
			},
		)
	default:
		return Result{}, fmt.Errorf(
			"%w: %q; known methods are %v",
			ErrUnknownMethod,
			request.Method,
			MethodNames(),
		)
	}

	if err != nil {
		return Result{}, err
	}

	return Result{
		Result:    result,
		Realised:  New(result.Taps),
		Prototype: New(fixture.prototype),
	}, nil
}
