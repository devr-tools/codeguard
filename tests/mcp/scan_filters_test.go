package mcp_test

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// 30 prompt files -> 30 fail + 30 warn prompt findings, plus 1 fail in other/.
const largeFixtureFiles = 30

// format "summary" alone (no filter) is how a new client opts into the
// summary-first shape; a call with no inputs at all keeps the 1.10 full report
// (see legacy_format_test.go).
func TestMCPScanSummaryFormatDefaultsToFiftyFindings(t *testing.T) {
	dir := t.TempDir()
	client := newMCPStdioClient(t, writePromptFixture(t, dir, largeFixtureFiles))

	got := client.mustCall("scan", map[string]any{"format": "summary"})
	if got.Summary.TotalFindings < 2*largeFixtureFiles+1 {
		t.Fatalf("total_findings = %d, want at least %d", got.Summary.TotalFindings, 2*largeFixtureFiles+1)
	}
	if len(got.Findings) != 50 || got.Summary.ReturnedFindings != 50 {
		t.Fatalf("default page returned %d findings (summary %d), want 50", len(got.Findings), got.Summary.ReturnedFindings)
	}
	if !got.Truncated || got.NextCursor == "" {
		t.Fatalf("expected truncated=true with next_cursor, got truncated=%v cursor=%q", got.Truncated, got.NextCursor)
	}
	if got.Summary.MatchedFindings != got.Summary.TotalFindings || got.Summary.FilteredOut.Total != 0 {
		t.Fatalf("unfiltered call should match every finding: %+v", got.Summary)
	}
	if got.ScanID == "" || got.Offset != 0 {
		t.Fatalf("expected scan_id and offset 0, got %q / %d", got.ScanID, got.Offset)
	}
	assertSectionSummary(t, got)
	assertTopRules(t, got)
	assertSeverityOrder(t, got.Findings)
	assertCompactFindings(t, got.Findings)
}

func assertSectionSummary(t *testing.T, got findingsPayload) {
	t.Helper()
	for _, section := range got.Summary.Sections {
		if section.ID == "prompts" {
			if section.Status != "fail" || section.Findings != 2*largeFixtureFiles+1 || section.Matched != section.Findings {
				t.Fatalf("unexpected prompts section summary: %+v", section)
			}
			return
		}
	}
	t.Fatalf("prompts section missing from summary: %+v", got.Summary.Sections)
}

func assertTopRules(t *testing.T, got findingsPayload) {
	t.Helper()
	if len(got.Summary.TopRules) < 2 {
		t.Fatalf("expected at least 2 top rules, got %+v", got.Summary.TopRules)
	}
	top := got.Summary.TopRules[0]
	if top.RuleID != "prompts.secret-interpolation" || top.Count != largeFixtureFiles+1 {
		t.Fatalf("unexpected top rule: %+v", top)
	}
	for i := 1; i < len(got.Summary.TopRules); i++ {
		if got.Summary.TopRules[i].Count > got.Summary.TopRules[i-1].Count {
			t.Fatalf("top_rules not sorted by count: %+v", got.Summary.TopRules)
		}
	}
}

func assertSeverityOrder(t *testing.T, findings []findingView) {
	t.Helper()
	rank := map[string]int{"fail": 2, "warn": 1}
	for i := 1; i < len(findings); i++ {
		prev, cur := findings[i-1], findings[i]
		if rank[cur.Level] > rank[prev.Level] {
			t.Fatalf("finding %d (%s) ranks above previous (%s)", i, cur.Level, prev.Level)
		}
		if rank[cur.Level] == rank[prev.Level] && cur.Confidence == prev.Confidence && cur.Path < prev.Path {
			t.Fatalf("findings %d/%d not ordered by path: %s then %s", i-1, i, prev.Path, cur.Path)
		}
	}
}

func assertCompactFindings(t *testing.T, findings []findingView) {
	t.Helper()
	for _, finding := range findings {
		if finding.RuleID == "" || finding.Level == "" || finding.Confidence == "" || finding.Message == "" || finding.Section == "" {
			t.Fatalf("compact finding missing core fields: %+v", finding)
		}
		if finding.Path != "" && !strings.HasPrefix(finding.Location, finding.Path) {
			t.Fatalf("location %q does not start with path %q", finding.Location, finding.Path)
		}
		if finding.Why != "" || finding.Fingerprint != "" {
			t.Fatalf("compact finding leaked verbose evidence: %+v", finding)
		}
	}
}

