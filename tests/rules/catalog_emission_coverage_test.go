// Package rules_test asserts catalog invariants that are otherwise unenforced.
package rules_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/rules"
)

// ruleIDLiteral matches a quoted rule id in check source. Rule ids are
// lowercase, dot-separated, and always carry at least one dot.
var ruleIDLiteral = regexp.MustCompile(`"([a-z][a-z0-9]*(?:\.[a-z0-9][a-z0-9_-]*)+)"`)

// diagnosticIDLiteral finds ids attached to a core.Diagnostic. Diagnostics are
// deliberately separate from findings - they describe scanner operation, are
// never eligible for baselines, and correctly have no catalog entry - so they
// must not be held to the catalog invariant. Detected structurally rather than
// by an allowlist, so a new diagnostic does not need this test updated.
var diagnosticIDLiteral = regexp.MustCompile(`core\.Diagnostic\{(?:[^{}]|\n)*?ID:\s*"([^"]+)"`)

// knownSectionPrefixes bounds the scan to strings that are actually rule ids.
// A literal like "config.yaml" matches the shape above but is not a rule.
var knownSectionPrefixes = []string{
	"quality.", "performance.", "design.", "security.", "prompts.", "ci.",
	"supply_chain.", "context.", "contracts.", "data.", "delivery.",
	"observability.", "operations.", "reliability.", "change.", "refactor.",
	"defensive.", "error.", "function.", "naming.", "smell.", "maintainability.",
}

// TestEveryEmittedRuleIDIsInTheCatalog is the invariant that was missing when
// five performance rules shipped emitting findings with no catalog entry: the
// scanner printed them, then `codeguard explain` answered "unknown rule" and
// they carried no metadata, fix template, OWASP mapping, or SARIF descriptor.
// Catalog-to-implementation was already enforced; this is the reverse.
func TestEveryEmittedRuleIDIsInTheCatalog(t *testing.T) {
	catalog := rules.Catalog()
	root := filepath.Join("..", "..", "internal", "codeguard", "checks")

	emitted := map[string][]string{}
	diagnostics := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path) //nolint:gosec // test walks a fixed in-repo tree
		if readErr != nil {
			return readErr
		}
		source := string(data)
		for _, match := range diagnosticIDLiteral.FindAllStringSubmatch(source, -1) {
			diagnostics[match[1]] = true
		}
		for _, match := range ruleIDLiteral.FindAllStringSubmatch(source, -1) {
			id := match[1]
			if !hasKnownSectionPrefix(id) {
				continue
			}
			// A trailing separator means the literal is a prefix fragment used
			// with strings.HasPrefix, not a rule id in its own right.
			if strings.HasSuffix(id, "-") || strings.HasSuffix(id, ".") {
				continue
			}
			emitted[id] = append(emitted[id], path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk checks tree: %v", err)
	}
	if len(emitted) == 0 {
		t.Fatal("found no rule id literals under checks/; the scan is not working")
	}

	missing := make([]string, 0)
	for id := range emitted {
		if diagnostics[id] {
			continue
		}
		if _, ok := catalog[id]; !ok {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	for _, id := range missing {
		t.Errorf("rule id %q is referenced in %s but has no entry in rules.Catalog(); `codeguard explain %s` would fail for a finding the scanner just printed",
			id, strings.Join(dedupe(emitted[id]), ", "), id)
	}
}

func hasKnownSectionPrefix(id string) bool {
	for _, prefix := range knownSectionPrefixes {
		if strings.HasPrefix(id, prefix) {
			return true
		}
	}
	return false
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
