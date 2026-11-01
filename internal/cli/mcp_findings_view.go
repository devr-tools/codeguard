package cli

import (
	"sort"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// mcpTopRulesLimit caps the top_rules list in the summary.
const mcpTopRulesLimit = 10

// mcpFindingsResponse is the summary-first payload the findings-returning MCP
// tools emit instead of the raw report. Field order is the JSON order, so an
// agent reads the summary before any individual finding.
type mcpFindingsResponse struct {
	Name        string             `json:"name"`
	Profile     string             `json:"profile,omitempty"`
	GeneratedAt string             `json:"generated_at,omitempty"`
	ScanID      string             `json:"scan_id"`
	Summary     mcpFindingsSummary `json:"summary"`
	Findings    []mcpFindingView   `json:"findings"`
	Offset      int                `json:"offset"`
	Truncated   bool               `json:"truncated"`
	NextCursor  string             `json:"next_cursor,omitempty"`
	Artifacts   []service.Artifact `json:"artifacts,omitempty"`
	Filters     mcpFindingFilters  `json:"filters"`
}

type mcpFindingsSummary struct {
	PassedSections     int                 `json:"passed_sections"`
	WarnedSections     int                 `json:"warned_sections"`
	FailedSections     int                 `json:"failed_sections"`
	TotalFindings      int                 `json:"total_findings"`
	SuppressedFindings int                 `json:"suppressed_findings"`
	MatchedFindings    int                 `json:"matched_findings"`
	ReturnedFindings   int                 `json:"returned_findings"`
	FilteredOut        mcpFilteredOut      `json:"filtered_out"`
	TopRules           []mcpRuleCount      `json:"top_rules"`
	Sections           []mcpSectionSummary `json:"sections"`
}

// mcpFilteredOut explains why findings are absent. sections, rules, min_level
// and paths count findings the MCP filters removed from total_findings (each
// attributed to the first filter it failed); min_confidence counts findings
// the scanner's confidence policy dropped before total_findings was computed.
type mcpFilteredOut struct {
	Total         int `json:"total"`
	Sections      int `json:"sections"`
	Rules         int `json:"rules"`
	MinLevel      int `json:"min_level"`
	Paths         int `json:"paths"`
	MinConfidence int `json:"min_confidence"`
}

type mcpRuleCount struct {
	RuleID string `json:"rule_id"`
	Level  string `json:"level"`
	Count  int    `json:"count"`
}

type mcpSectionSummary struct {
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	Status             string              `json:"status"`
	Findings           int                 `json:"findings"`
	Matched            int                 `json:"matched"`
	ConfidenceFiltered int                 `json:"confidence_filtered,omitempty"`
	Suppressed         int                 `json:"suppressed,omitempty"`
	Diagnostics        []mcpDiagnosticView `json:"diagnostics,omitempty"`
}

type mcpDiagnosticView struct {
	ID          string `json:"id"`
	Level       string `json:"level"`
	Message     string `json:"message"`
	Path        string `json:"path,omitempty"`
	Operational bool   `json:"operational,omitempty"`
}

// mcpFindingView is one compact finding. The fields after how_to_fix are
// evidence an agent rarely needs to triage and are only set when verbose.
type mcpFindingView struct {
	RuleID     string `json:"rule_id"`
	Level      string `json:"level"`
	Confidence string `json:"confidence"`
	Section    string `json:"section"`
	Location   string `json:"location,omitempty"`
	Path       string `json:"path,omitempty"`
	Line       int    `json:"line,omitempty"`
	Message    string `json:"message"`
	HowToFix   string `json:"how_to_fix,omitempty"`

	Title       string            `json:"title,omitempty"`
	Severity    string            `json:"severity,omitempty"`
	Why         string            `json:"why,omitempty"`
	Column      int               `json:"column,omitempty"`
	Fingerprint string            `json:"fingerprint,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// mcpSectionFindings pairs a finding with the section that reported it.
type mcpSectionFindings struct {
	section string
	finding core.Finding
}

// buildFindingsResponse filters, orders and pages report for one MCP call.
// offset is the index into the ordered matched findings to start the page at.
func buildFindingsResponse(report service.Report, filters mcpFindingFilters, offset int) mcpFindingsResponse {
	resp := mcpFindingsResponse{
		Name:        report.Name,
		Profile:     report.Profile,
		GeneratedAt: report.GeneratedAt,
		Filters:     filters,
		Offset:      offset,
	}
	resp.Summary = summarizeReportHeader(report)
	matched := resp.collectMatched(report, filters)
	sortSectionFindings(matched)
	resp.Summary.MatchedFindings = len(matched)
	resp.Summary.TopRules = topRules(matched)
	resp.Findings = pageFindings(matched, offset, filters.MaxFindings, filters.Verbose)
	resp.Summary.ReturnedFindings = len(resp.Findings)
	resp.Truncated = offset+len(resp.Findings) < len(matched)
	if filters.Verbose {
		resp.Artifacts = report.Artifacts
	}
	return resp
}

func summarizeReportHeader(report service.Report) mcpFindingsSummary {
	return mcpFindingsSummary{
		PassedSections:     report.Summary.PassedSections,
		WarnedSections:     report.Summary.WarnedSections,
		FailedSections:     report.Summary.FailedSections,
		TotalFindings:      report.Summary.TotalFindings,
		SuppressedFindings: report.Summary.SuppressedFindings,
		TopRules:           []mcpRuleCount{},
		Sections:           []mcpSectionSummary{},
	}
}

// collectMatched walks every section, records per-section and per-reason
// counts on the summary, and returns the findings that passed every filter.
func (resp *mcpFindingsResponse) collectMatched(report service.Report, filters mcpFindingFilters) []mcpSectionFindings {
	matched := make([]mcpSectionFindings, 0)
	for _, section := range report.Sections {
		row := sectionSummaryRow(section, filters.Verbose)
		for _, finding := range section.Findings {
			reason := filters.exclusionReason(section.ID, finding)
			if reason == filterReasonNone {
				row.Matched++
				matched = append(matched, mcpSectionFindings{section: section.ID, finding: finding})
				continue
			}
			resp.Summary.FilteredOut.add(reason)
		}
		resp.Summary.FilteredOut.MinConfidence += section.ConfidenceFilteredCount
		resp.Summary.Sections = append(resp.Summary.Sections, row)
	}
	resp.Summary.FilteredOut.Total += resp.Summary.FilteredOut.MinConfidence
	return matched
}

func (f *mcpFilteredOut) add(reason string) {
	f.Total++
	switch reason {
	case filterReasonSection:
		f.Sections++
	case filterReasonRule:
		f.Rules++
	case filterReasonMinLevel:
		f.MinLevel++
	case filterReasonPath:
		f.Paths++
	}
}

// sectionSummaryRow builds a section's summary line. Operational diagnostics
// (a check that crashed or a target that could not be walked) are always
// listed because they mean the section's findings are incomplete; the rest
// only appear when verbose.
func sectionSummaryRow(section service.SectionResult, verbose bool) mcpSectionSummary {
	row := mcpSectionSummary{
		ID:                 section.ID,
		Name:               section.Name,
		Status:             string(section.Status),
		Findings:           len(section.Findings),
		ConfidenceFiltered: section.ConfidenceFilteredCount,
		Suppressed:         section.SuppressedCount,
	}
	for _, diag := range section.Diagnostics {
		if !diag.Operational && !verbose {
			continue
		}
		row.Diagnostics = append(row.Diagnostics, mcpDiagnosticView{
			ID: diag.ID, Level: diag.Level, Message: diag.Message, Path: diag.Path, Operational: diag.Operational,
		})
	}
	return row
}

// sortSectionFindings orders by level (fail first), then confidence (high
// first), then path and line, with rule id and fingerprint as tie-breakers so
// the order — and therefore every cursor offset — is fully deterministic.
func sortSectionFindings(items []mcpSectionFindings) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].finding, items[j].finding
		if ra, rb := levelRank(a.Level), levelRank(b.Level); ra != rb {
			return ra > rb
		}
		if ca, cb := core.ConfidenceRank(a.Confidence), core.ConfidenceRank(b.Confidence); ca != cb {
			return ca > cb
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		return a.Fingerprint < b.Fingerprint
	})
}

func topRules(items []mcpSectionFindings) []mcpRuleCount {
	counts := map[string]*mcpRuleCount{}
	for _, item := range items {
		entry, ok := counts[item.finding.RuleID]
		if !ok {
			entry = &mcpRuleCount{RuleID: item.finding.RuleID, Level: item.finding.Level}
			counts[item.finding.RuleID] = entry
		}
		entry.Count++
		if levelRank(item.finding.Level) > levelRank(entry.Level) {
			entry.Level = item.finding.Level
		}
	}
	out := make([]mcpRuleCount, 0, len(counts))
	for _, entry := range counts {
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].RuleID < out[j].RuleID
	})
	if len(out) > mcpTopRulesLimit {
		out = out[:mcpTopRulesLimit]
	}
	return out
}

func pageFindings(items []mcpSectionFindings, offset int, limit int, verbose bool) []mcpFindingView {
	out := make([]mcpFindingView, 0, min(limit, max(len(items)-offset, 0)))
	for i := offset; i < len(items) && len(out) < limit; i++ {
		out = append(out, compactFinding(items[i].section, items[i].finding, verbose))
	}
	return out
}

func compactFinding(sectionID string, finding core.Finding, verbose bool) mcpFindingView {
	confidence := core.NormalizedConfidence(finding.Confidence)
	if confidence == "" {
		confidence = core.ConfidenceMedium
	}
	view := mcpFindingView{
		RuleID:     finding.RuleID,
		Level:      finding.Level,
		Confidence: confidence,
		Section:    sectionID,
		Location:   findingLocation(finding),
		Path:       finding.Path,
		Line:       finding.Line,
		Message:    finding.Message,
		HowToFix:   strings.TrimSpace(finding.HowToFix),
	}
	if verbose {
		view.Title = finding.Title
		view.Severity = finding.Severity
		view.Why = finding.Why
		view.Column = finding.Column
		view.Fingerprint = finding.Fingerprint
		view.Metadata = finding.Metadata
	}
	return view
}

func findingLocation(finding core.Finding) string {
	if finding.Path == "" {
		return ""
	}
	if finding.Line <= 0 {
		return finding.Path
	}
	return finding.Path + ":" + strconv.Itoa(finding.Line)
}