func TestMCPScanVerboseIncludesEvidence(t *testing.T) {
	dir := t.TempDir()
	client := newMCPStdioClient(t, writePromptFixture(t, dir, 2))

	got := client.mustCall("scan", map[string]any{"verbose": true, "max_findings": 3})
	if len(got.Findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(got.Findings))
	}
	for _, finding := range got.Findings {
		if finding.Fingerprint == "" {
			t.Fatalf("verbose finding missing fingerprint: %+v", finding)
		}
	}
}

func TestMCPScanFilters(t *testing.T) {
	dir := t.TempDir()
	client := newMCPStdioClient(t, writePromptFixture(t, dir, 4))
	baseline := client.mustCall("scan", map[string]any{"max_findings": 500})
	total := baseline.Summary.TotalFindings

	cases := []struct {
		name    string
		args    map[string]any
		want    func(findingView) bool
		matched int
		reason  func(findingsPayload) int
	}{
		{
			name: "sections", args: map[string]any{"sections": []string{"prompts"}},
			want: func(f findingView) bool { return f.Section == "prompts" }, matched: 9,
			reason: func(p findingsPayload) int { return p.Summary.FilteredOut.Sections },
		},
		{
			name: "rules exact", args: map[string]any{"rules": []string{"prompts.unsafe-instructions"}},
			want: func(f findingView) bool { return f.RuleID == "prompts.unsafe-instructions" }, matched: 4,
			reason: func(p findingsPayload) int { return p.Summary.FilteredOut.Rules },
		},
		{
			name: "rules prefix", args: map[string]any{"rules": []string{"prompts."}},
			want: func(f findingView) bool { return strings.HasPrefix(f.RuleID, "prompts.") }, matched: 9,
			reason: func(p findingsPayload) int { return p.Summary.FilteredOut.Rules },
		},
		{
			name: "rules star prefix", args: map[string]any{"rules": []string{"prompts.secret*"}},
			want: func(f findingView) bool { return f.RuleID == "prompts.secret-interpolation" }, matched: 5,
			reason: func(p findingsPayload) int { return p.Summary.FilteredOut.Rules },
		},
		{
			name: "min_level fail", args: map[string]any{"min_level": "fail"},
			want: func(f findingView) bool { return f.Level == "fail" }, matched: 5,
			reason: func(p findingsPayload) int { return p.Summary.FilteredOut.MinLevel },
		},
		{
			name: "paths glob", args: map[string]any{"paths": []string{"other/**"}},
			want: func(f findingView) bool { return strings.HasPrefix(f.Path, "other/") }, matched: 1,
			reason: func(p findingsPayload) int { return p.Summary.FilteredOut.Paths },
		},
		{
			name: "paths single segment", args: map[string]any{"paths": []string{"prompts/p01.prompt", "prompts/p02.*", "prompts/*3.prompt"}},
			want: func(f findingView) bool {
				return f.Path == "prompts/p01.prompt" || f.Path == "prompts/p02.prompt" || f.Path == "prompts/p03.prompt"
			},
			matched: 6,
			reason:  func(p findingsPayload) int { return p.Summary.FilteredOut.Paths },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.args["max_findings"] = 500
			got := client.mustCall("scan", tc.args)
			if got.Summary.MatchedFindings != tc.matched || len(got.Findings) != tc.matched {
				t.Fatalf("matched=%d returned=%d, want %d", got.Summary.MatchedFindings, len(got.Findings), tc.matched)
			}
			for _, finding := range got.Findings {
				if !tc.want(finding) {
					t.Fatalf("finding escaped filter: %+v", finding)
				}
			}
			if got.Summary.TotalFindings != total {
				t.Fatalf("total_findings changed under a post-scan filter: %d vs %d", got.Summary.TotalFindings, total)
			}
			if r := tc.reason(got); r != total-tc.matched || got.Summary.FilteredOut.Total != r {
				t.Fatalf("filtered_out reason=%d total=%d, want %d", r, got.Summary.FilteredOut.Total, total-tc.matched)
			}
		})
	}
}

