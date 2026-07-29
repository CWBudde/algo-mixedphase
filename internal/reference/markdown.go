package reference

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

const (
	tableStart = "<!-- reference-results:start -->"
	tableEnd   = "<!-- reference-results:end -->"
)

// UpdateMarkdownTable replaces the marked reference table in path with rows.
func UpdateMarkdownTable(path string, rows []Row) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reference: read Markdown document: %w", err)
	}

	start := bytes.Index(content, []byte(tableStart))
	end := bytes.Index(content, []byte(tableEnd))

	if start < 0 || end < 0 || end <= start {
		return fmt.Errorf(
			"reference: Markdown document is missing ordered table markers",
		)
	}

	replacement := tableStart + "\n\n" + markdownTable(rows) + "\n" + tableEnd
	updated := make([]byte, 0, len(content)+len(replacement))
	updated = append(updated, content[:start]...)
	updated = append(updated, replacement...)
	updated = append(updated, content[end+len(tableEnd):]...)

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("reference: stat Markdown document: %w", err)
	}

	if err := os.WriteFile(path, updated, info.Mode().Perm()); err != nil {
		return fmt.Errorf("reference: write Markdown document: %w", err)
	}

	return nil
}

func markdownTable(rows []Row) string {
	header := []string{
		"Target",
		"Method",
		"Rel. error",
		"Mean delay",
		"Pre-peak",
	}
	tableRows := make([][]string, 0, len(rows))

	widths := make([]int, len(header))
	for index, value := range header {
		widths[index] = len(value)
	}

	previousTarget := ""

	for _, row := range rows {
		target := ""
		if row.Target != previousTarget {
			target = displayName(row.Target)
			previousTarget = row.Target
		}

		values := []string{
			target,
			displayName(row.Method),
			fmt.Sprintf("%.5f%%", 100*row.RelativeMagnitudeError),
			fmt.Sprintf("%.2f", row.MeanGroupDelay),
			fmt.Sprintf("%.2f%%", 100*row.PrePeakEnergyRatio),
		}
		tableRows = append(tableRows, values)

		for index, value := range values {
			widths[index] = max(widths[index], len(value))
		}
	}

	var output strings.Builder

	writeMarkdownRow(&output, header, widths)
	writeMarkdownSeparator(&output, widths)

	for _, values := range tableRows {
		writeMarkdownRow(&output, values, widths)
	}

	return output.String()
}

func writeMarkdownRow(
	output *strings.Builder,
	values []string,
	widths []int,
) {
	output.WriteString("| ")

	for index, value := range values {
		if index >= 2 {
			_, _ = fmt.Fprintf(output, "%*s", widths[index], value)
		} else {
			_, _ = fmt.Fprintf(output, "%-*s", widths[index], value)
		}

		if index == len(values)-1 {
			output.WriteString(" |\n")
		} else {
			output.WriteString(" | ")
		}
	}
}

func writeMarkdownSeparator(output *strings.Builder, widths []int) {
	output.WriteString("| ")

	for index, width := range widths {
		if index >= 2 {
			output.WriteString(strings.Repeat("-", width-1))
			output.WriteByte(':')
		} else {
			output.WriteByte(':')
			output.WriteString(strings.Repeat("-", width-1))
		}

		if index == len(widths)-1 {
			output.WriteString(" |\n")
		} else {
			output.WriteString(" | ")
		}
	}
}

func displayName(name string) string {
	switch name {
	case "budde-iterative":
		return "Budde iterative"
	case "phase-interpolation":
		return "phase interpolation"
	case "complex-minimax":
		return "complex minimax"
	case "low-group-delay":
		return "low group delay"
	case "parametric-eq":
		return "parametric EQ"
	case "deep-notch":
		return "deep notch"
	case "room-correction":
		return "room correction"
	default:
		return name
	}
}
