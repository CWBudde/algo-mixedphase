// Command mixedphase runs the fixed Phase 3 reference suite.
//
// The default invocation is deterministic: it takes no timings, so its CSV is
// byte-identical across runs and machines and can serve as the committed
// regression golden. Pass -timings to additionally measure runtime into a
// separate, deliberately non-reproducible artifact.
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

	flag.Parse()

	if err := run(
		*trials,
		*document,
		*timings,
		*responses,
		*impulses,
	); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(
	trials int,
	document, timings, responses, impulses string,
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
		frequencyRows, impulseRows, responseErr :=
			reference.RepresentativeResponses()
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
