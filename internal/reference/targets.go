package reference

import (
	"bufio"
	_ "embed"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	algofft "github.com/cwbudde/algo-fft"
)

const (
	prototypeLength = 257
	targetFFTSize   = 4096
)

//go:embed testdata/room-response.csv
var roomResponseCSV string

type magnitudeCurve func(float64) float64

type roomPoint struct {
	frequency float64
	response  float64
}

// Targets constructs the fixed reference targets at the published budgets.
//
// The first five are smooth curves whose minimum-phase factor fits comfortably
// inside the tap budget. steep-crossover does not, and that is the point of
// including it: see its definition below.
func Targets() ([]Target, error) {
	return targetsFor(prototypeLength, targetFFTSize, FFTSize)
}

// targetsFor constructs the same six curves at an arbitrary prototype length and
// on arbitrary grids.
//
// The curves themselves are analytic and carry no length, so a longer prototype
// represents the same physical response more completely rather than a different
// one. That is what makes a length sweep meaningful: at 257 taps an eighth-order
// 800 Hz crossover is itself already truncated, so a 513-tap design would be
// measured against a target shorter than the filter and every method would score
// alike.
//
//   - prototypeTaps is the fixture length. It must not exceed targetFFT.
//   - targetFFT is the grid the zero-phase magnitude is sampled and inverted on.
//   - weightGrid is the grid the delay and magnitude weights are built for, and
//     must match the grid the designs and the analysis run on.
func targetsFor(prototypeTaps, targetFFT, weightGrid int) ([]Target, error) {
	if prototypeTaps <= 0 || targetFFT <= 0 || weightGrid <= 0 {
		return nil, fmt.Errorf(
			"reference: prototype %d, target grid %d and weight grid %d must all be positive",
			prototypeTaps,
			targetFFT,
			weightGrid,
		)
	}

	if prototypeTaps > targetFFT {
		return nil, fmt.Errorf(
			"reference: prototype length %d exceeds the target grid %d",
			prototypeTaps,
			targetFFT,
		)
	}

	roomPoints, err := parseRoomResponse(roomResponseCSV)
	if err != nil {
		return nil, err
	}

	definitions := []struct {
		name      string
		curve     magnitudeCurve
		delayBand func(float64) bool
	}{
		{
			name:  "low-pass",
			curve: firstOrderLowPass(1000),
			delayBand: func(frequency float64) bool {
				return frequency <= 1000
			},
		},
		{
			name:  "parametric-eq",
			curve: parametricEQ(3000, 9, 0.18),
			delayBand: func(frequency float64) bool {
				return frequency >= 1800 && frequency <= 5000
			},
		},
		{
			name:  "crossover",
			curve: linkwitzRileyLowPass(2000),
			delayBand: func(frequency float64) bool {
				return frequency <= 1500
			},
		},
		{
			name:  "deep-notch",
			curve: deepNotch(6000, -60, 0.10),
			delayBand: func(frequency float64) bool {
				return frequency >= 4000 && frequency <= 9000
			},
		},
		{
			name:  "room-correction",
			curve: roomCorrection(roomPoints),
			delayBand: func(frequency float64) bool {
				return frequency >= 20 && frequency <= 20000
			},
		},
		// steep-crossover is the one target that starves the minimum-phase
		// factor.
		//
		// The five curves above are smooth enough that a minimum-phase filter
		// reproduces them inside the Length-2*Delay taps the split leaves it.
		// When that happens the residual quotient is unit-magnitude, its
		// zero-phase inverse transform is a unit impulse, and the alternating
		// correction has nothing to do: the design degenerates to a delayed
		// minimum-phase filter and every reported number describes a filter the
		// method never shaped.
		//
		// An eighth-order crossover at 800 Hz has a minimum-phase impulse
		// response longer than that budget, so the linear factor has to carry
		// real shaping. It is the only target here that exercises the method
		// the repository exists to evaluate.
		{
			name:  "steep-crossover",
			curve: linkwitzRileyOrder(800, 8),
			delayBand: func(frequency float64) bool {
				return frequency <= 600
			},
		},
	}

	targets := make([]Target, 0, len(definitions))
	for _, definition := range definitions {
		prototype, buildErr := buildPrototype(
			definition.curve,
			prototypeTaps,
			targetFFT,
		)
		if buildErr != nil {
			return nil, fmt.Errorf(
				"reference: build %s target: %w",
				definition.name,
				buildErr,
			)
		}

		targets = append(targets, Target{
			Name:      definition.name,
			Prototype: prototype,
			DelayWeight: buildDelayWeight(
				definition.curve,
				definition.delayBand,
				weightGrid,
			),
			MagnitudeWeight: buildMagnitudeWeight(definition.curve, weightGrid),
		})
	}

	return targets, nil
}

