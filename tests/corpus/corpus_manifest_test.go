package corpus_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

const (
	gapFalseNegative = "false-negative"
	gapFalsePositive = "false-positive"
)

// manifest is the parsed expectations file. Rules is the closed set of rule
// IDs the corpus measures: findings for any other rule are ignored so the
// harness stays deterministic across optional analysis engines.
type manifest struct {
	Version int            `yaml:"version" json:"version"`
	Rules   []string       `yaml:"rules" json:"rules"`
	Groups  []fixtureGroup `yaml:"groups" json:"groups"`
}

// fixtureGroup is one scan: a fixture subtree scanned as a single target with
// the given language. Entropy opts the scan into the high-entropy heuristic;
// TreeSitter opts it into parsers.treesitter "auto" (the tree-sitter parsing
// path with regex fallback).
type fixtureGroup struct {
	Name       string        `yaml:"name" json:"name"`
	Language   string        `yaml:"language" json:"language"`
	Root       string        `yaml:"root" json:"root"`
	Entropy    bool          `yaml:"entropy" json:"entropy"`
	TreeSitter bool          `yaml:"treesitter" json:"treesitter"`
	Files      []fixtureFile `yaml:"files" json:"files"`
}

// fixtureFile maps one fixture to the findings it must (and must not)
// produce. A file with no MustFire and no KnownGaps must stay silent.
type fixtureFile struct {
	Path      string        `yaml:"path" json:"path"`
	MustFire  []expectation `yaml:"must_fire" json:"must_fire"`
	KnownGaps []knownGap    `yaml:"known_gaps" json:"known_gaps"`
}

// expectation is a required finding. Line 0 means "any line in the file".
type expectation struct {
	Rule string `yaml:"rule" json:"rule"`
	Line int    `yaml:"line" json:"line"`
}

// knownGap is a documented detector deficiency. The harness asserts the gap
// still exists so that a fixed gap fails loudly and gets promoted.
type knownGap struct {
	Rule   string `yaml:"rule" json:"rule"`
	Type   string `yaml:"type" json:"type"`
	Line   int    `yaml:"line" json:"line"`
	Reason string `yaml:"reason" json:"reason"`
}

// loadManifest reads expectations.yaml (or expectations.json; JSON is a
// subset of YAML so one decoder covers both) and validates it.
func loadManifest(t *testing.T) manifest {
	t.Helper()
	data, err := os.ReadFile("expectations.yaml")
	if os.IsNotExist(err) {
		data, err = os.ReadFile("expectations.json")
	}
	if err != nil {
		t.Fatalf("read expectations manifest: %v", err)
	}
	var man manifest
	if err := yaml.Unmarshal(data, &man); err != nil {
		t.Fatalf("parse expectations manifest: %v", err)
	}
	validateManifest(t, man)
	return man
}

func validateManifest(t *testing.T, man manifest) {
	t.Helper()
	if len(man.Rules) == 0 || len(man.Groups) == 0 {
		t.Fatal("expectations manifest must declare rules and groups")
	}
	inScope := ruleSet(man)
	for _, ruleID := range man.Rules {
		if _, ok := codeguard.ExplainRule(ruleID); !ok {
			t.Errorf("manifest rule %q is not in the codeguard rule catalog", ruleID)
		}
	}
	for _, group := range man.Groups {
		validateGroup(t, group, inScope)
	}
}

func validateGroup(t *testing.T, group fixtureGroup, inScope map[string]bool) {
	t.Helper()
	for _, file := range group.Files {
		for _, exp := range file.MustFire {
			if !inScope[exp.Rule] {
				t.Errorf("%s/%s: must_fire rule %q is not in the manifest rules list", group.Name, file.Path, exp.Rule)
			}
		}
		for _, gap := range file.KnownGaps {
			if !inScope[gap.Rule] {
				t.Errorf("%s/%s: known_gaps rule %q is not in the manifest rules list", group.Name, file.Path, gap.Rule)
			}
			if gap.Type != gapFalseNegative && gap.Type != gapFalsePositive {
				t.Errorf("%s/%s: known gap type %q must be %s or %s", group.Name, file.Path, gap.Type, gapFalseNegative, gapFalsePositive)
			}
			if gap.Reason == "" {
				t.Errorf("%s/%s: known gap for %s needs a reason", group.Name, file.Path, gap.Rule)
			}
		}
	}
}

func ruleSet(man manifest) map[string]bool {
	set := make(map[string]bool, len(man.Rules))
	for _, ruleID := range man.Rules {
		set[ruleID] = true
	}
	return set
}
