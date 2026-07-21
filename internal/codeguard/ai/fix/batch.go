package fix

import (
	"context"
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/rules"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// VerifyBatch verifies compatible deterministic fixes as one aggregate patch.
// It conservatively treats any two candidates that edit the same file as a
// conflict. This avoids ordering-dependent patches and lets the caller send
// those cases through reviewed single-fix verification instead.
func VerifyBatch(ctx context.Context, req BatchRequest) (BatchResult, error) {
	result := BatchResult{}
	catalog := rules.Catalog()
	claimedFiles := make(map[string]struct{})
	diffs := make([]string, 0, len(req.Items))

	for index, item := range req.Items {
		issue := BatchIssue{Index: index, RuleID: item.Finding.RuleID, Fingerprint: item.Finding.Fingerprint}
		metadata, knownRule := catalog[item.Finding.RuleID]
		if !knownRule {
			issue.Reason = BatchReasonUnknownRule
			result.Skipped = append(result.Skipped, issue)
			continue
		}
		if metadata.FixTemplate.Kind != core.FixTemplateKindDeterministic {
			issue.Reason = BatchReasonNonDeterministic
			result.Skipped = append(result.Skipped, issue)
			continue
		}

		diff := strings.TrimSpace(item.Candidate.Diff)
		if diff == "" {
			issue.Reason = BatchReasonEmptyDiff
			result.Skipped = append(result.Skipped, issue)
			continue
		}
		files := runnersupport.ChangedFilesFromUnifiedDiff(diff)
		if len(files) == 0 {
			issue.Reason = BatchReasonNoChangedFiles
			result.Skipped = append(result.Skipped, issue)
			continue
		}
		conflicts := conflictingFiles(files, claimedFiles)
		if len(conflicts) > 0 {
			issue.Reason = BatchReasonConflictingFiles
			issue.Detail = strings.Join(conflicts, ",")
			result.Skipped = append(result.Skipped, issue)
			continue
		}
		for _, file := range files {
			claimedFiles[file] = struct{}{}
		}
		diffs = append(diffs, diff)
		result.Included = append(result.Included, index)
	}

	if len(diffs) == 0 {
		return result, nil
	}

	aggregate := strings.Join(diffs, "\n")
	verified, err := Verify(ctx, req.Config, core.Finding{}, Candidate{
		Summary: fmt.Sprintf("verified batch of %d deterministic fixes", len(result.Included)),
		Diff:    aggregate,
	}, req.Options)
	if err != nil {
		for _, index := range result.Included {
			item := req.Items[index]
			result.Failures = append(result.Failures, BatchIssue{
				Index:       index,
				RuleID:      item.Finding.RuleID,
				Fingerprint: item.Finding.Fingerprint,
				Reason:      BatchReasonAggregateVerification,
				Detail:      err.Error(),
			})
		}
		result.Included = nil
		return result, fmt.Errorf("verify batch: %w", err)
	}
	result.Verification = verified
	return result, nil
}

func conflictingFiles(files []string, claimed map[string]struct{}) []string {
	conflicts := make([]string, 0)
	for _, file := range files {
		if _, exists := claimed[file]; exists {
			conflicts = append(conflicts, file)
		}
	}
	return conflicts
}
