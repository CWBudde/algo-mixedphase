package reference

import (
	"bufio"
	_ "embed"
	"encoding/csv"
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

// Targets constructs the five fixed Phase 3 reference targets.
func Targets() ([]Target, error) {
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
	}

	targets := make([]Target, 0, len(definitions))
	for _, definition := range definitions {
		prototype, buildErr := buildPrototype(definition.curve)
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
			),
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
	return func(frequency float64) float64 {
		ratio := frequency / cutoff

		return 1 / (1 + math.Pow(ratio, 4))
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

func buildPrototype(curve magnitudeCurve) ([]float64, error) {
	plan, err := algofft.NewPlan64(targetFFTSize)
	if err != nil {
		return nil, fmt.Errorf("create target FFT plan: %w", err)
	}

	spectrum := make([]complex128, targetFFTSize)
	for bin := range targetFFTSize/2 + 1 {
		frequency := float64(bin) * SampleRate / targetFFTSize
		spectrum[bin] = complex(curve(frequency), 0)

		if bin > 0 && bin < targetFFTSize/2 {
			spectrum[targetFFTSize-bin] = spectrum[bin]
		}
	}

	periodic := make([]complex128, targetFFTSize)
	if err := plan.Inverse(periodic, spectrum); err != nil {
		return nil, fmt.Errorf("inverse target FFT: %w", err)
	}

	prototype := make([]float64, prototypeLength)
	middle := prototypeLength / 2

	for index := range prototype {
		offset := index - middle

		source := offset
		if source < 0 {
			source += targetFFTSize
		}

		window := 0.5 - 0.5*math.Cos(
			2*math.Pi*float64(index)/float64(prototypeLength-1),
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
) []float64 {
	weight := make([]float64, FFTSize/2+1)
	for bin := range weight {
		frequency := float64(bin) * SampleRate / FFTSize
		if inBand(frequency) {
			magnitude := curve(frequency)
			weight[bin] = magnitude * magnitude
		}
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
		if err == io.EOF {
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
