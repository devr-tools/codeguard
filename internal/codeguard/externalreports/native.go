package externalreports

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// gitleaksFinding deliberately omits Match and Secret. Those fields can hold a
// credential and must never enter CodeGuard's in-memory finding model.
type gitleaksFinding struct {
	RuleID      string `json:"RuleID"`
	File        string `json:"File"`
	StartLine   int    `json:"StartLine"`
	StartColumn int    `json:"StartColumn"`
}

func importGitleaks(data []byte, source core.ExternalReportConfig) (core.SectionResult, error) {
	var results []gitleaksFinding
	if err := json.Unmarshal(data, &results); err != nil {
		return core.SectionResult{}, fmt.Errorf("invalid Gitleaks JSON: %w", err)
	}
	tool := externalTool(source, "gitleaks")
	findings := make([]core.Finding, 0, len(results))
	for _, result := range results {
		ruleID := firstNonEmpty(strings.TrimSpace(result.RuleID), "unknown")
		finding := core.Finding{
			RuleID:   "external." + tool + "." + normalizedRuleID(ruleID),
			Level:    "fail",
			Severity: "fail",
			Title:    "Gitleaks secret finding: " + truncate(ruleID, 512),
			// Do not use report descriptions here. Besides being untrusted input,
			// custom scanner rules can include a matched value in their text.
			Message:  "Gitleaks reported a potential secret. Rotate or remove it and use a secure secret store.",
			Why:      "A credential-like value was detected in the repository.",
			HowToFix: "Remove the secret, rotate the affected credential, and load it from a secure secret store.",
			Section:  "External Reports",
			Path:     safeReportPath(result.File),
			Line:     positive(result.StartLine),
			Column:   positive(result.StartColumn),
			Metadata: map[string]string{
				"external_source": tool, "external_rule_id": truncate(ruleID, 512), "external_format": "gitleaks",
			},
		}
		finding.Fingerprint = fingerprint(finding)
		finding.ContextFingerprint = finding.Fingerprint
		findings = append(findings, finding)
	}
	return newExternalSection(findings), nil
}

// trivyReport models only safe identity, location, and severity fields. In
// particular, Trivy's Secrets.Match and Secrets.Code are intentionally absent.
type trivyReport struct {
	Results []trivyResult `json:"Results"`
}
type trivyResult struct {
	Target            string                  `json:"Target"`
	Vulnerabilities   []trivyVulnerability    `json:"Vulnerabilities"`
	Misconfigurations []trivyMisconfiguration `json:"Misconfigurations"`
	Secrets           []trivySecret           `json:"Secrets"`
}
type trivyVulnerability struct {
	VulnerabilityID  string `json:"VulnerabilityID"`
	PkgName          string `json:"PkgName"`
	InstalledVersion string `json:"InstalledVersion"`
	FixedVersion     string `json:"FixedVersion"`
	Severity         string `json:"Severity"`
}
type trivyMisconfiguration struct {
	ID       string `json:"ID"`
	Severity string `json:"Severity"`
	Location struct {
		Filename  string `json:"Filename"`
		StartLine int    `json:"StartLine"`
	} `json:"Location"`
	CauseMetadata struct {
		Resource  string `json:"Resource"`
		StartLine int    `json:"StartLine"`
	} `json:"CauseMetadata"`
}
type trivySecret struct {
	RuleID    string `json:"RuleID"`
	Category  string `json:"Category"`
	Severity  string `json:"Severity"`
	StartLine int    `json:"StartLine"`
}

