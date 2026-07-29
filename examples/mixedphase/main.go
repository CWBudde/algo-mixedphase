// Command mixedphase runs the fixed Phase 3 reference suite.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cwbudde/algo-mixedphase/internal/reference"
)

func main() {
	trials := flag.Int(
		"trials",
		3,
		"number of design runs used for the best runtime",
	)
	document := flag.String(
		"document",
		"",
		"Markdown document whose marked result table should be updated",
	)

	flag.Parse()

	rows, err := reference.Run(*trials)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *document != "" {
		if err := reference.UpdateMarkdownTable(*document, rows); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	if err := reference.WriteCSV(os.Stdout, rows); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