func TestMCPScanMinConfidenceUsesConfigPolicy(t *testing.T) {
	dir := t.TempDir()
	client := newMCPStdioClient(t, writePromptFixture(t, dir, 3))
	baseline := client.mustCall("scan", map[string]any{"max_findings": 500})

	// The prompt findings carry no explicit confidence, which ranks as medium.
	medium := client.mustCall("scan", map[string]any{"min_confidence": "medium", "max_findings": 500})
	if medium.Summary.TotalFindings != baseline.Summary.TotalFindings || medium.Summary.FilteredOut.MinConfidence != 0 {
		t.Fatalf("min_confidence=medium should keep medium findings: %+v", medium.Summary)
	}
	high := client.mustCall("scan", map[string]any{"min_confidence": "high", "max_findings": 500})
	if high.Summary.MatchedFindings != 0 || len(high.Findings) != 0 {
		t.Fatalf("min_confidence=high should drop medium findings, got %d", high.Summary.MatchedFindings)
	}
	if high.Summary.FilteredOut.MinConfidence != baseline.Summary.TotalFindings || high.Summary.FilteredOut.Total != baseline.Summary.TotalFindings {
		t.Fatalf("expected %d confidence-filtered findings, got %+v", baseline.Summary.TotalFindings, high.Summary.FilteredOut)
	}
	// Applied through checks.min_confidence, it also changes the section verdict.
	if high.Summary.FailedSections != 0 {
		t.Fatalf("expected no failed sections once every finding is confidence-filtered, got %d", high.Summary.FailedSections)
	}
}

func TestMCPScanRejectsInvalidFilters(t *testing.T) {
	dir := t.TempDir()
	client := newMCPStdioClient(t, writePromptFixture(t, dir, 1))
	cases := map[string]map[string]any{
		"unknown section":     {"sections": []string{"nope"}},
		"bad min_level":       {"min_level": "info"},
		"bad min_confidence":  {"min_confidence": "certain"},
		"negative page":       {"max_findings": -1},
		"garbage cursor":      {"cursor": "not-a-cursor"},
		"changed_only + full": {"changed_only": true, "mode": "full"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if result := client.callTool("scan", args); !result.IsError {
				t.Fatalf("expected isError for %v, got %s", args, result.Raw)
			}
		})
	}
}

func TestMCPScanPaginationIsStableWithoutDuplicatesOrGaps(t *testing.T) {
	dir := t.TempDir()
	client := newMCPStdioClient(t, writePromptFixture(t, dir, largeFixtureFiles))

	whole := client.mustCall("scan", map[string]any{"max_findings": 500})
	if whole.Truncated || whole.NextCursor != "" {
		t.Fatalf("a page holding every finding must not be truncated")
	}
	paged, pages := client.collectAllPages("scan", map[string]any{"max_findings": 7})
	wantPages := (whole.Summary.MatchedFindings + 6) / 7
	if pages != wantPages {
		t.Fatalf("pages = %d, want %d", pages, wantPages)
	}
	seen := map[string]bool{}
	for _, finding := range paged {
		if seen[finding.key()] {
			t.Fatalf("duplicate finding across pages: %s", finding.key())
		}
		seen[finding.key()] = true
	}
	if !reflect.DeepEqual(keys(paged), keys(whole.Findings)) {
		t.Fatalf("paged order differs from single-page order:\npaged=%v\nwhole=%v", keys(paged), keys(whole.Findings))
	}

	// Filters and page size travel with the call: pages of a filtered scan
	// still cover exactly the filtered set, even if max_findings changes.
	filtered, _ := client.collectAllPages("scan", map[string]any{"max_findings": 4, "min_level": "warn", "rules": []string{"prompts.unsafe-instructions"}})
	if len(filtered) != largeFixtureFiles {
		t.Fatalf("filtered pagination returned %d findings, want %d", len(filtered), largeFixtureFiles)
	}
}

