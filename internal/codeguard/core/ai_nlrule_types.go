package core

// AINLRuleCacheVerdict stores the matches one natural-language rule
// evaluation produced for one file, keyed by a content hash of the rule,
// runtime, prompt version, and file contents.
type AINLRuleCacheVerdict struct {
	Matches []AINLRuleCacheMatch `json:"matches,omitempty"`
}

// AINLRuleCacheMatch mirrors one runtime match so cached verdicts can be
// replayed without re-invoking the runtime.
type AINLRuleCacheMatch struct {
	Line      int    `json:"line,omitempty"`
	Column    int    `json:"column,omitempty"`
	Message   string `json:"message,omitempty"`
	Rationale string `json:"rationale,omitempty"`
}
