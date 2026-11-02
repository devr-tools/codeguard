package mcp_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// A call with none of the summary inputs must return exactly what 1.10
// returned: the full report, byte for byte apart from the timestamp.
func TestMCPLegacyGoldenScan(t *testing.T) {
	dir := t.TempDir()
	configPath := writeLegacyFixture(t, dir)

	if legacy := legacyBinaryClient(t, configPath); legacy != nil {
		assertLegacyGolden(t, legacyScanGolden, normalizeLegacyResult(t, legacy.rawToolResult("scan", map[string]any{}), dir), true)
		assertLegacyGolden(t, legacyScanSchemaGolden, toolOutputSchema(t, legacy, "scan"), true)
	}

	client := newMCPStdioClient(t, configPath)
	for name, args := range map[string]map[string]any{
		"no arguments":         {},
		"scan-shaping only":    {"mode": "full"},
		"explicit report":      {"format": "report"},
		"null summary inputs":  {"max_findings": nil, "cursor": nil, "verbose": nil, "sections": nil, "changed_only": nil},
		"null format":          {"format": nil},
		"blank format":         {"format": ""},
		"report with mode":     {"format": "report", "mode": "full"},
		"config path override": {"config_path": configPath},
	} {
		t.Run(name, func(t *testing.T) {
			got := normalizeLegacyResult(t, client.rawToolResult("scan", args), dir)
			assertLegacyGolden(t, legacyScanGolden, got, false)
		})
	}
}

func TestMCPLegacyGoldenValidatePatch(t *testing.T) {
	dir := t.TempDir()
	configPath := writeLegacyFixture(t, dir)
	args := map[string]any{"diff": legacyPatchDiff}

	if legacy := legacyBinaryClient(t, configPath); legacy != nil {
		assertLegacyGolden(t, legacyPatchGolden, normalizeLegacyResult(t, legacy.rawToolResult("validate_patch", args), dir), true)
	}

	client := newMCPStdioClient(t, configPath)
	assertLegacyGolden(t, legacyPatchGolden, normalizeLegacyResult(t, client.rawToolResult("validate_patch", args), dir), false)
	explicit := map[string]any{"diff": legacyPatchDiff, "format": "report"}
	assertLegacyGolden(t, legacyPatchGolden, normalizeLegacyResult(t, client.rawToolResult("validate_patch", explicit), dir), false)
}

// The 1.10 output schema must still accept the legacy response, and the
// schema advertised now must accept both shapes, each through exactly one
// oneOf branch.
func TestMCPOutputSchemaDescribesBothShapes(t *testing.T) {
	dir := t.TempDir()
	configPath := writeLegacyFixture(t, dir)
	client := newMCPStdioClient(t, configPath)

	oldRaw, err := os.ReadFile(legacyScanSchemaGolden)
	if err != nil {
		t.Fatalf("read 1.10 schema: %v", err)
	}
	oldSchema := mustDecodeSchema(t, oldRaw)

	for _, tool := range []struct {
		name string
		args map[string]any
	}{
		{"scan", map[string]any{}},
		{"validate_patch", map[string]any{"diff": legacyPatchDiff}},
	} {
		t.Run(tool.name, func(t *testing.T) {
			current := mustDecodeSchema(t, toolOutputSchema(t, client, tool.name))
			if current["type"] != "object" {
				t.Fatalf("outputSchema type = %v, want object", current["type"])
			}
			legacy := structuredContent(t, client.rawToolResult(tool.name, tool.args))
			if err := validateSchema(oldSchema, legacy, "$"); err != nil {
				t.Fatalf("1.10 schema rejects legacy response: %v", err)
			}
			if err := validateSchema(current, legacy, "$"); err != nil {
				t.Fatalf("advertised schema rejects legacy response: %v", err)
			}
			summaryArgs := map[string]any{"format": "summary"}
			for key, value := range tool.args {
				summaryArgs[key] = value
			}
			summary := structuredContent(t, client.rawToolResult(tool.name, summaryArgs))
			if err := validateSchema(current, summary, "$"); err != nil {
				t.Fatalf("advertised schema rejects summary response: %v", err)
			}
			assertOneOfTitles(t, current, legacy, "report")
			assertOneOfTitles(t, current, summary, "summary")
		})
	}
}

func assertOneOfTitles(t *testing.T, schema map[string]any, value any, want string) {
	t.Helper()
	branches, _ := schema["oneOf"].([]any)
	var matched []string
	for _, branch := range branches {
		sub, _ := branch.(map[string]any)
		if validateSchema(sub, value, "$") == nil {
			matched = append(matched, sub["title"].(string))
		}
	}
	if len(matched) != 1 || matched[0] != want {
		t.Fatalf("response matched oneOf branches %v, want only %q", matched, want)
	}
}

