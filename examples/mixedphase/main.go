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

	flag.Parse()

	if err := run(*trials, *document, *timings); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(trials int, document, timings string) error {
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

	return reference.WriteCSV(os.Stdout, rows)
}

func writeTimings(path string, rows []reference.Row, trials int) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create timings CSV: %w", err)
	}

	machine := runtime.GOOS + "/" + runtime.GOARCH

	if err := reference.WriteTimingsCSV(
		file,
		rows,
		machine,
		runtime.Version(),
		trials,
	); err != nil {
		_ = file.Close()

		return err
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close timings CSV: %w", err)
	}

	return nil
}
