package report_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func orderingFinding(ruleID string, title string, line int, confidence string) codeguard.Finding {
	return codeguard.Finding{
		RuleID:     ruleID,
		Title:      title,
		Level:      "warn",
		Severity:   "warn",
		Section:    "Security",
		Message:    "finding at line " + itoa(line),
		Why:        "finding at line " + itoa(line),
		Path:       "app/main.go",
		Line:       line,
		Confidence: confidence,
	}
}

func itoa(value int) string {
	digits := ""
	if value == 0 {
		return "0"
	}
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}

func renderText(t testing.TB, report codeguard.Report) string {
	t.Helper()
	var buf bytes.Buffer
	if err := codeguard.WriteReport(&buf, report, "text"); err != nil {
		t.Fatalf("write text report: %v", err)
	}
	return buf.String()
}

func lineOrder(t testing.TB, rendered string, needles ...string) []int {
	t.Helper()
	positions := make([]int, 0, len(needles))
	for _, needle := range needles {
		idx := strings.Index(rendered, needle)
		if idx < 0 {
			t.Fatalf("rendered report missing %q:\n%s", needle, rendered)
		}
		positions = append(positions, idx)
	}
	return positions
}

func confidenceReport(findings []codeguard.Finding, filtered int) codeguard.Report {
	return codeguard.Report{
		Sections: []codeguard.SectionResult{{
			ID:                      "security",
			Name:                    "Security",
			Status:                  "warn",
			Findings:                findings,
			ConfidenceFilteredCount: filtered,
		}},
	}
}

// Within one rule group the most trustworthy findings are listed first, so a
// reader meets the high-confidence hits before the speculative ones.
func TestTextReportOrdersFindingsByConfidenceWithinGroup(t *testing.T) {
	rendered := renderText(t, confidenceReport([]codeguard.Finding{
		orderingFinding("security.demo", "Demo rule", 10, codeguard.ConfidenceLow),
		orderingFinding("security.demo", "Demo rule", 20, codeguard.ConfidenceHigh),
		orderingFinding("security.demo", "Demo rule", 30, codeguard.ConfidenceMedium),
	}, 0))

	positions := lineOrder(t, rendered, "line 20", "line 30", "line 10")
	if !(positions[0] < positions[1] && positions[1] < positions[2]) {
		t.Fatalf("findings not ordered high, medium, low:\n%s", rendered)
	}
}

// Equal confidence keeps scan order, so output stays reproducible.
func TestTextReportConfidenceSortIsStable(t *testing.T) {
	rendered := renderText(t, confidenceReport([]codeguard.Finding{
		orderingFinding("security.demo", "Demo rule", 30, codeguard.ConfidenceHigh),
		orderingFinding("security.demo", "Demo rule", 10, codeguard.ConfidenceHigh),
		orderingFinding("security.demo", "Demo rule", 20, codeguard.ConfidenceHigh),
	}, 0))

	positions := lineOrder(t, rendered, "line 30", "line 10", "line 20")
	if !(positions[0] < positions[1] && positions[1] < positions[2]) {
		t.Fatalf("equal-confidence findings were reordered:\n%s", rendered)
	}
}

// Groups keep their first-appearance order: confidence sorts inside a group,
// never across groups.
func TestTextReportKeepsGroupOrder(t *testing.T) {
	rendered := renderText(t, confidenceReport([]codeguard.Finding{
		orderingFinding("security.first", "First rule", 10, codeguard.ConfidenceLow),
		orderingFinding("security.second", "Second rule", 20, codeguard.ConfidenceHigh),
	}, 0))

	positions := lineOrder(t, rendered, "First rule", "Second rule")
	if positions[0] > positions[1] {
		t.Fatalf("group order changed:\n%s", rendered)
	}
}

func TestTextReportStatesConfidenceFilteredCount(t *testing.T) {
	rendered := renderText(t, confidenceReport([]codeguard.Finding{
		orderingFinding("security.demo", "Demo rule", 10, codeguard.ConfidenceHigh),
	}, 3))
	if !strings.Contains(rendered, "confidence filtered: 3") {
		t.Fatalf("rendered report does not state the confidence-filtered count:\n%s", rendered)
	}
}

func TestTextReportOmitsConfidenceLineWhenNothingFiltered(t *testing.T) {
	rendered := renderText(t, confidenceReport([]codeguard.Finding{
		orderingFinding("security.demo", "Demo rule", 10, codeguard.ConfidenceHigh),
	}, 0))
	if strings.Contains(rendered, "confidence filtered") {
		t.Fatalf("rendered report mentions confidence filtering with nothing filtered:\n%s", rendered)
	}
}

// JSON is a machine surface: section finding order must stay exactly as the
// scan produced it, so the confidence sort is presentation-only.
func TestJSONReportPreservesScanOrder(t *testing.T) {
	report := confidenceReport([]codeguard.Finding{
		orderingFinding("security.demo", "Demo rule", 10, codeguard.ConfidenceLow),
		orderingFinding("security.demo", "Demo rule", 20, codeguard.ConfidenceHigh),
	}, 0)
	var buf bytes.Buffer
	if err := codeguard.WriteReport(&buf, report, "json"); err != nil {
		t.Fatalf("write json report: %v", err)
	}
	rendered := buf.String()
	first := strings.Index(rendered, "finding at line 10")
	second := strings.Index(rendered, "finding at line 20")
	if first < 0 || second < 0 {
		t.Fatalf("json report missing findings:\n%s", rendered)
	}
	if first > second {
		t.Fatalf("json findings were reordered by confidence:\n%s", rendered)
	}
}
