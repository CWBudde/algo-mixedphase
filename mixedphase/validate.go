package mixedphase

import (
	"fmt"
	"math"
)

// validatePrototype rejects an empty prototype and any non-finite tap.
//
// Every design in this package reduces the prototype to a magnitude spectrum
// before doing anything else, and a single NaN there spreads to the peak used
// for the scale-relative magnitude floor. From that point on the floor itself
// is NaN, every comparison against it is false, and the design returns a slice
// of NaN taps with a nil error. Refusing the input is the only place the
// failure is still attributable.
func validatePrototype(prototype []float64) error {
	if len(prototype) == 0 {
		return ErrEmptyPrototype
	}

	for i, tap := range prototype {
		if math.IsNaN(tap) || math.IsInf(tap, 0) {
			return fmt.Errorf(
				"%w: tap %d is %v",
				ErrNonFinitePrototype,
				i,
				tap,
			)
		}
	}

	return nil
}

// validateFinite rejects a non-finite configuration value.
//
// Range checks of the form "value < low || value > high" admit NaN, because
// both comparisons are false for it. Fields guarded only that way therefore
// need this in front.
func validateFinite(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("%w: %s is %v", ErrNonFiniteConfig, name, value)
	}

	return nil
}

// field pairs a configuration value with the name to report it under.
type field struct {
	name  string
	value float64
}

// validateFiniteFields checks fields in the given order and reports the first
// offender.
//
// The order is fixed deliberately: ranging over a map would make the reported
// field depend on Go's randomised map iteration, so the same bad configuration
// could produce different error messages on different runs.
func validateFiniteFields(fields ...field) error {
	for _, current := range fields {
		if err := validateFinite(current.name, current.value); err != nil {
			return err
		}
	}

	return nil
}
