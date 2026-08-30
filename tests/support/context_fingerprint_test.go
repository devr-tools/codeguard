package support_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

type fingerprintCaseExpectation struct {
	wantFallback bool
	wantSame     bool
}

const contextFingerprintBase = "alpha one\nbeta two\ngamma three\ndelta four\nepsilon five\n"

// contextFingerprintFinding builds a finding through the real NewFinding path
// against a throwaway target containing a single file, so the test exercises
// exactly the fingerprinting a scan would perform.
func contextFingerprintFinding(t *testing.T, content string, rel string, line int) core.Finding {
	t.Helper()
	dir := t.TempDir()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	sc := runnersupport.Context{Cfg: core.Config{Targets: []core.TargetConfig{{Name: "repo", Path: dir}}}}
	return runnersupport.NewFinding(sc, runnersupport.FindingInput{
		RuleID:  "test.rule",
		Level:   "warn",
		Path:    rel,
		Line:    line,
		Message: "fixture finding",
	})
}

// TestContextFingerprintNormalization locks in the context-fingerprint
// contract: it hashes the rule, path, and the whitespace-normalized ±2-line
// window around the finding, so pure line shifts and whitespace churn leave it
// unchanged, while edits inside the window (or an unusable location, which
// falls back to the legacy fingerprint) change it.
func TestContextFingerprintNormalization(t *testing.T) {
	base := contextFingerprintFinding(t, contextFingerprintBase, "src/app.go", 3)
	if base.ContextFingerprint == "" || base.ContextFingerprint == base.Fingerprint {
		t.Fatalf("base finding must have a distinct context fingerprint, got %q (exact %q)", base.ContextFingerprint, base.Fingerprint)
	}

	cases := []struct {
		name         string
		content      string
		line         int
		wantSame     bool
		wantFallback bool
	}{
		{
			name:     "identical content",
			content:  contextFingerprintBase,
			line:     3,
			wantSame: true,
		},
		{
			name:     "whitespace runs collapse and trim",
			content:  "  alpha \t one\nbeta    two\n\tgamma\tthree  \ndelta  four\nepsilon five\n",
			line:     3,
			wantSame: true,
		},
		{
			name:     "lines inserted above the window shift the finding",
			content:  "pad one\npad two\n" + contextFingerprintBase,
			line:     5,
			wantSame: true,
		},
		{
			name:     "lines appended below the window",
			content:  contextFingerprintBase + "zeta six\neta seven\n",
			line:     3,
			wantSame: true,
		},
		{
			name:     "edit inside the context window",
			content:  "alpha one\nbeta two\ngamma three\ndelta CHANGED\nepsilon five\n",
			line:     3,
			wantSame: false,
		},
		{
			name:     "finding at start of file clamps the window",
			content:  contextFingerprintBase,
			line:     1,
			wantSame: false,
		},
		{
			name:         "line zero falls back to exact",
			content:      contextFingerprintBase,
			line:         0,
			wantFallback: true,
		},
		{
			name:         "line past end of file falls back to exact",
			content:      contextFingerprintBase,
			line:         99,
			wantFallback: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertContextFingerprintCase(t, base, tc.content, tc.line, fingerprintCaseExpectation{
				wantFallback: tc.wantFallback,
				wantSame:     tc.wantSame,
			})
		})
	}
}

func assertContextFingerprintCase(t *testing.T, base core.Finding, content string, line int, want fingerprintCaseExpectation) {
	t.Helper()
	finding := contextFingerprintFinding(t, content, "src/app.go", line)
	if want.wantFallback {
		if finding.ContextFingerprint != finding.Fingerprint {
			t.Fatalf("expected fallback to exact fingerprint, got context %q exact %q", finding.ContextFingerprint, finding.Fingerprint)
		}
		return
	}
	if finding.ContextFingerprint == finding.Fingerprint {
		t.Fatalf("expected a real context fingerprint, got exact fallback %q", finding.Fingerprint)
	}
	same := finding.ContextFingerprint == base.ContextFingerprint
	if same != want.wantSame {
		t.Fatalf("context fingerprint match = %v, want %v (context %q, base %q)", same, want.wantSame, finding.ContextFingerprint, base.ContextFingerprint)
	}
}

