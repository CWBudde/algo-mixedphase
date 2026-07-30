// Command mixedphase runs the fixed Phase 3 reference suite.
//
// The default invocation is deterministic: it takes no timings, so its CSV is
// byte-identical across runs on a fixed platform and toolchain and can serve as
// the committed regression golden. It is not byte-identical across build
// targets — a js/wasm build moves several low-group-delay rows — so the gate
// only holds for the platform CI runs on. Pass -timings to additionally measure
// runtime into a separate, deliberately non-reproducible artifact.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/cwbudde/algo-mixedphase/internal/reference"
)

func main() {
	trials := flag.Int(
		"trials",
		0,
		"number of design runs used for the best runtime; "+
			"zero skips timing entirely and keeps the output deterministic",
	)
	document := flag.String(
		"document",
		"",
		"Markdown document whose marked result table should be updated",
	)
	timings := flag.String(
		"timings",
		"",
		"CSV path for wall-clock measurements; implies at least one trial",
	)
	responses := flag.String(
		"responses",
		"",
		"CSV path for the representative realised frequency responses",
	)
	impulses := flag.String(
		"impulses",
		"",
		"CSV path for the representative peak-aligned impulse responses",
	)
	sweep := flag.String(
		"sweep",
		"",
		"CSV path for the output-length and delay-budget sweep",
	)
	regimes := flag.String(
		"regimes",
		"",
		"CSV path for the phase-continuum and group-delay-floor regimes",
	)

	flag.Parse()

	if err := run(
		*trials,
		*document,
		*timings,
		*responses,
		*impulses,
		*sweep,
		*regimes,
	); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(
	trials int,
	document, timings, responses, impulses, sweep, regimes string,
) error {
	if timings != "" && trials == 0 {
		trials = 1
	}

	rows, err := reference.Run(trials)
	if err != nil {
		return err
	}

	if document != "" {
		if err := reference.UpdateMarkdownTable(document, rows); err != nil {
			return err
		}
	}

	if timings != "" {
		if err := writeTimings(timings, rows, trials); err != nil {
			return err
		}
	}

	if responses != "" || impulses != "" {
		frequencyRows, impulseRows, responseErr := reference.RepresentativeResponses()
		if responseErr != nil {
			return responseErr
		}

		if responses != "" {
			if responseErr := writeArtifact(
				responses,
				func(file *os.File) error {
					return reference.WriteResponseCSV(file, frequencyRows)
				},
			); responseErr != nil {
				return responseErr
			}
		}

		if impulses != "" {
			if responseErr := writeArtifact(
				impulses,
				func(file *os.File) error {
					return reference.WriteImpulseCSV(file, impulseRows)
				},
			); responseErr != nil {
				return responseErr
			}
		}
	}

	if sweep != "" {
		sweepRows, sweepErr := reference.SweepRows()
		if sweepErr != nil {
			return sweepErr
		}

		if sweepErr := writeArtifact(
			sweep,
			func(file *os.File) error {
				return reference.WriteSweepCSV(file, sweepRows)
			},
		); sweepErr != nil {
			return sweepErr
		}
	}

	if regimes != "" {
		regimeRows, regimeErr := reference.RegimeRows()
		if regimeErr != nil {
			return regimeErr
		}

		if regimeErr := writeArtifact(
			regimes,
			func(file *os.File) error {
				return reference.WriteRegimesCSV(file, regimeRows)
			},
		); regimeErr != nil {
			return regimeErr
		}
	}

	return reference.WriteCSV(os.Stdout, rows)
}

func writeTimings(path string, rows []reference.Row, trials int) error {
	machine := runtime.GOOS + "/" + runtime.GOARCH

	return writeArtifact(path, func(file *os.File) error {
		return reference.WriteTimingsCSV(
			file,
			rows,
			machine,
			runtime.Version(),
			trials,
		)
	})
}

func writeArtifact(
	path string,
	write func(*os.File) error,
) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create CSV %s: %w", path, err)
	}

	if err := write(file); err != nil {
		_ = file.Close()

		return err
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close CSV %s: %w", path, err)
	}

	return nil
}
