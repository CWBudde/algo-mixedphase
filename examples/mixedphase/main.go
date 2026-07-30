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
	continuum := flag.String(
		"continuum",
		"",
		"CSV path for the unified group-delay knob across both branches",
	)
	continuumImpulses := flag.String(
		"continuum-impulses",
		"",
		"CSV path for the impulse responses sampled along the continuum",
	)

	flag.Parse()

	if err := run(options{
		trials:            *trials,
		document:          *document,
		timings:           *timings,
		responses:         *responses,
		impulses:          *impulses,
		sweep:             *sweep,
		regimes:           *regimes,
		continuum:         *continuum,
		continuumImpulses: *continuumImpulses,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// options collects the artifact paths, which are all optional and all
// independent. They outgrew a positional parameter list.
type options struct {
	trials int

	document          string
	timings           string
	responses         string
	impulses          string
	sweep             string
	regimes           string
	continuum         string
	continuumImpulses string
}

func run(opts options) error {
	trials := opts.trials
	if opts.timings != "" && trials == 0 {
		trials = 1
	}

	rows, err := reference.Run(trials)
	if err != nil {
		return err
	}

	if opts.document != "" {
		if err := reference.UpdateMarkdownTable(opts.document, rows); err != nil {
			return err
		}
	}

	if opts.timings != "" {
		if err := writeTimings(opts.timings, rows, trials); err != nil {
			return err
		}
	}

	if err := writeResponses(opts); err != nil {
		return err
	}

	if opts.sweep != "" {
		sweepRows, sweepErr := reference.SweepRows()
		if sweepErr != nil {
			return sweepErr
		}

		if sweepErr := writeArtifact(
			opts.sweep,
			func(file *os.File) error {
				return reference.WriteSweepCSV(file, sweepRows)
			},
		); sweepErr != nil {
			return sweepErr
		}
	}

	if opts.regimes != "" {
		regimeRows, regimeErr := reference.RegimeRows()
		if regimeErr != nil {
			return regimeErr
		}

		if regimeErr := writeArtifact(
			opts.regimes,
			func(file *os.File) error {
				return reference.WriteRegimesCSV(file, regimeRows)
			},
		); regimeErr != nil {
			return regimeErr
		}
	}

	if err := writeContinuum(opts); err != nil {
		return err
	}

	return reference.WriteCSV(os.Stdout, rows)
}

// writeResponses emits the representative response and impulse artifacts, which
// share one design pass.
func writeResponses(opts options) error {
	if opts.responses == "" && opts.impulses == "" {
		return nil
	}

	frequencyRows, impulseRows, err := reference.RepresentativeResponses()
	if err != nil {
		return err
	}

	if opts.responses != "" {
		if err := writeArtifact(
			opts.responses,
			func(file *os.File) error {
				return reference.WriteResponseCSV(file, frequencyRows)
			},
		); err != nil {
			return err
		}
	}

	if opts.impulses != "" {
		if err := writeArtifact(
			opts.impulses,
			func(file *os.File) error {
				return reference.WriteImpulseCSV(file, impulseRows)
			},
		); err != nil {
			return err
		}
	}

	return nil
}

// writeContinuum emits the unified-knob artifacts.
func writeContinuum(opts options) error {
	if opts.continuum != "" {
		continuumRows, err := reference.ContinuumRows()
		if err != nil {
			return err
		}

		if err := writeArtifact(
			opts.continuum,
			func(file *os.File) error {
				return reference.WriteContinuumCSV(file, continuumRows)
			},
		); err != nil {
			return err
		}
	}

	if opts.continuumImpulses == "" {
		return nil
	}

	impulseRows, err := reference.ContinuumImpulseRows()
	if err != nil {
		return err
	}

	return writeArtifact(
		opts.continuumImpulses,
		func(file *os.File) error {
			return reference.WriteContinuumImpulseCSV(file, impulseRows)
		},
	)
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