func firstOrderLowPass(cutoff float64) magnitudeCurve {
	return func(frequency float64) float64 {
		ratio := frequency / cutoff

		return 1 / math.Sqrt(1+ratio*ratio)
	}
}

func parametricEQ(
	centre, gainDB, octaveSigma float64,
) magnitudeCurve {
	return func(frequency float64) float64 {
		if frequency == 0 {
			return 1
		}

		distance := math.Log2(frequency / centre)
		gain := gainDB * math.Exp(
			-0.5*distance*distance/(octaveSigma*octaveSigma),
		)

		return math.Pow(10, gain/20)
	}
}

func linkwitzRileyLowPass(cutoff float64) magnitudeCurve {
	return linkwitzRileyOrder(cutoff, 4)
}

// linkwitzRileyOrder generalises the crossover magnitude to any even order.
//
// The order controls how long the corresponding minimum-phase impulse response
// is, which is the property that decides whether a fixed tap budget can hold
// it.
func linkwitzRileyOrder(cutoff float64, order int) magnitudeCurve {
	return func(frequency float64) float64 {
		ratio := frequency / cutoff

		return 1 / (1 + math.Pow(ratio, float64(order)))
	}
}

func deepNotch(
	centre, depthDB, octaveSigma float64,
) magnitudeCurve {
	return func(frequency float64) float64 {
		if frequency == 0 {
			return 1
		}

		distance := math.Log2(frequency / centre)
		gain := depthDB * math.Exp(
			-0.5*distance*distance/(octaveSigma*octaveSigma),
		)

		return math.Pow(10, gain/20)
	}
}

func roomCorrection(points []roomPoint) magnitudeCurve {
	return func(frequency float64) float64 {
		responseDB := interpolateRoomResponse(points, frequency)
		correctionDB := max(-12.0, min(12.0, -responseDB))

		return math.Pow(10, correctionDB/20)
	}
}

// buildPrototype samples a zero-phase magnitude curve, transforms it, and
// windows the result to the requested prototype length.
//
// Every target here is zero-phase. That is a deliberate limitation rather than
// an oversight: none of the compared designs fits a prescribed excess phase —
// each one synthesises phase from the magnitude alone — so a target carrying
// excess phase would produce identical rows for every method and measure
// nothing. See the failure-mode notes in the package docs.
func buildPrototype(
	curve magnitudeCurve,
	length, gridSize int,
) ([]float64, error) {
	plan, err := algofft.NewPlan64(gridSize)
	if err != nil {
		return nil, fmt.Errorf("create target FFT plan: %w", err)
	}

	spectrum := make([]complex128, gridSize)

	for bin := range gridSize/2 + 1 {
		frequency := float64(bin) * SampleRate / float64(gridSize)
		spectrum[bin] = complex(curve(frequency), 0)

		if bin > 0 && bin < gridSize/2 {
			spectrum[gridSize-bin] = spectrum[bin]
		}
	}

	periodic := make([]complex128, gridSize)
	if err := plan.Inverse(periodic, spectrum); err != nil {
		return nil, fmt.Errorf("inverse target FFT: %w", err)
	}

	prototype := make([]float64, length)
	middle := length / 2

	for index := range prototype {
		offset := index - middle

		source := offset
		if source < 0 {
			source += gridSize
		}

		window := 0.5 - 0.5*math.Cos(
			2*math.Pi*float64(index)/float64(length-1),
		)
		prototype[index] = real(periodic[source]) * window
	}

	wantedDC := curve(0)

	actualDC := 0.0
	for _, value := range prototype {
		actualDC += value
	}

	if actualDC == 0 {
		return nil, fmt.Errorf("target has zero DC gain")
	}

	scale := wantedDC / actualDC
	for index := range prototype {
		prototype[index] *= scale
	}

	return prototype, nil
}

