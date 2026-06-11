package report

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func writeSectionTable(b *strings.Builder, sections []core.SectionResult) {
	headers := []string{"Check", "Status", "Findings"}
	rows, widths := sectionRows(sections, headers)

	b.WriteString(renderDivider(widths))
	b.WriteByte('\n')
	b.WriteString(renderRow(headers, widths))
	b.WriteByte('\n')
	b.WriteString(renderDivider(widths))
	b.WriteByte('\n')
	for _, row := range rows {
		b.WriteString(renderRow(row, widths))
		b.WriteByte('\n')
	}
	b.WriteString(renderDivider(widths))
	b.WriteString("\n\n")
}

func sectionRows(sections []core.SectionResult, headers []string) ([][]string, []int) {
	rows := make([][]string, 0, len(sections))
	widths := []int{len(headers[0]), len(headers[1]), len(headers[2])}
	for _, section := range sections {
		row := []string{section.Name, statusCell(section.Status), fmt.Sprintf("%d", len(section.Findings))}
		rows = append(rows, row)
		updateWidths(widths, row)
	}
	return rows, widths
}

func updateWidths(widths []int, row []string) {
	for i, cell := range row {
		cellWidth := printableWidth(cell)
		if cellWidth > widths[i] {
			widths[i] = cellWidth
		}
	}
}

func renderRow(cells []string, widths []int) string {
	padded := make([]string, 0, len(cells))
	for i, cell := range cells {
		padded = append(padded, " "+cell+strings.Repeat(" ", widths[i]-printableWidth(cell))+" ")
	}
	return "|" + strings.Join(padded, "|") + "|"
}

func renderDivider(widths []int) string {
	segments := make([]string, 0, len(widths))
	for _, width := range widths {
		segments = append(segments, strings.Repeat("-", width+2))
	}
	return "+" + strings.Join(segments, "+") + "+"
}

func statusCell(status core.Status) string {
	label := strings.ToUpper(string(status))
	if status == core.StatusPass {
		return "✓ " + label
	}
	return label
}
