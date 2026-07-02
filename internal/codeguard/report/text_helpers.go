package report

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func writeTextHeader(w io.Writer, report core.Report) error {
	if _, err := io.WriteString(w, brandBanner()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s\n", report.Name); err != nil {
		return err
	}
	if report.Profile == "" {
		return nil
	}
	_, err := fmt.Fprintf(w, "profile: %s\n", report.Profile)
	return err
}

func writeTextOverview(w io.Writer, report core.Report) error {
	headers := []string{"Section", "Status", "Findings", "Suppressed"}
	rows := make([][]string, 0, len(report.Sections))
	for _, section := range report.Sections {
		rows = append(rows, []string{
			section.Name,
			statusLabel(string(section.Status)),
			strconv.Itoa(len(section.Findings)),
			strconv.Itoa(section.SuppressedCount),
		})
	}

	widths := columnWidths(headers, rows)
	if _, err := fmt.Fprintf(w, "\n%s\n", tableBorder(widths)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s\n", tableRow(headers, widths, false)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s\n", tableBorder(widths)); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(w, "%s\n", tableRow(row, widths, true)); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%s\n", tableBorder(widths))
	return err
}

func writeTextSection(w io.Writer, section core.SectionResult) error {
	if _, err := fmt.Fprintf(w, "\n[%s] %s\n", renderStatus(string(section.Status), true), section.Name); err != nil {
		return err
	}
	if len(section.Findings) == 0 {
		if _, err := io.WriteString(w, "  ✅ no findings\n"); err != nil {
			return err
		}
		return nil
	}
	for _, group := range groupTextFindings(section.Findings) {
		if _, err := fmt.Fprintf(w, "\n  %s\n", group.name); err != nil {
			return err
		}
		for idx, finding := range group.findings {
			if err := writeTextFinding(w, idx+1, finding); err != nil {
				return err
			}
		}
	}
	if section.SuppressedCount > 0 {
		if _, err := fmt.Fprintf(w, "\n  suppressed: %d\n", section.SuppressedCount); err != nil {
			return err
		}
	}
	return nil
}

type textFindingGroup struct {
	name     string
	findings []core.Finding
}

func groupTextFindings(findings []core.Finding) []textFindingGroup {
	order := make([]string, 0)
	groups := make(map[string][]core.Finding)
	for _, finding := range findings {
		name := firstNonEmpty(finding.Title, finding.RuleID)
		if _, ok := groups[name]; !ok {
			order = append(order, name)
		}
		groups[name] = append(groups[name], finding)
	}
	out := make([]textFindingGroup, 0, len(order))
	for _, name := range order {
		out = append(out, textFindingGroup{
			name:     name,
			findings: groups[name],
		})
	}
	return out
}

func writeTextFinding(w io.Writer, index int, finding core.Finding) error {
	if _, err := fmt.Fprintf(w, "  %d. at: %s\n", index, findingLocation(finding)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "     rule: %s\n", finding.RuleID); err != nil {
		return err
	}
	why := firstNonEmpty(finding.Why, finding.Message)
	if finding.Confidence == core.ConfidenceLow {
		why += " (low confidence)"
	}
	if _, err := fmt.Fprintf(w, "     why: %s\n", why); err != nil {
		return err
	}
	if finding.HowToFix != "" {
		if _, err := fmt.Fprintf(w, "     fix: %s\n", finding.HowToFix); err != nil {
			return err
		}
	}
	return nil
}

func findingLocation(finding core.Finding) string {
	if finding.Line > 0 {
		return fmt.Sprintf("%s:%d", finding.Path, finding.Line)
	}
	if finding.Path == "" {
		return "(repository)"
	}
	return finding.Path
}

func columnWidths(headers []string, rows [][]string) []int {
	widths := make([]int, len(headers))
	for idx, header := range headers {
		widths[idx] = len(header)
	}
	for _, row := range rows {
		for idx, cell := range row {
			if len(cell) > widths[idx] {
				widths[idx] = len(cell)
			}
		}
	}
	return widths
}

func tableBorder(widths []int) string {
	parts := make([]string, 0, len(widths))
	for _, width := range widths {
		parts = append(parts, strings.Repeat("-", width+2))
	}
	return "+" + strings.Join(parts, "+") + "+"
}

func tableRow(values []string, widths []int, colorStatus bool) string {
	cells := make([]string, 0, len(values))
	for idx, value := range values {
		display := value
		if colorStatus && idx == 1 {
			display = renderStatus(value, false)
		}
		cells = append(cells, fmt.Sprintf(" %-*s ", widths[idx], display))
	}
	return "|" + strings.Join(cells, "|") + "|"
}
