package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var qualityErrorDefensiveFixTemplates = map[string]core.FixTemplate{
	"error.logged-and-returned":             {Kind: guided, Text: "Pick one logging owner for the error path.\n\nBefore:\nlog.Printf(\"save user: %v\", err)\nreturn err\n\nAfter:\nreturn fmt.Errorf(\"save user %s: %w\", userID, err)"},
	"error.generic-message":                 {Kind: guided, Text: "Replace generic text with safe operation/resource context.\n\nBefore:\nreturn errors.New(\"failed\")\n\nAfter:\nreturn fmt.Errorf(\"parse payment webhook %s: %w\", eventID, err)"},
	"error.wrong-abstraction-level":         {Kind: guided, Text: "Translate low-level infrastructure details at the boundary, preserving the cause internally."},
	"error.inconsistent-wrapping":           {Kind: guided, Text: "Use one error wrapping style throughout the function so callers can rely on a consistent contract."},
	"error.retryable-not-distinguished":     {Kind: guided, Text: "Classify transient/retryable failures separately from permanent failures before retrying."},
	"error.user-message-leaks-internals":    {Kind: guided, Text: "Return a safe user message and record DB/transport/stack details only in internal logs or traces."},
	"error.partial-failure-hidden":          {Kind: guided, Text: "Return aggregate errors or an explicit partial-result contract for skipped failed items."},
	"error.cleanup-error-ignored":           {Kind: guided, Text: "Check close/rollback/delete errors and join them with the primary error when both happen."},
	"error.panic-on-recoverable-path":       {Kind: guided, Text: "Return a typed error for recoverable request, validation, parsing, or I/O failures instead of panicking."},
	"error.exception-used-for-control-flow": {Kind: guided, Text: "Use explicit branch results, optionals, or status values for expected outcomes instead of throw/raise."},
	"error.fallback-hides-corruption":       {Kind: guided, Text: "Surface decode/validation corruption instead of silently returning default data."},
	"defensive.unvalidated-boundary-input":  {Kind: guided, Text: "Validate request, event, payload, or body input before consuming fields or passing it downstream."},
	"defensive.invalid-state-representable": {Kind: guided, Text: "Replace boolean combinations/raw strings with an enum, tagged union, or state machine that encodes valid states."},
	"defensive.null-assumption":             {Kind: guided, Text: "Guard nil/null/None/optional values before dereference, or make the boundary type non-nullable."},
	"defensive.integer-overflow":            {Kind: guided, Text: "Guard count/size arithmetic before multiplication, addition, shifts, or allocation sizing."},
	"defensive.bounds-assumption":           {Kind: guided, Text: "Check length/existence before indexing, or use a safe lookup API."},
	"defensive.unsafe-default":              {Kind: guided, Text: "Make security/safety defaults fail closed and require explicit opt-out for unsafe behavior."},
	"defensive.non-exhaustive-branch":       {Kind: guided, Text: "Add an explicit default/unreachable branch or exhaustive assertion for enum-like state switches."},
	"defensive.unchecked-external-response": {Kind: guided, Text: "Check transport errors and response status/ok before reading or trusting the body."},
	"defensive.missing-schema-validation":   {Kind: guided, Text: "Validate decoded JSON/event payloads with a schema or invariant-checking constructor."},
	"defensive.missing-resource-limit":      {Kind: guided, Text: "Apply explicit maximum bytes, item counts, deadlines, or quotas to boundary reads/uploads."},
	"defensive.invalid-state-transition":    {Kind: guided, Text: "Route state changes through a transition helper that checks current and next states."},
	"defensive.fail-open-authorization":     {Kind: guided, Text: "Fail closed on authorization errors and require an explicit allow decision."},
}