// A finding whose file cannot be resolved under any target must fall back to
// the legacy fingerprint rather than fingerprinting nothing.
func TestContextFingerprintUnreadableFileFallsBack(t *testing.T) {
	sc := runnersupport.Context{Cfg: core.Config{Targets: []core.TargetConfig{{Name: "repo", Path: t.TempDir()}}}}
	finding := runnersupport.NewFinding(sc, runnersupport.FindingInput{
		RuleID:  "test.rule",
		Level:   "warn",
		Path:    "missing/file.go",
		Line:    3,
		Message: "fixture finding",
	})
	if finding.ContextFingerprint != finding.Fingerprint {
		t.Fatalf("expected fallback to exact fingerprint, got context %q exact %q", finding.ContextFingerprint, finding.Fingerprint)
	}
}

// A production change that folds Message, Confidence, or Metadata back into
// fingerprint construction must fail this test: those fields are diagnostic
// evidence, not finding identity.
func TestFindingFingerprintsIgnoreDiagnosticProseAndMetadata(t *testing.T) {
	dir := t.TempDir()
	rel := "src/app.go"
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(contextFingerprintBase), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	sc := runnersupport.Context{Cfg: core.Config{Targets: []core.TargetConfig{{Name: "repo", Path: dir}}}}

	before := runnersupport.NewFinding(sc, runnersupport.FindingInput{
		RuleID:     "test.rule",
		Level:      "warn",
		Path:       rel,
		Line:       3,
		Message:    "old diagnostic prose",
		Confidence: "low",
		Metadata:   map[string]string{"mutation_target": "global", "origin": "shared"},
	})
	after := runnersupport.NewFinding(sc, runnersupport.FindingInput{
		RuleID:     "test.rule",
		Level:      "warn",
		Path:       rel,
		Line:       3,
		Message:    "new diagnostic prose",
		Confidence: "high",
		Metadata:   map[string]string{"mutation_target": "argument", "origin": "caller_owned"},
	})

	if before.Fingerprint != after.Fingerprint {
		t.Errorf("exact fingerprint changed with diagnostic evidence: %q -> %q", before.Fingerprint, after.Fingerprint)
	}
	if before.ContextFingerprint != after.ContextFingerprint {
		t.Errorf("context fingerprint changed with diagnostic evidence: %q -> %q", before.ContextFingerprint, after.ContextFingerprint)
	}
	if before.ContentFingerprint != after.ContentFingerprint {
		t.Errorf("content fingerprint changed with diagnostic evidence: %q -> %q", before.ContentFingerprint, after.ContentFingerprint)
	}
}

func TestFindingExactFingerprintIgnoresMessageWithoutSourceContext(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "short.go"), []byte("package sample\n"), 0o644); err != nil {
		t.Fatalf("write short source: %v", err)
	}
	sc := runnersupport.Context{Cfg: core.Config{Targets: []core.TargetConfig{{Name: "repo", Path: dir}}}}
	cases := []struct {
		name string
		path string
		line int
	}{
		{name: "pathless", line: 0},
		{name: "unreadable", path: "missing.go", line: 3},
		{name: "invalid line", path: "short.go", line: 99},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			first := runnersupport.NewFinding(sc, runnersupport.FindingInput{RuleID: "test.rule", Path: tc.path, Line: tc.line, Message: "first prose"})
			second := runnersupport.NewFinding(sc, runnersupport.FindingInput{RuleID: "test.rule", Path: tc.path, Line: tc.line, Message: "second prose"})
			if first.Fingerprint != second.Fingerprint || first.ContextFingerprint != second.ContextFingerprint || first.ContentFingerprint != second.ContentFingerprint {
				t.Fatalf("message changed source-unavailable identity: %#v -> %#v", first, second)
			}
		})
	}
}
