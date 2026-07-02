package security_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/report"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestEverySecurityRuleHasOWASPCategory(t *testing.T) {
	valid := make(map[core.OWASPCategory]bool, len(core.OWASPTop10))
	for _, category := range core.OWASPTop10 {
		valid[category] = true
	}

	// security.command-check is intentionally unmapped: its category depends on
	// the external command it wraps.
	const exempt = "security.command-check"

	for _, rule := range service.Rules() {
		if !strings.HasPrefix(rule.ID, "security.") || rule.ID == exempt {
			continue
		}
		if rule.OWASPCategory == "" {
			t.Errorf("security rule %q has no OWASP category", rule.ID)
			continue
		}
		if !valid[rule.OWASPCategory] {
			t.Errorf("security rule %q has unknown OWASP category %q", rule.ID, rule.OWASPCategory)
		}
	}
}

func TestOWASPCoverageReportsGaps(t *testing.T) {
	coverage := service.OWASPCoverage()
	if len(coverage) != len(core.OWASPTop10) {
		t.Fatalf("coverage entries = %d, want %d", len(coverage), len(core.OWASPTop10))
	}
	byCode := map[string]service.OWASPCoverageEntry{}
	for _, entry := range coverage {
		byCode[entry.Code] = entry
	}
	if !byCode["A03:2021"].Covered || len(byCode["A03:2021"].RuleIDs) == 0 {
		t.Error("expected A03 Injection to be covered")
	}
	if !byCode["A10:2021"].Covered {
		t.Error("expected A10 SSRF to be covered by the SSRF taint rules")
	}
	// A04 Insecure Design remains an intentional gap: it is a design-level risk
	// that static heuristics cannot reliably detect. The report must surface it
	// as a gap rather than imply coverage.
	if byCode["A04:2021"].Covered {
		t.Error("expected A04 Insecure Design to be reported as a coverage gap")
	}
	if !byCode["A09:2021"].Covered {
		t.Error("expected A09 Logging/Monitoring to be covered by the log-exposure rules")
	}
}

func TestOWASPA09CoverageListsLogRules(t *testing.T) {
	for _, entry := range service.OWASPCoverage() {
		if entry.Code != "A09:2021" {
			continue
		}
		got := strings.Join(entry.RuleIDs, ",")
		for _, want := range []string{"security.log-secret-exposure", "security.unsanitized-error-response"} {
			if !strings.Contains(got, want) {
				t.Errorf("A09 rules = %q, missing %s", got, want)
			}
		}
		return
	}
	t.Fatal("A09:2021 coverage entry not found")
}

func TestSARIFOutputCarriesOWASPTag(t *testing.T) {
	rep := core.Report{
		Sections: []core.SectionResult{{
			Name: "Security",
			Findings: []core.Finding{{
				RuleID:  "security.taint.go",
				Level:   "fail",
				Message: "tainted input reaches exec.Command",
				Path:    "main.go",
				Line:    10,
			}},
		}},
	}
	var buf bytes.Buffer
	if err := report.Write(&buf, rep, "sarif"); err != nil {
		t.Fatalf("write sarif: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "A03:2021") {
		t.Fatalf("expected OWASP category in SARIF output, got: %s", out)
	}
	if !strings.Contains(out, "OWASP:A03:2021") {
		t.Fatalf("expected OWASP tag in SARIF output, got: %s", out)
	}
	for _, want := range []string{"taxonomies", "OWASP Top 10", "relationships", "superset"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected SARIF to contain %q, got: %s", want, out)
		}
	}
}
