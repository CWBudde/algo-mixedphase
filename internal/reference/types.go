// Package reference implements the fixed-target comparison used by the
// repository documentation.
package reference

import "time"

const (
	// SampleRate is shared by every reference target.
	SampleRate = 48000
	// TapCount is the fixed output support shared by every design.
	TapCount = 129
	// FFTSize is the fixed design and analysis grid.
	FFTSize = 1024
	// DelayBudget is the linear-phase delay requested from the three
	// phase-controlled designs. The low-group-delay method chooses its own.
	DelayBudget = 16
)

const (
	iterativePasses       = 12
	minimaxPasses         = 16
	lowDelayIterations    = 80
	lowDelayPenaltyStages = 4
	lowDelayToleranceDB   = 2.0
)

// Target is one fixed magnitude-response fixture and the frequency weights
// used to assess its group delay.
type Target struct {
	Name        string
	Prototype   []float64
	DelayWeight []float64
}

// Row is one method/target result in the committed reference CSV.
type Row struct {
	Target                 string
	Method                 string
	SampleRate             int
	Taps                   int
	FFTSize                int
	DelayBudget            int
	Iterations             int
	Runtime                time.Duration
	RelativeMagnitudeError float64
	RMSMagnitudeErrorDB    float64
	MaxMagnitudeErrorDB    float64
	MeanGroupDelay         float64
	GroupDelayRipple       float64
	PeakGroupDelay         float64
	PeakIndex              int
	EnergyCentroid         float64
	PrePeakEnergyRatio     float64
	CoefficientPeak        float64
	CoefficientRangeDB     float64
	ConstraintViolation    float64
}
