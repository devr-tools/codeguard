package cli

import (
	"encoding/json"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestAuditPreservesEveryEntryMatchingAnySupportedFingerprint(t *testing.T) {
	file := core.BaselineFile{Entries: []core.BaselineEntry{
		{Fingerprint: "exact", ContextFingerprint: "ctx-a", RuleID: "security.secret", Path: "services/a.go"},
		{Fingerprint: "old-line", ContextFingerprint: "shared-context", RuleID: "defensive.input", Path: "domains/a.go"},
		{Fingerprint: "old-twin", ContextFingerprint: "shared-context", RuleID: "defensive.input", Path: "domains/a.go"},
		{Fingerprint: "old-path", ContentFingerprint: "shared-content", RuleID: "error.context", Path: "platform/old.go"},
		{Fingerprint: "stale", RuleID: "quality.dead-code", Path: "legacy/a.go"},
		{Fingerprint: "", RuleID: "quality.invalid", Path: "bad.go"},
	}}
	findings := []core.Finding{
		{Fingerprint: "exact", ContextFingerprint: "different", RuleID: "security.secret", Path: "services/a.go", Line: 2},
		{Fingerprint: "new-line-1", ContextFingerprint: "shared-context", RuleID: "defensive.input", Path: "domains/a.go", Line: 10},
		{Fingerprint: "new-line-2", ContextFingerprint: "shared-context", RuleID: "defensive.input", Path: "domains/a.go", Line: 20},
		{Fingerprint: "new-path", ContentFingerprint: "shared-content", RuleID: "error.context", Path: "platform/new.go", Line: 4},
	}

	result := Audit(file, findings, Options{SampleLimit: 2})
	if result.Counts.ActiveExact != 1 || result.Counts.ActiveContext != 2 || result.Counts.ActiveContent != 1 || result.Counts.Stale != 1 || result.Counts.Invalid != 1 {
		t.Fatalf("counts = %#v", result.Counts)
	}
	if len(result.Collisions) == 0 {
		t.Fatal("expected shared-context collision to be reported")
	}
	if got := len(result.ActiveEntries()); got != 4 {
		t.Fatalf("active entries = %d, want 4", got)
	}
	if got := len(result.PrunableEntries()); got != 1 || got == len(result.ActiveEntries()) {
		t.Fatalf("prunable entries = %d, want only stale entry", got)
	}
}

func TestAuditOutputIsDeterministicAndHighRiskFirst(t *testing.T) {
	file := core.BaselineFile{Entries: []core.BaselineEntry{
		{Fingerprint: "q", RuleID: "quality.mutable-global", Path: "platform/z.go", Message: "quality"},
		{Fingerprint: "e", RuleID: "error.lost-context", Path: "services/b.go", Message: "error"},
		{Fingerprint: "s", RuleID: "security.secret", Path: "domains/a.go", Message: "security"},
	}}
	findings := []core.Finding{
		{Fingerprint: "q", RuleID: "quality.mutable-global", Path: "platform/z.go", Line: 3, Confidence: "low", Message: "quality"},
		{Fingerprint: "e", RuleID: "error.lost-context", Path: "services/b.go", Line: 2, Confidence: "medium", Message: "error"},
		{Fingerprint: "s", RuleID: "security.secret", Path: "domains/a.go", Line: 1, Confidence: "high", Message: "security"},
	}

	first := Audit(file, findings, Options{SampleLimit: 1})
	second := Audit(core.BaselineFile{Entries: []core.BaselineEntry{file.Entries[2], file.Entries[0], file.Entries[1]}}, []core.Finding{findings[1], findings[2], findings[0]}, Options{SampleLimit: 1})
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatalf("audit output changed with input order:\n%s\n%s", a, b)
	}
	if len(first.ByRisk) < 3 || first.ByRisk[0].Name != "security" || first.ByRisk[1].Name != "error-handling" || first.ByRisk[2].Name != "structural-quality" {
		t.Fatalf("risk ordering = %#v", first.ByRisk)
	}
	if len(first.ByOwner) != 3 || first.ByOwner[0].Name != "domains" {
		t.Fatalf("owner grouping = %#v", first.ByOwner)
	}
	if len(first.ByRule[0].Confidence) == 0 || len(first.ByRule[0].Languages) == 0 {
		t.Fatalf("rule distributions missing: %#v", first.ByRule[0])
	}
}
