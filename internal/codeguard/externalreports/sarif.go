// Package externalreports safely imports findings from reports that were
// produced before CodeGuard ran. It intentionally never starts a scanner.
package externalreports

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const maxReportBytes int64 = 16 << 20 // 16 MiB: report files are untrusted CI input.

var unsafePath = regexp.MustCompile(`(^|[\\/])\.\.([\\/]|$)`)
var unsafeNamespace = regexp.MustCompile(`[^a-z0-9._-]+`)
var windowsAbsolutePath = regexp.MustCompile(`^[a-zA-Z]:[\\/]`)

// Import reads configured external reports and returns one normal report
// section per input. It accepts SARIF 2.x, Gitleaks JSON, and Trivy JSON.
// It only reads reports produced before CodeGuard ran; it never starts a
// scanner or retains the secret-bearing fields present in secret reports.
func Import(cfg []core.ExternalReportConfig) ([]core.SectionResult, error) {
	sections := make([]core.SectionResult, 0, len(cfg))
	for _, report := range cfg {
		format := strings.ToLower(strings.TrimSpace(report.Format))
		if format != "sarif" && format != "gitleaks" && format != "trivy" {
			return nil, fmt.Errorf("external report %q: unsupported format %q", report.Path, report.Format)
		}
		data, err := readSafe(report.Path)
		if err != nil {
			return nil, fmt.Errorf("external report %q: %w", report.Path, err)
		}
		var section core.SectionResult
		switch format {
		case "sarif":
			section, err = importSARIF(data, report)
		case "gitleaks":
			section, err = importGitleaks(data, report)
		case "trivy":
			section, err = importTrivy(data, report)
		}
		if err != nil {
			return nil, fmt.Errorf("external report %q: %w", report.Path, err)
		}
		sections = append(sections, section)
	}
	return sections, nil
}

func readSafe(path string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("path is required")
	}
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) && (clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator))) {
		return nil, fmt.Errorf("path escapes the repository")
	}
	info, err := os.Lstat(clean)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("must be a regular, non-symlink file")
	}
	if info.Size() > maxReportBytes {
		return nil, fmt.Errorf("exceeds %d byte size limit", maxReportBytes)
	}
	f, err := os.Open(clean) //nolint:gosec // config path is validated and bounded above
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxReportBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxReportBytes {
		return nil, fmt.Errorf("exceeds %d byte size limit", maxReportBytes)
	}
	return data, nil
}

type sarifLog struct {
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}
type sarifRun struct {
	Tool struct {
		Driver struct {
			Name  string                `json:"name"`
			Rules []sarifRuleDefinition `json:"rules"`
		} `json:"driver"`
	} `json:"tool"`
	Results []sarifResult `json:"results"`
}
type sarifRuleDefinition struct {
	ID               string       `json:"id"`
	ShortDescription sarifMessage `json:"shortDescription"`
	Help             sarifMessage `json:"help"`
}
type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}
type sarifMessage struct {
	Text     string `json:"text"`
	Markdown string `json:"markdown"`
}
type sarifLocation struct {
	PhysicalLocation struct {
		ArtifactLocation struct {
			URI string `json:"uri"`
		} `json:"artifactLocation"`
		Region struct {
			StartLine   int `json:"startLine"`
			StartColumn int `json:"startColumn"`
		} `json:"region"`
	} `json:"physicalLocation"`
}

func importSARIF(data []byte, source core.ExternalReportConfig) (core.SectionResult, error) {
	var log sarifLog
	if err := json.Unmarshal(data, &log); err != nil {
		return core.SectionResult{}, fmt.Errorf("invalid SARIF JSON: %w", err)
	}
	if len(log.Runs) == 0 {
		return core.SectionResult{ID: "external", Name: "External Reports", Status: core.StatusPass}, nil
	}
	findings := make([]core.Finding, 0)
	for _, run := range log.Runs {
		rules := make(map[string]sarifRuleDefinition, len(run.Tool.Driver.Rules))
		for _, rule := range run.Tool.Driver.Rules {
			rules[rule.ID] = rule
		}
		tool := namespace(firstNonEmpty(source.Source, run.Tool.Driver.Name, "sarif"))
		if tool == "" {
			tool = "sarif"
		}
		for _, result := range run.Results {
			ruleID := strings.TrimSpace(result.RuleID)
			if ruleID == "" {
				ruleID = "unknown"
			}
			path, line, column := resultLocation(result)
			rule := rules[result.RuleID]
			message := truncate(firstNonEmpty(result.Message.Text, result.Message.Markdown, rule.ShortDescription.Text, "External scanner finding"), 4096)
			level := sarifLevel(result.Level)
			namespacedRuleID := namespace(ruleID)
			if namespacedRuleID == "" {
				namespacedRuleID = "unknown"
			}
			finding := core.Finding{
				RuleID: "external." + tool + "." + namespacedRuleID, Level: level, Severity: level,
				Title: truncate(firstNonEmpty(rule.ShortDescription.Text, ruleID), 512), Section: "External Reports",
				Message: message, Why: message, HowToFix: truncate(firstNonEmpty(rule.Help.Text, rule.Help.Markdown), 2048),
				Path: path, Line: line, Column: column,
				Metadata: map[string]string{"external_source": tool, "external_rule_id": ruleID, "external_format": "sarif"},
			}
			finding.Fingerprint = fingerprint(finding)
			finding.ContextFingerprint = finding.Fingerprint
			findings = append(findings, finding)
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Fingerprint < findings[j].Fingerprint })
	section := core.SectionResult{ID: "external", Name: "External Reports", Status: core.StatusPass, Findings: findings}
	for _, f := range findings {
		if f.Level == "fail" {
			section.Status = core.StatusFail
		} else if f.Level == "warn" && section.Status != core.StatusFail {
			section.Status = core.StatusWarn
		}
	}
	return section, nil
}

func resultLocation(result sarifResult) (string, int, int) {
	if len(result.Locations) == 0 {
		return "", 0, 0
	}
	location := result.Locations[0].PhysicalLocation
	uri := strings.TrimSpace(location.ArtifactLocation.URI)
	if uri == "" || filepath.IsAbs(filepath.FromSlash(uri)) || windowsAbsolutePath.MatchString(uri) || unsafePath.MatchString(uri) || strings.Contains(uri, "://") {
		return "", 0, 0
	}
	normalized := strings.ReplaceAll(uri, "\\", "/")
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(normalized))), location.Region.StartLine, location.Region.StartColumn
}

func namespace(value string) string {
	value = unsafeNamespace.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "-")
	return strings.Trim(value, "-.")
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
func truncate(value string, max int) string {
	if len(value) > max {
		return value[:max]
	}
	return value
}
func sarifLevel(level string) string {
	if strings.EqualFold(strings.TrimSpace(level), "error") {
		return "fail"
	}
	return "warn"
}
func fingerprint(f core.Finding) string {
	return fmt.Sprintf("external:%s:%s:%d:%s", f.RuleID, f.Path, f.Line, f.Message)
}
