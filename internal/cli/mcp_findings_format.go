package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Response formats for the findings-returning MCP tools (scan,
// validate_patch).
//
// mcpFormatReport is the full report every 1.x release before the
// summary-first shape returned: top-level sections[] with every finding and
// all of its evidence. It stays the default for a call that passes none of
// the summary inputs, so an MCP client written for 1.10 keeps getting exactly
// what it was built against. The default may change in the next major
// version.
//
// mcpFormatSummary is the summary-first, filtered, paginated response. Passing
// any summary input selects it, as does format "summary" on its own.
const (
	mcpFormatReport  = "report"
	mcpFormatSummary = "summary"
)

// mcpSummaryInputKeys are the inputs that only make sense for the summary
// shape. Passing any of them (with a non-null value) opts the call into it.
var mcpSummaryInputKeys = []string{
	"sections", "rules", "min_level", "min_confidence", "paths",
	"max_findings", "cursor", "verbose",
}

// mcpScanOnlySummaryInputKeys are summary inputs that exist only on scan.
var mcpScanOnlySummaryInputKeys = []string{"changed_only"}

// resolveFindingsFormat decides which response shape a findings tool call
// gets. It looks at which keys the caller actually sent rather than at
// decoded values, so an explicit `verbose: false` or `max_findings: 50` still
// opts in: a new client that sets an input is asking for the new shape.
func resolveFindingsFormat(raw json.RawMessage, extraSummaryKeys ...string) (string, error) {
	var present map[string]json.RawMessage
	if err := json.Unmarshal(raw, &present); err != nil {
		return "", err
	}
	summaryInputs := make([]string, 0, len(mcpSummaryInputKeys)+len(extraSummaryKeys))
	for _, key := range append(append([]string(nil), mcpSummaryInputKeys...), extraSummaryKeys...) {
		if value, ok := present[key]; ok && !bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			summaryInputs = append(summaryInputs, key)
		}
	}
	format, err := requestedFormat(present["format"])
	if err != nil {
		return "", err
	}
	switch format {
	case mcpFormatSummary:
		return mcpFormatSummary, nil
	case mcpFormatReport:
		if len(summaryInputs) > 0 {
			return "", fmt.Errorf("format %q returns the full report and cannot be combined with %s; omit format or set it to %q",
				mcpFormatReport, strings.Join(summaryInputs, ", "), mcpFormatSummary)
		}
		return mcpFormatReport, nil
	}
	if len(summaryInputs) > 0 {
		return mcpFormatSummary, nil
	}
	return mcpFormatReport, nil
}

func requestedFormat(raw json.RawMessage) (string, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("invalid format: must be %q or %q", mcpFormatReport, mcpFormatSummary)
	}
	format := strings.ToLower(strings.TrimSpace(value))
	switch format {
	case "", mcpFormatReport, mcpFormatSummary:
		return format, nil
	default:
		return "", fmt.Errorf("invalid format %q: must be %q or %q", value, mcpFormatReport, mcpFormatSummary)
	}
}
