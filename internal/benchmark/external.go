package benchmark

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const ExternalSchemaVersion = 1

var supportedExternalTools = map[string]bool{
	"gitleaks":   true,
	"semgrep":    true,
	"trivy":      true,
	"trufflehog": true,
}

type ExternalReportInput struct {
	Tool string `json:"tool"`
	Path string `json:"path"`
}

type ExternalReport struct {
	Version  int               `json:"version"`
	Reports  []ExternalSource  `json:"reports"`
	Summary  ExternalSummary   `json:"summary"`
	Findings []ExternalFinding `json:"findings"`
}

type ExternalSource struct {
	Tool     string `json:"tool"`
	Path     string `json:"path"`
	Format   string `json:"format"`
	Findings int    `json:"findings"`
}

type ExternalFinding struct {
	Tool     string `json:"tool"`
	RuleID   string `json:"rule_id"`
	Category string `json:"category"`
	Severity string `json:"severity,omitempty"`
	Path     string `json:"path"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	Message  string `json:"message,omitempty"`
	Source   string `json:"source,omitempty"`
}

type ExternalSummary struct {
	Total      int              `json:"total"`
	ByTool     []ExternalBucket `json:"by_tool"`
	ByCategory []ExternalBucket `json:"by_category"`
	ByPathLine []ExternalBucket `json:"by_path_line"`
}

type ExternalBucket struct {
	Tool     string `json:"tool,omitempty"`
	Category string `json:"category,omitempty"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	Count    int    `json:"count"`
}

func ImportExternalReports(inputs []ExternalReportInput) (ExternalReport, error) {
	if len(inputs) == 0 {
		return ExternalReport{}, fmt.Errorf("at least one external report is required")
	}
	report := ExternalReport{Version: ExternalSchemaVersion}
	for _, input := range inputs {
		tool, err := normalizeExternalTool(input.Tool)
		if err != nil {
			return ExternalReport{}, err
		}
		if strings.TrimSpace(input.Path) == "" {
			return ExternalReport{}, fmt.Errorf("external report path is required for %s", tool)
		}
		findings, format, err := parseExternalReport(tool, input.Path)
		if err != nil {
			return ExternalReport{}, err
		}
		report.Reports = append(report.Reports, ExternalSource{Tool: tool, Path: input.Path, Format: format, Findings: len(findings)})
		report.Findings = append(report.Findings, findings...)
	}
	sortExternalFindings(report.Findings)
	report.Summary = summarizeExternalFindings(report.Findings)
	return report, nil
}

func RenderExternalMarkdown(report ExternalReport) string {
	var b strings.Builder
	b.WriteString("# External Benchmark Summary\n\n")
	b.WriteString("This report normalizes saved external scanner reports. It compares raw finding counts only; it does not score true positives, false positives, or false negatives.\n\n")
	b.WriteString("## Reports\n\n")
	b.WriteString("| Tool | Format | Findings | Path |\n")
	b.WriteString("| --- | --- | ---: | --- |\n")
	for _, source := range report.Reports {
		fmt.Fprintf(&b, "| %s | %s | %d | `%s` |\n", markdownCell(source.Tool), markdownCell(source.Format), source.Findings, markdownCell(source.Path))
	}
	b.WriteString("\n## Counts By Tool\n\n")
	b.WriteString("| Tool | Count |\n")
	b.WriteString("| --- | ---: |\n")
	for _, bucket := range report.Summary.ByTool {
		fmt.Fprintf(&b, "| %s | %d |\n", markdownCell(bucket.Tool), bucket.Count)
	}
	b.WriteString("\n## Counts By Category\n\n")
	b.WriteString("| Tool | Category | Count |\n")
	b.WriteString("| --- | --- | ---: |\n")
	for _, bucket := range report.Summary.ByCategory {
		fmt.Fprintf(&b, "| %s | %s | %d |\n", markdownCell(bucket.Tool), markdownCell(bucket.Category), bucket.Count)
	}
	b.WriteString("\n## Counts By Path And Line\n\n")
	b.WriteString("| Tool | Category | Path | Line | Count |\n")
	b.WriteString("| --- | --- | --- | ---: | ---: |\n")
	for _, bucket := range report.Summary.ByPathLine {
		line := ""
		if bucket.Line > 0 {
			line = strconv.Itoa(bucket.Line)
		}
		fmt.Fprintf(&b, "| %s | %s | `%s` | %s | %d |\n", markdownCell(bucket.Tool), markdownCell(bucket.Category), markdownCell(bucket.Path), line, bucket.Count)
	}
	return b.String()
}

