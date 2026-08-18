package change

import (
	"fmt"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestAbstractionUseCountsScansCorpusForAllNames(t *testing.T) {
	env := support.Context{
		Config: core.Config{Targets: []core.TargetConfig{{Name: "repo"}}},
		VisitTargetFiles: func(_ core.TargetConfig, _ func(string) bool, visit func(string, []byte)) {
			visit("service.go", []byte("First Firstish First\nSecond Second Second Second\n"))
		},
	}

	counts := abstractionUseCounts(env, []abstractionDecl{{name: "First"}, {name: "Second"}})
	if counts["First"] != 2 {
		t.Fatalf("First count = %d, want 2", counts["First"])
	}
	if counts["Second"] != 3 {
		t.Fatalf("Second count = %d, want capped count 3", counts["Second"])
	}
}

func TestOneUseAbstractionFindingsCapsUntrustedDeclarations(t *testing.T) {
	var source strings.Builder
	for i := 0; i < maxOneUseAbstractionDecls+1; i++ {
		fmt.Fprintf(&source, "type Boundary%d interface {}\n", i)
	}
	env := support.Context{
		NewFinding: func(input support.FindingInput) core.Finding {
			return core.Finding{RuleID: input.RuleID}
		},
	}

	findings := oneUseAbstractionFindings(env, []changedFileContent{{
		path:   "service.go",
		head:   []byte(source.String()),
		ranges: core.ChangedLineRanges{AllChanged: true},
	}})
	if len(findings) != maxOneUseAbstractionDecls {
		t.Fatalf("finding count = %d, want cap %d", len(findings), maxOneUseAbstractionDecls)
	}
}
