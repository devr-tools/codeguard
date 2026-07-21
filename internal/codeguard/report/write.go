package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	rulespkg "github.com/devr-tools/codeguard/internal/codeguard/rules"
	"github.com/devr-tools/codeguard/internal/version"
)

func Write(w io.Writer, report core.Report, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		return writeText(w, report)
	case "json":
		return writeJSON(w, report)
	case "sarif":
		return writeSARIF(w, report)
	case "cyclonedx", "cyclonedx-json":
		return writeCycloneDX(w, report)
	case "github":
		return writeGitHubAnnotations(w, report)
	case "github-comment":
		return writeGitHubComment(w, report)
	default:
		return fmt.Errorf("unsupported report format %q", format)
	}
}

func writeJSON(w io.Writer, report core.Report) error {
	payload := struct {
		CodeGuardVersion string `json:"codeguard_version"`
		core.Report
	}{
		CodeGuardVersion: version.Number,
		Report:           report,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func writeText(w io.Writer, report core.Report) error {
	if err := writeTextHeader(w, report); err != nil {
		return err
	}
	if err := writeTextOverview(w, report); err != nil {
		return err
	}
	for _, section := range report.Sections {
		if err := writeTextSection(w, section); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "\nSummary: %d pass, %d warn, %d fail, %d findings, %d suppressed\n",
		report.Summary.PassedSections,
		report.Summary.WarnedSections,
		report.Summary.FailedSections,
		report.Summary.TotalFindings,
		report.Summary.SuppressedFindings,
	)
	return err
}

func writeGitHubAnnotations(w io.Writer, report core.Report) error {
	if _, err := fmt.Fprintf(w, "::notice title=CodeGuard::version %s\n", escapeGitHubAnnotation(version.Number)); err != nil {
		return err
	}
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			level := "warning"
			if finding.Level == "fail" {
				level = "error"
			}
			message := escapeGitHubAnnotation(fmt.Sprintf("[%s] %s. Fix: %s", finding.RuleID, firstNonEmpty(finding.Why, finding.Message), firstNonEmpty(finding.HowToFix, "see rule guidance")))
			if finding.Path != "" {
				if _, err := fmt.Fprintf(w, "::%s file=%s,line=%d,col=%d::%s\n", level, finding.Path, max(1, finding.Line), max(1, finding.Column), message); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(w, "::%s::%s\n", level, message); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeSARIF(w io.Writer, report core.Report) error {
	payload := map[string]any{
		"version": "2.1.0",
		"$schema": "https://json.schemastore.org/sarif-2.1.0.json",
	}
	rulesSeen := map[string]struct{}{}
	var sarifRules []sarifRule
	var results []sarifResult
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if _, ok := rulesSeen[finding.RuleID]; !ok {
				rulesSeen[finding.RuleID] = struct{}{}
				sarifRules = append(sarifRules, buildSARIFRule(rulespkg.Catalog(), finding))
			}
			results = append(results, buildSARIFResult(finding))
		}
	}
	sort.Slice(sarifRules, func(i, j int) bool { return sarifRules[i].ID < sarifRules[j].ID })

	// invocation records that codeguard ran, so a consumer can attribute the
	// SARIF file to a specific run (SOC 3 monitoring / audit trail). The analysis
	// completing successfully is independent of whether findings were reported.
	invocation := map[string]any{"executionSuccessful": true}
	if report.GeneratedAt != "" {
		invocation["endTimeUtc"] = report.GeneratedAt
	}
	run := map[string]any{
		"tool": map[string]any{
			"driver": map[string]any{
				"name":            "codeguard",
				"version":         version.Number,
				"semanticVersion": strings.TrimPrefix(version.Number, "v"),
				"informationUri":  "https://github.com/devr-tools/codeguard",
				"rules":           sarifRules,
			},
		},
		"taxonomies":  []any{owaspTaxonomy()},
		"invocations": []any{invocation},
		"results":     results,
	}
	if report.Profile != "" {
		run["properties"] = map[string]any{"profile": report.Profile}
	}
	payload["runs"] = []any{run}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func escapeGitHubAnnotation(value string) string {
	replacer := strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	return replacer.Replace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
