package support_test

import (
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// The confidence policy is applied when a section is finalized, which is after
// the per-file findings cache. Changing a threshold must therefore leave every
// section config fingerprint untouched: a threshold change re-renders a scan,
// it never re-runs one.
func TestConfidencePolicyDoesNotInvalidateCachedFindings(t *testing.T) {
	catalog := map[string]core.RuleMetadata{
		"security.demo": {ID: "security.demo", Section: "Security", DefaultLevel: "fail"},
	}
	base := core.Config{Name: "cache"}
	baseline := runnersupport.SectionConfigHashes(base, catalog)

	withPolicy := base
	withPolicy.Checks.MinConfidence = core.ConfidencePolicyConfig{
		Default:  core.ConfidenceHigh,
		Sections: map[string]string{"security": core.ConfidenceHigh},
	}
	withPolicy.Checks.ConfidenceDemotion = true
	changed := runnersupport.SectionConfigHashes(withPolicy, catalog)

	if len(baseline) != len(changed) {
		t.Fatalf("family count changed: %d -> %d", len(baseline), len(changed))
	}
	for family, want := range baseline {
		if got := changed[family]; got != want {
			t.Fatalf("config fingerprint for family %q changed when the confidence policy changed; cached findings would be discarded", family)
		}
	}
}

// A setting that does affect per-file findings must still change the
// fingerprint, so the test above cannot pass by fingerprinting nothing.
func TestSectionConfigHashesStillTrackFindingRelevantSettings(t *testing.T) {
	catalog := map[string]core.RuleMetadata{
		"security.demo": {ID: "security.demo", Section: "Security", DefaultLevel: "fail"},
	}
	base := core.Config{Name: "cache"}
	baseline := runnersupport.SectionConfigHashes(base, catalog)

	changed := base
	changed.Parsers.TreeSitter = core.TreeSitterModeAuto
	updated := runnersupport.SectionConfigHashes(changed, catalog)

	if updated["security"] == baseline["security"] {
		t.Fatal("changing parsers.treesitter must change the security fingerprint")
	}
}