func TestMCPScanCursorRejectsMismatchedFilters(t *testing.T) {
	dir := t.TempDir()
	client := newMCPStdioClient(t, writePromptFixture(t, dir, 5))
	first := client.mustCall("scan", map[string]any{"max_findings": 2, "min_level": "warn"})
	if first.NextCursor == "" {
		t.Fatalf("expected a cursor")
	}
	if result := client.callTool("scan", map[string]any{"cursor": first.NextCursor, "min_level": "fail"}); !result.IsError || !strings.Contains(result.Text, "different filters") {
		t.Fatalf("expected filter-mismatch error, got %s", result.Raw)
	}
	if result := client.callTool("validate_patch", map[string]any{"cursor": first.NextCursor, "min_level": "warn"}); !result.IsError {
		t.Fatalf("expected a scan cursor to be rejected by validate_patch, got %s", result.Raw)
	}
	// Changing only the page size or verbosity keeps the cursor valid.
	next := client.mustCall("scan", map[string]any{"cursor": first.NextCursor, "min_level": "warn", "max_findings": 3, "verbose": true})
	if next.Offset != 2 || len(next.Findings) != 3 || next.ScanID != first.ScanID {
		t.Fatalf("unexpected resumed page: offset=%d n=%d scan=%s", next.Offset, len(next.Findings), next.ScanID)
	}
}

func TestMCPScanSummaryOnlyPage(t *testing.T) {
	dir := t.TempDir()
	client := newMCPStdioClient(t, writePromptFixture(t, dir, 2))
	got := client.mustCall("scan", map[string]any{"max_findings": 0})
	if len(got.Findings) != 0 || got.Summary.ReturnedFindings != 0 {
		t.Fatalf("max_findings=0 should return no findings, got %d", len(got.Findings))
	}
	if !got.Truncated || got.NextCursor == "" || got.Summary.MatchedFindings == 0 {
		t.Fatalf("summary-only page should be truncated with a cursor: %+v", got.Summary)
	}
	page := client.mustCall("scan", map[string]any{"cursor": got.NextCursor, "max_findings": 500})
	if len(page.Findings) != got.Summary.MatchedFindings || page.Truncated {
		t.Fatalf("cursor page returned %d of %d findings", len(page.Findings), got.Summary.MatchedFindings)
	}
}

func TestMCPScanChangedOnlyLimitsToDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	configPath := writePromptConfig(t, dir)
	writeFixtureFile(t, filepath.Join(dir, "prompts", "committed.prompt"), "Use ${OPENAI_API_KEY} here.\n")
	writeFixtureFile(t, filepath.Join(dir, "prompts", "edited.prompt"), "Keep prompts generic.\n")
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "base")
	writeFixtureFile(t, filepath.Join(dir, "prompts", "edited.prompt"), "Use ${OPENAI_API_KEY} now.\n")

	client := newMCPStdioClient(t, configPath)
	full := client.mustCall("scan", map[string]any{"rules": []string{"prompts.secret-interpolation"}})
	if paths := findingPaths(full.Findings); !reflect.DeepEqual(paths, []string{"prompts/committed.prompt", "prompts/edited.prompt"}) {
		t.Fatalf("full scan paths = %v", paths)
	}
	changed := client.mustCall("scan", map[string]any{"changed_only": true, "base_ref": "HEAD", "rules": []string{"prompts.secret-interpolation"}})
	if paths := findingPaths(changed.Findings); !reflect.DeepEqual(paths, []string{"prompts/edited.prompt"}) {
		t.Fatalf("changed_only paths = %v, want only the edited file", paths)
	}
}

