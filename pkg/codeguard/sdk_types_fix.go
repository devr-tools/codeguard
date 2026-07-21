package codeguard

import internalfix "github.com/devr-tools/codeguard/internal/codeguard/ai/fix"

type FixGenerator = internalfix.Generator
type FixGenerateInput = internalfix.GenerateInput
type FixCandidate = internalfix.Candidate
type FixGenerateRequest = internalfix.GenerateRequest
type FixOptions = internalfix.Options
type FixVerificationCommand = internalfix.VerificationCommand
type FixCommandResult = internalfix.CommandResult
type VerifiedFix = internalfix.Result
type FixBatchItem = internalfix.BatchItem
type FixBatchIssue = internalfix.BatchIssue
type FixBatchRequest = internalfix.BatchRequest
type FixBatchResult = internalfix.BatchResult

const (
	FixBatchReasonUnknownRule           = internalfix.BatchReasonUnknownRule
	FixBatchReasonNonDeterministic      = internalfix.BatchReasonNonDeterministic
	FixBatchReasonEmptyDiff             = internalfix.BatchReasonEmptyDiff
	FixBatchReasonNoChangedFiles        = internalfix.BatchReasonNoChangedFiles
	FixBatchReasonConflictingFiles      = internalfix.BatchReasonConflictingFiles
	FixBatchReasonAggregateVerification = internalfix.BatchReasonAggregateVerification
)
