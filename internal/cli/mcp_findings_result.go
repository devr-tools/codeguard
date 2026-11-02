package cli

import (
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// findingsToolResult shapes a fresh report for the requested response format:
// the full report, exactly as 1.x returned it before the summary-first shape,
// or the cached, summary-first first page.
func (s *mcpToolService) findingsToolResult(tool string, format string, report service.Report, filters mcpFindingFilters) map[string]any {
	if format == mcpFormatReport {
		return toolSuccessResult(report)
	}
	return s.findingsResult(tool, report, filters)
}

// findingsResult caches a fresh report and returns its first page.
func (s *mcpToolService) findingsResult(tool string, report service.Report, filters mcpFindingFilters) map[string]any {
	scanID, err := s.reports.put(tool, report)
	if err != nil {
		return toolErrorResult(err.Error())
	}
	return toolSuccessResult(pagedFindings(tool, scanID, report, filters, 0))
}

// resumeFindingsResult serves a later page from the report a cursor refers
// to, without re-running the scan.
func (s *mcpToolService) resumeFindingsResult(tool string, rawCursor string, filters mcpFindingFilters) map[string]any {
	cursor, entry, err := s.reports.resolveCursor(rawCursor, tool, filters)
	if err != nil {
		return toolErrorResult(err.Error())
	}
	resp := pagedFindings(tool, entry.id, entry.report, filters, cursor.Offset)
	if cursor.Offset > resp.Summary.MatchedFindings {
		return toolErrorResult(errCursorPastEnd.Error())
	}
	return toolSuccessResult(resp)
}

func pagedFindings(tool string, scanID string, report service.Report, filters mcpFindingFilters, offset int) mcpFindingsResponse {
	resp := buildFindingsResponse(report, filters, offset)
	resp.ScanID = scanID
	if resp.Truncated {
		resp.NextCursor = encodeCursor(mcpCursor{
			ScanID:  scanID,
			Tool:    tool,
			Offset:  offset + len(resp.Findings),
			Filters: filters.fingerprint(),
		})
	}
	return resp
}