func TestMCPValidatePatchSharesFiltersAndPaging(t *testing.T) {
	dir := t.TempDir()
	replacements := setupPromptConfig(t, dir)
	client := newMCPStdioClient(t, replacements["__CONFIG_PATH__"])
	diff := replacements["__DIFF__"]

	whole := client.mustCall("validate_patch", map[string]any{"diff": diff, "format": "summary"})
	if whole.Summary.FailedSections == 0 || whole.Summary.TotalFindings == 0 || len(whole.Findings) == 0 {
		t.Fatalf("expected failing patch findings: %+v", whole.Summary)
	}
	summaryOnly := client.mustCall("validate_patch", map[string]any{"diff": diff, "max_findings": 0, "min_level": "fail"})
	if len(summaryOnly.Findings) != 0 || summaryOnly.NextCursor == "" {
		t.Fatalf("expected summary-only page with cursor: %+v", summaryOnly)
	}
	page := client.mustCall("validate_patch", map[string]any{"cursor": summaryOnly.NextCursor, "min_level": "fail"})
	if len(page.Findings) != summaryOnly.Summary.MatchedFindings {
		t.Fatalf("cursor page returned %d, want %d", len(page.Findings), summaryOnly.Summary.MatchedFindings)
	}
	for _, finding := range page.Findings {
		if finding.Level != "fail" {
			t.Fatalf("min_level=fail leaked %+v", finding)
		}
	}
	none := client.mustCall("validate_patch", map[string]any{"diff": diff, "paths": []string{"does/not/exist/**"}})
	if none.Summary.MatchedFindings != 0 || none.Summary.FilteredOut.Paths != whole.Summary.TotalFindings {
		t.Fatalf("paths filter not applied to validate_patch: %+v", none.Summary)
	}
}

func TestMCPToolsListAdvertisesFindingFilters(t *testing.T) {
	dir := t.TempDir()
	client := newMCPStdioClient(t, writePromptFixture(t, dir, 1))
	line := client.request("tools/list", map[string]any{})
	var resp struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				InputSchema struct {
					Properties map[string]json.RawMessage `json:"properties"`
					Required   []string                   `json:"required"`
				} `json:"inputSchema"`
				OutputSchema struct {
					Properties map[string]json.RawMessage `json:"properties"`
				} `json:"outputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	decodeLine(t, line, &resp)
	shared := []string{"format", "sections", "rules", "min_level", "min_confidence", "paths", "max_findings", "cursor", "verbose"}
	want := map[string][]string{
		"scan":           append([]string{"changed_only", "base_ref", "mode", "target_path"}, shared...),
		"validate_patch": append([]string{"diff"}, shared...),
	}
	for _, tool := range resp.Result.Tools {
		props, ok := want[tool.Name]
		if !ok {
			continue
		}
		delete(want, tool.Name)
		for _, prop := range props {
			if _, ok := tool.InputSchema.Properties[prop]; !ok {
				t.Fatalf("%s inputSchema missing %q: %v", tool.Name, prop, sortedKeys(tool.InputSchema.Properties))
			}
		}
		for _, prop := range []string{"summary", "findings", "truncated", "next_cursor"} {
			if _, ok := tool.OutputSchema.Properties[prop]; !ok {
				t.Fatalf("%s outputSchema missing %q", tool.Name, prop)
			}
		}
		assertMaxFindingsSchema(t, tool.Name, tool.InputSchema.Properties["max_findings"])
		assertEnum(t, tool.InputSchema.Properties["min_level"], []string{"warn", "fail"})
		assertEnum(t, tool.InputSchema.Properties["min_confidence"], []string{"low", "medium", "high"})
	}
	if len(want) != 0 {
		t.Fatalf("tools missing from tools/list: %v", want)
	}
}

func assertMaxFindingsSchema(t *testing.T, tool string, raw json.RawMessage) {
	t.Helper()
	var schema struct {
		Type    string `json:"type"`
		Default int    `json:"default"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil || schema.Type != "integer" || schema.Default != 50 {
		t.Fatalf("%s max_findings schema = %s", tool, raw)
	}
}

func assertEnum(t *testing.T, raw json.RawMessage, want []string) {
	t.Helper()
	var schema struct {
		Enum []string `json:"enum"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil || !reflect.DeepEqual(schema.Enum, want) {
		t.Fatalf("enum = %s, want %v", raw, want)
	}
}

func keys(findings []findingView) []string {
	out := make([]string, 0, len(findings))
	for _, finding := range findings {
		out = append(out, finding.key())
	}
	return out
}

func findingPaths(findings []findingView) []string {
	out := make([]string, 0, len(findings))
	for _, finding := range findings {
		out = append(out, finding.Path)
	}
	sort.Strings(out)
	return out
}

func sortedKeys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