func markdownCell(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "|", `\|`)
	value = strings.ReplaceAll(value, "`", "'")
	return value
}

func WriteMarkdown(path, text string) error {
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return fmt.Errorf("write benchmark Markdown: %w", err)
	}
	return nil
}

func normalizeExternalTool(tool string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(tool))
	if !supportedExternalTools[normalized] {
		return "", fmt.Errorf("unsupported external tool %q", tool)
	}
	return normalized, nil
}

func parseExternalReport(tool, path string) ([]ExternalFinding, string, error) {
	// #nosec G304 -- benchmark operators explicitly choose saved tool reports.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read %s report %q: %w", tool, path, err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, "", fmt.Errorf("parse %s report %q: empty report", tool, path)
	}
	if isSARIF(trimmed) {
		findings, err := parseSARIFReport(tool, trimmed)
		return findings, "sarif", err
	}
	switch tool {
	case "gitleaks":
		findings, err := parseGitleaksJSON(trimmed)
		return findings, "json", err
	case "semgrep":
		findings, err := parseSemgrepJSON(trimmed)
		return findings, "json", err
	case "trivy":
		findings, err := parseTrivyJSON(trimmed)
		return findings, "json", err
	case "trufflehog":
		findings, err := parseTruffleHogJSON(trimmed)
		return findings, "json", err
	default:
		return nil, "", fmt.Errorf("unsupported external tool %q", tool)
	}
}

func isSARIF(data []byte) bool {
	var header struct {
		Version string          `json:"version"`
		Runs    json.RawMessage `json:"runs"`
		Schema  string          `json:"$schema"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return false
	}
	return header.Runs != nil && (strings.Contains(header.Schema, "sarif") || strings.HasPrefix(header.Version, "2."))
}

type sarifLog struct {
	Runs []struct {
		Tool struct {
			Driver sarifDriver `json:"driver"`
		} `json:"tool"`
		Results []sarifResult `json:"results"`
	} `json:"runs"`
}

type sarifDriver struct {
	Rules []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	ShortDescription sarifMessage   `json:"shortDescription"`
	FullDescription  sarifMessage   `json:"fullDescription"`
	Properties       map[string]any `json:"properties"`
}

type sarifResult struct {
	RuleID     string          `json:"ruleId"`
	RuleIndex  *int            `json:"ruleIndex"`
	Level      string          `json:"level"`
	Message    sarifMessage    `json:"message"`
	Locations  []sarifLocation `json:"locations"`
	Properties map[string]any  `json:"properties"`
}

type sarifMessage struct {
	Text string `json:"text"`
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

func parseSARIFReport(tool string, data []byte) ([]ExternalFinding, error) {
	var log sarifLog
	if err := json.Unmarshal(data, &log); err != nil {
		return nil, fmt.Errorf("parse %s SARIF: %w", tool, err)
	}
	var findings []ExternalFinding
	for _, run := range log.Runs {
		rules := map[string]sarifRule{}
		for _, rule := range run.Tool.Driver.Rules {
			rules[rule.ID] = rule
		}
		for _, result := range run.Results {
			ruleID := result.RuleID
			if ruleID == "" && result.RuleIndex != nil && *result.RuleIndex >= 0 && *result.RuleIndex < len(run.Tool.Driver.Rules) {
				ruleID = run.Tool.Driver.Rules[*result.RuleIndex].ID
			}
			rule := rules[ruleID]
			message := firstNonEmpty(result.Message.Text, rule.ShortDescription.Text, rule.FullDescription.Text, rule.Name)
			location := firstSARIFLocation(result.Locations)
			rawCategory := firstStringProperty(result.Properties, "category", "kind")
			if rawCategory == "" {
				rawCategory = firstStringProperty(rule.Properties, "category", "kind", "precision", "problem.severity")
			}
			if rawCategory == "" {
				rawCategory = strings.Join(stringSliceProperty(rule.Properties, "tags"), " ")
			}
			findings = append(findings, ExternalFinding{
				Tool:     tool,
				RuleID:   ruleID,
				Category: normalizeFindingCategory(tool, rawCategory, ruleID, message),
				Severity: normalizeSeverity(firstNonEmpty(result.Level, firstStringProperty(rule.Properties, "security-severity", "problem.severity"))),
				Path:     cleanExternalPath(location.path),
				Line:     location.line,
				Column:   location.column,
				Message:  message,
				Source:   "sarif",
			})
		}
	}
	return findings, nil
}

type sarifLocationValue struct {
	path   string
	line   int
	column int
}

func firstSARIFLocation(locations []sarifLocation) sarifLocationValue {
	if len(locations) == 0 {
		return sarifLocationValue{}
	}
	physical := locations[0].PhysicalLocation
	return sarifLocationValue{
		path:   physical.ArtifactLocation.URI,
		line:   physical.Region.StartLine,
		column: physical.Region.StartColumn,
	}
}

func parseSemgrepJSON(data []byte) ([]ExternalFinding, error) {
	var report struct {
		Results []struct {
			CheckID string `json:"check_id"`
			Path    string `json:"path"`
			Start   struct {
				Line int `json:"line"`
				Col  int `json:"col"`
			} `json:"start"`
			Extra struct {
				Message  string         `json:"message"`
				Severity string         `json:"severity"`
				Metadata map[string]any `json:"metadata"`
			} `json:"extra"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("parse semgrep JSON: %w", err)
	}
	findings := make([]ExternalFinding, 0, len(report.Results))
	for _, result := range report.Results {
		rawCategory := firstStringProperty(result.Extra.Metadata, "category", "owasp", "cwe", "confidence")
		findings = append(findings, ExternalFinding{
			Tool:     "semgrep",
			RuleID:   result.CheckID,
			Category: normalizeFindingCategory("semgrep", rawCategory, result.CheckID, result.Extra.Message),
			Severity: normalizeSeverity(result.Extra.Severity),
			Path:     cleanExternalPath(result.Path),
			Line:     result.Start.Line,
			Column:   result.Start.Col,
			Message:  result.Extra.Message,
			Source:   "json",
		})
	}
	return findings, nil
}

