package core

// InlineSuppressionsConfig tunes the hygiene diagnostics a scan reports about
// inline codeguard:ignore directives. Both settings are applied when the
// report is assembled, after every section has run, so they never change a
// finding and never invalidate the per-file cache.
type InlineSuppressionsConfig struct {
	// ReportUnused reports a suppression.unused-directive warning for a
	// directive that covered no finding in a full scan. Nil (the default) and
	// true turn it on; false turns it off.
	ReportUnused *bool `json:"report_unused,omitempty" yaml:"report_unused,omitempty"`
	// RequireReason reports a suppression.missing-reason warning for a
	// directive with no explanation after the rule id (and optional until
	// date). Nil (the default) and true turn it on; false turns it off.
	RequireReason *bool `json:"require_reason,omitempty" yaml:"require_reason,omitempty"`
}

// ReportUnusedEnabled reports whether unused directives are diagnosed; it is
// on unless report_unused is explicitly false.
func (c InlineSuppressionsConfig) ReportUnusedEnabled() bool {
	return c.ReportUnused == nil || *c.ReportUnused
}

// RequireReasonEnabled reports whether directives without a reason are
// diagnosed; it is on unless require_reason is explicitly false.
func (c InlineSuppressionsConfig) RequireReasonEnabled() bool {
	return c.RequireReason == nil || *c.RequireReason
}