// Every summary input, even with a default-looking value, opts the call into
// the summary-first shape.
func TestMCPEachSummaryInputSelectsSummaryShape(t *testing.T) {
	dir := t.TempDir()
	client := newMCPStdioClient(t, writeLegacyFixture(t, dir))

	shared := map[string]any{
		"format":         "summary",
		"sections":       []string{"prompts"},
		"rules":          []string{"prompts."},
		"min_level":      "warn",
		"min_confidence": "low",
		"paths":          []string{"**"},
		"max_findings":   50,
		"verbose":        false,
	}
	for _, tool := range []struct {
		name  string
		base  map[string]any
		extra map[string]any
	}{
		{"scan", map[string]any{}, map[string]any{"changed_only": false}},
		{"validate_patch", map[string]any{"diff": legacyPatchDiff}, nil},
	} {
		inputs := map[string]any{}
		for key, value := range shared {
			inputs[key] = value
		}
		for key, value := range tool.extra {
			inputs[key] = value
		}
		for key, value := range inputs {
			t.Run(tool.name+"/"+key, func(t *testing.T) {
				assertSummaryShape(t, client.rawToolResult(tool.name, withArgs(tool.base, map[string]any{key: value})))
			})
		}
		// The report cache keeps only recent scans, so take the cursor
		// right before using it.
		t.Run(tool.name+"/cursor", func(t *testing.T) {
			first := client.mustCall(tool.name, withArgs(tool.base, map[string]any{"format": "summary", "max_findings": 1}))
			if first.NextCursor == "" {
				t.Fatalf("expected a cursor from a one-finding page")
			}
			assertSummaryShape(t, client.rawToolResult(tool.name, map[string]any{"cursor": first.NextCursor}))
		})
	}
}

func TestMCPFormatSummaryUsesSummaryDefaults(t *testing.T) {
	dir := t.TempDir()
	client := newMCPStdioClient(t, writeLegacyFixture(t, dir))

	got := client.mustCall("scan", map[string]any{"format": "summary"})
	if got.ScanID == "" || got.Truncated || got.NextCursor != "" {
		t.Fatalf("unexpected paging fields: scan_id=%q truncated=%v cursor=%q", got.ScanID, got.Truncated, got.NextCursor)
	}
	if got.Summary.TotalFindings != 5 || got.Summary.MatchedFindings != 5 || len(got.Findings) != 5 {
		t.Fatalf("summary should return all 5 findings unfiltered: %+v", got.Summary)
	}
	assertSeverityOrder(t, got.Findings)
	assertCompactFindings(t, got.Findings)

	raw := client.rawToolResult("scan", map[string]any{"format": "summary"})
	content := structuredContent(t, raw).(map[string]any)
	filters, _ := content["filters"].(map[string]any)
	if filters["max_findings"] != float64(50) {
		t.Fatalf("format summary should apply the default page size, filters = %v", filters)
	}

	patch := client.mustCall("validate_patch", map[string]any{"diff": legacyPatchDiff, "format": "summary"})
	if patch.ScanID == "" || patch.Summary.TotalFindings == 0 || len(patch.Findings) != patch.Summary.TotalFindings {
		t.Fatalf("validate_patch format summary: %+v", patch.Summary)
	}
}

func TestMCPFormatRejectsConflictsAndUnknownValues(t *testing.T) {
	dir := t.TempDir()
	client := newMCPStdioClient(t, writeLegacyFixture(t, dir))

	cases := []struct {
		tool string
		args map[string]any
		want string
	}{
		{"scan", map[string]any{"format": "report", "min_level": "fail"}, "min_level"},
		{"scan", map[string]any{"format": "report", "changed_only": true}, "changed_only"},
		{"scan", map[string]any{"format": "xml"}, "invalid format"},
		{"scan", map[string]any{"format": 3}, "invalid format"},
		{"validate_patch", map[string]any{"diff": legacyPatchDiff, "format": "report", "max_findings": 5}, "max_findings"},
		{"validate_patch", map[string]any{"diff": legacyPatchDiff, "format": "full"}, "invalid format"},
	}
	for _, tc := range cases {
		result := client.callTool(tc.tool, tc.args)
		if !result.IsError || !strings.Contains(result.Text, tc.want) {
			t.Fatalf("%s %v: expected error mentioning %q, got isError=%v %s", tc.tool, tc.args, tc.want, result.IsError, result.Text)
		}
	}
}

func assertSummaryShape(t *testing.T, raw json.RawMessage) {
	t.Helper()
	keys := topLevelKeys(t, structuredContent(t, raw))
	for _, key := range []string{"scan_id", "summary", "findings", "truncated", "filters"} {
		if !keys[key] {
			t.Fatalf("summary response missing %q: %s", key, raw)
		}
	}
	if keys["sections"] {
		t.Fatalf("summary response carries the legacy top-level sections[]: %s", raw)
	}
	if bytes.Contains(raw, []byte(`"context_fingerprint"`)) {
		t.Fatalf("summary response leaked full report evidence: %s", raw)
	}
}

func withArgs(base map[string]any, extra map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(extra))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}

func mustDecodeSchema(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	schema, err := decodeSchema(raw)
	if err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	return schema
}