func parseGitleaksJSON(data []byte) ([]ExternalFinding, error) {
	var leaks []struct {
		RuleID      string   `json:"RuleID"`
		Description string   `json:"Description"`
		File        string   `json:"File"`
		StartLine   int      `json:"StartLine"`
		StartColumn int      `json:"StartColumn"`
		Tags        []string `json:"Tags"`
	}
	if err := json.Unmarshal(data, &leaks); err != nil {
		return nil, fmt.Errorf("parse gitleaks JSON: %w", err)
	}
	findings := make([]ExternalFinding, 0, len(leaks))
	for _, leak := range leaks {
		rawCategory := strings.Join(leak.Tags, " ")
		findings = append(findings, ExternalFinding{
			Tool:     "gitleaks",
			RuleID:   leak.RuleID,
			Category: normalizeFindingCategory("gitleaks", rawCategory, leak.RuleID, leak.Description),
			Severity: "warning",
			Path:     cleanExternalPath(leak.File),
			Line:     leak.StartLine,
			Column:   leak.StartColumn,
			Message:  leak.Description,
			Source:   "json",
		})
	}
	return findings, nil
}

func parseTrivyJSON(data []byte) ([]ExternalFinding, error) {
	var report struct {
		Results []struct {
			Target  string `json:"Target"`
			Class   string `json:"Class"`
			Type    string `json:"Type"`
			Secrets []struct {
				RuleID    string `json:"RuleID"`
				Category  string `json:"Category"`
				Severity  string `json:"Severity"`
				Title     string `json:"Title"`
				StartLine int    `json:"StartLine"`
			} `json:"Secrets"`
			Misconfigurations []struct {
				ID       string `json:"ID"`
				Type     string `json:"Type"`
				Severity string `json:"Severity"`
				Title    string `json:"Title"`
				Message  string `json:"Message"`
			} `json:"Misconfigurations"`
			Vulnerabilities []struct {
				VulnerabilityID string `json:"VulnerabilityID"`
				PkgName         string `json:"PkgName"`
				Severity        string `json:"Severity"`
				Title           string `json:"Title"`
			} `json:"Vulnerabilities"`
		} `json:"Results"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("parse trivy JSON: %w", err)
	}
	var findings []ExternalFinding
	for _, result := range report.Results {
		path := cleanExternalPath(result.Target)
		for _, secret := range result.Secrets {
			message := firstNonEmpty(secret.Title, secret.Category)
			findings = append(findings, ExternalFinding{
				Tool:     "trivy",
				RuleID:   secret.RuleID,
				Category: normalizeFindingCategory("trivy", "secret "+secret.Category, secret.RuleID, message),
				Severity: normalizeSeverity(secret.Severity),
				Path:     path,
				Line:     secret.StartLine,
				Message:  message,
				Source:   "json",
			})
		}
		for _, misconfiguration := range result.Misconfigurations {
			message := firstNonEmpty(misconfiguration.Title, misconfiguration.Message)
			findings = append(findings, ExternalFinding{
				Tool:     "trivy",
				RuleID:   misconfiguration.ID,
				Category: normalizeFindingCategory("trivy", firstNonEmpty(misconfiguration.Type, result.Class, result.Type), misconfiguration.ID, message),
				Severity: normalizeSeverity(misconfiguration.Severity),
				Path:     path,
				Message:  message,
				Source:   "json",
			})
		}
		for _, vulnerability := range result.Vulnerabilities {
			message := firstNonEmpty(vulnerability.Title, vulnerability.PkgName)
			findings = append(findings, ExternalFinding{
				Tool:     "trivy",
				RuleID:   vulnerability.VulnerabilityID,
				Category: normalizeFindingCategory("trivy", "vulnerability", vulnerability.VulnerabilityID, message),
				Severity: normalizeSeverity(vulnerability.Severity),
				Path:     path,
				Message:  message,
				Source:   "json",
			})
		}
	}
	return findings, nil
}

func parseTruffleHogJSON(data []byte) ([]ExternalFinding, error) {
	decoder := json.NewDecoder(bufio.NewReader(bytes.NewReader(data)))
	var findings []ExternalFinding
	for {
		var item map[string]any
		if err := decoder.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("parse trufflehog JSON: %w", err)
		}
		ruleID := firstMapString(item, "DetectorName", "DetectorType", "detectorName", "detectorType")
		message := firstMapString(item, "Redacted", "Raw", "redacted", "raw")
		path := firstNestedString(item, []string{"SourceMetadata", "Data", "Filesystem", "file"}, []string{"SourceMetadata", "Data", "Git", "file"})
		line := firstNestedInt(item, []string{"SourceMetadata", "Data", "Filesystem", "line"}, []string{"SourceMetadata", "Data", "Git", "line"})
		findings = append(findings, ExternalFinding{
			Tool:     "trufflehog",
			RuleID:   ruleID,
			Category: normalizeFindingCategory("trufflehog", "secret", ruleID, message),
			Severity: "warning",
			Path:     cleanExternalPath(path),
			Line:     line,
			Message:  message,
			Source:   "json",
		})
	}
	return findings, nil
}

func summarizeExternalFindings(findings []ExternalFinding) ExternalSummary {
	summary := ExternalSummary{Total: len(findings)}
	toolCounts := map[string]int{}
	categoryCounts := map[string]int{}
	pathLineCounts := map[string]int{}
	for _, finding := range findings {
		toolCounts[finding.Tool]++
		categoryCounts[strings.Join([]string{finding.Tool, finding.Category}, "\x00")]++
		pathLineCounts[strings.Join([]string{finding.Tool, finding.Category, finding.Path, strconv.Itoa(finding.Line)}, "\x00")]++
	}
	for key, count := range toolCounts {
		summary.ByTool = append(summary.ByTool, ExternalBucket{Tool: key, Count: count})
	}
	for key, count := range categoryCounts {
		parts := strings.Split(key, "\x00")
		summary.ByCategory = append(summary.ByCategory, ExternalBucket{Tool: parts[0], Category: parts[1], Count: count})
	}
	for key, count := range pathLineCounts {
		parts := strings.Split(key, "\x00")
		line, _ := strconv.Atoi(parts[3])
		summary.ByPathLine = append(summary.ByPathLine, ExternalBucket{Tool: parts[0], Category: parts[1], Path: parts[2], Line: line, Count: count})
	}
	sortBuckets(summary.ByTool)
	sortBuckets(summary.ByCategory)
	sortBuckets(summary.ByPathLine)
	return summary
}

func sortExternalFindings(findings []ExternalFinding) {
	sort.Slice(findings, func(i, j int) bool {
		left, right := findings[i], findings[j]
		return strings.Join([]string{left.Tool, left.Category, left.Path, fmt.Sprintf("%012d", left.Line), left.RuleID, left.Message}, "\x00") <
			strings.Join([]string{right.Tool, right.Category, right.Path, fmt.Sprintf("%012d", right.Line), right.RuleID, right.Message}, "\x00")
	})
}

func sortBuckets(buckets []ExternalBucket) {
	sort.Slice(buckets, func(i, j int) bool {
		left, right := buckets[i], buckets[j]
		return strings.Join([]string{left.Tool, left.Category, left.Path, fmt.Sprintf("%012d", left.Line)}, "\x00") <
			strings.Join([]string{right.Tool, right.Category, right.Path, fmt.Sprintf("%012d", right.Line)}, "\x00")
	})
}

func normalizeFindingCategory(tool, rawCategory, ruleID, message string) string {
	if tool == "gitleaks" || tool == "trufflehog" {
		return "secrets"
	}
	text := strings.ToLower(strings.Join([]string{rawCategory, ruleID, message}, " "))
	switch {
	case strings.Contains(text, "secret"), strings.Contains(text, "credential"), strings.Contains(text, "token"), strings.Contains(text, "password"), strings.Contains(text, "private key"), strings.Contains(text, "api-key"), strings.Contains(text, "apikey"):
		return "secrets"
	case strings.Contains(text, "cve-"), strings.Contains(text, "vulnerability"), strings.Contains(text, "dependency"):
		return "supply-chain"
	case strings.Contains(text, "tls"), strings.Contains(text, "ssl"), strings.Contains(text, "certificate"):
		return "tls"
	case strings.Contains(text, "shell"), strings.Contains(text, "command injection"), strings.Contains(text, "subprocess"), strings.Contains(text, "os command"):
		return "shell-exec"
	case strings.Contains(text, "eval"), strings.Contains(text, "dynamic code"):
		return "dynamic-code"
	case strings.Contains(text, "innerhtml"), strings.Contains(text, "html"), strings.Contains(text, "xss"):
		return "unsafe-html"
	case strings.Contains(text, "ssrf"):
		return "ssrf"
	case strings.Contains(text, "sql injection"), strings.Contains(text, "taint"), strings.Contains(text, "path traversal"):
		return "taint"
	case strings.Contains(text, "weak hash"), strings.Contains(text, "weak cipher"), strings.Contains(text, "crypto"), strings.Contains(text, "md5"), strings.Contains(text, "sha1"), strings.Contains(text, "des"), strings.Contains(text, "rc4"):
		return "crypto-weakness"
	case strings.Contains(text, "deserial"), strings.Contains(text, "pickle"), strings.Contains(text, "yaml.load"):
		return "deserialization"
	case strings.Contains(text, "cors"):
		return "cors"
	case strings.Contains(text, "docker"), strings.Contains(text, "container"), strings.Contains(text, "root user"):
		return "container-user"
	case strings.Contains(text, "iac"), strings.Contains(text, "misconfig"):
		return "iac"
	default:
		return "uncategorized"
	}
}

func normalizeSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "error", "critical", "high":
		return "error"
	case "warning", "warn", "medium":
		return "warning"
	case "note", "notice", "info", "informational", "low":
		return "note"
	default:
		return ""
	}
}

func cleanExternalPath(path string) string {
	path = strings.TrimSpace(strings.TrimPrefix(path, "file://"))
	if path == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstStringProperty(values map[string]any, keys ...string) string {
	if len(values) == 0 {
		return ""
	}
	for _, key := range keys {
		if value, ok := values[key]; ok {
			if text, ok := value.(string); ok {
				return text
			}
			if list, ok := value.([]any); ok {
				parts := make([]string, 0, len(list))
				for _, item := range list {
					if text, ok := item.(string); ok {
						parts = append(parts, text)
					}
				}
				return strings.Join(parts, " ")
			}
		}
	}
	return ""
}

func stringSliceProperty(values map[string]any, key string) []string {
	if len(values) == 0 {
		return nil
	}
	raw, ok := values[key]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

func firstMapString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok {
			return value
		}
	}
	return ""
}

func firstNestedString(values map[string]any, paths ...[]string) string {
	for _, path := range paths {
		value, ok := nestedValue(values, path).(string)
		if ok {
			return value
		}
	}
	return ""
}

func firstNestedInt(values map[string]any, paths ...[]string) int {
	for _, path := range paths {
		switch value := nestedValue(values, path).(type) {
		case float64:
			return int(value)
		case int:
			return value
		}
	}
	return 0
}

func nestedValue(values map[string]any, path []string) any {
	var current any = values
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = object[key]
	}
	return current
}