func importTrivy(data []byte, source core.ExternalReportConfig) (core.SectionResult, error) {
	var report trivyReport
	if err := json.Unmarshal(data, &report); err != nil {
		return core.SectionResult{}, fmt.Errorf("invalid Trivy JSON: %w", err)
	}
	tool := externalTool(source, "trivy")
	findings := make([]core.Finding, 0)
	for _, result := range report.Results {
		for _, vulnerability := range result.Vulnerabilities {
			id := firstNonEmpty(strings.TrimSpace(vulnerability.VulnerabilityID), "unknown")
			pkg := truncate(strings.TrimSpace(vulnerability.PkgName), 512)
			finding := newTrivyFinding(tool, "vulnerability."+id, trivyLevel(vulnerability.Severity), result.Target, 0,
				"Trivy vulnerability: "+truncate(id, 512),
				"Trivy reported "+severityLabel(vulnerability.Severity)+" vulnerability "+truncate(id, 512)+packageSuffix(pkg),
				map[string]string{"trivy_kind": "vulnerability", "trivy_package": pkg, "trivy_installed_version": truncate(vulnerability.InstalledVersion, 256), "trivy_fixed_version": truncate(vulnerability.FixedVersion, 256)})
			findings = append(findings, finding)
		}
		for _, misconfiguration := range result.Misconfigurations {
			id := firstNonEmpty(strings.TrimSpace(misconfiguration.ID), "unknown")
			path := firstNonEmpty(misconfiguration.Location.Filename, result.Target, misconfiguration.CauseMetadata.Resource)
			line := firstPositive(misconfiguration.Location.StartLine, misconfiguration.CauseMetadata.StartLine)
			finding := newTrivyFinding(tool, "misconfiguration."+id, trivyLevel(misconfiguration.Severity), path, line,
				"Trivy misconfiguration: "+truncate(id, 512),
				"Trivy reported "+severityLabel(misconfiguration.Severity)+" misconfiguration "+truncate(id, 512)+".",
				map[string]string{"trivy_kind": "misconfiguration"})
			findings = append(findings, finding)
		}
		for _, secret := range result.Secrets {
			id := firstNonEmpty(strings.TrimSpace(secret.RuleID), strings.TrimSpace(secret.Category), "unknown")
			finding := newTrivyFinding(tool, "secret."+id, "fail", result.Target, secret.StartLine,
				"Trivy secret finding: "+truncate(id, 512),
				"Trivy reported a potential secret. Rotate or remove it and use a secure secret store.",
				map[string]string{"trivy_kind": "secret"})
			findings = append(findings, finding)
		}
	}
	return newExternalSection(findings), nil
}

func newTrivyFinding(tool, rule, level, path string, line int, title, message string, metadata map[string]string) core.Finding {
	ruleID := normalizedRuleID(rule)
	metadata["external_source"] = tool
	metadata["external_rule_id"] = truncate(rule, 512)
	metadata["external_format"] = "trivy"
	finding := core.Finding{RuleID: "external." + tool + "." + ruleID, Level: level, Severity: level, Title: title, Message: message, Why: message, Section: "External Reports", Path: safeReportPath(path), Line: positive(line), Metadata: metadata}
	finding.Fingerprint = fingerprint(finding)
	finding.ContextFingerprint = finding.Fingerprint
	return finding
}

func newExternalSection(findings []core.Finding) core.SectionResult {
	sort.Slice(findings, func(i, j int) bool { return findings[i].Fingerprint < findings[j].Fingerprint })
	section := core.SectionResult{ID: "external", Name: "External Reports", Status: core.StatusPass, Findings: findings}
	for _, finding := range findings {
		if finding.Level == "fail" {
			section.Status = core.StatusFail
		} else if finding.Level == "warn" && section.Status != core.StatusFail {
			section.Status = core.StatusWarn
		}
	}
	return section
}

func externalTool(source core.ExternalReportConfig, fallback string) string {
	tool := namespace(firstNonEmpty(source.Source, fallback))
	if tool == "" {
		return fallback
	}
	return tool
}
func normalizedRuleID(value string) string {
	if result := namespace(value); result != "" {
		return result
	}
	return "unknown"
}
func safeReportPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || filepath.IsAbs(filepath.FromSlash(value)) || windowsAbsolutePath.MatchString(value) || unsafePath.MatchString(value) || strings.Contains(value, "://") {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.ReplaceAll(value, "\\\\", "/"))))
}
func positive(value int) int {
	if value > 0 {
		return value
	}
	return 0
}
func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
func trivyLevel(severity string) string {
	if strings.EqualFold(strings.TrimSpace(severity), "critical") || strings.EqualFold(strings.TrimSpace(severity), "high") {
		return "fail"
	}
	return "warn"
}
func severityLabel(severity string) string {
	if value := strings.ToLower(strings.TrimSpace(severity)); value != "" {
		return value
	}
	return "unknown"
}
func packageSuffix(pkg string) string {
	if pkg == "" {
		return ""
	}
	return " in package " + pkg
}