func buildDelayWeight(
	curve magnitudeCurve,
	inBand func(float64) bool,
	gridSize int,
) []float64 {
	weight := make([]float64, gridSize/2+1)
	for bin := range weight {
		frequency := float64(bin) * SampleRate / float64(gridSize)
		if inBand(frequency) {
			magnitude := curve(frequency)
			weight[bin] = magnitude * magnitude
		}
	}

	return weight
}

// magnitudeWeightFloor bounds how much a deep stopband can be emphasised.
//
// The weight is the reciprocal of the target magnitude, so a 60 dB notch would
// otherwise be weighted a thousand times more heavily than the passband and the
// design would spend everything on a band nobody hears. Flooring the magnitude
// at 1e-3 of its peak caps the ratio at 60 dB.
const magnitudeWeightFloor = 1e-3

// buildMagnitudeWeight weights each bin by the inverse target magnitude.
//
// This is the weighting the mixedphase package documents for an absolute
// complex objective: without it, a design minimising |H_wanted - H| treats an
// error in a -60 dB stopband as a thousand times less important than the same
// absolute error in the passband, and the stopband collapses.
func buildMagnitudeWeight(curve magnitudeCurve, gridSize int) []float64 {
	weight := make([]float64, gridSize/2+1)

	peak := 0.0
	for bin := range weight {
		peak = max(peak, curve(float64(bin)*SampleRate/FFTSize))
	}

	if peak == 0 {
		for bin := range weight {
			weight[bin] = 1
		}

		return weight
	}

	for bin := range weight {
		magnitude := curve(float64(bin) * SampleRate / FFTSize)
		weight[bin] = peak / max(magnitude, magnitudeWeightFloor*peak)
	}

	return weight
}

func parseRoomResponse(input string) ([]roomPoint, error) {
	scanner := bufio.NewScanner(strings.NewReader(input))

	var records strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "#") {
			records.WriteString(line)
			records.WriteByte('\n')
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reference: read room response: %w", err)
	}

	reader := csv.NewReader(strings.NewReader(records.String()))
	if _, err := reader.Read(); err != nil {
		return nil, fmt.Errorf("reference: read room response header: %w", err)
	}

	var points []roomPoint

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("reference: read room response row: %w", err)
		}

		frequency, err := strconv.ParseFloat(record[0], 64)
		if err != nil {
			return nil, fmt.Errorf(
				"reference: parse room frequency %q: %w",
				record[0],
				err,
			)
		}

		response, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			return nil, fmt.Errorf(
				"reference: parse room response %q: %w",
				record[1],
				err,
			)
		}

		points = append(points, roomPoint{
			frequency: frequency,
			response:  response,
		})
	}

	if len(points) < 2 {
		return nil, fmt.Errorf("reference: room response needs at least two rows")
	}

	return points, nil
}

func interpolateRoomResponse(points []roomPoint, frequency float64) float64 {
	if frequency <= points[0].frequency {
		return points[0].response
	}

	last := points[len(points)-1]
	if frequency >= last.frequency {
		return last.response
	}

	for index := 1; index < len(points); index++ {
		upper := points[index]
		if frequency > upper.frequency {
			continue
		}

		lower := points[index-1]
		position := math.Log(frequency/lower.frequency) /
			math.Log(upper.frequency/lower.frequency)

		return lower.response + position*(upper.response-lower.response)
	}

	return last.response
}
