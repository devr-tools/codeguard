package quality

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestGoPerformanceFindingsDetectSyncIOInHTTPHandler(t *testing.T) {
	findings := performanceFindingsForSource(t, "handler.go", `package sample

import (
	httpPkg "net/http"
	"os"
)

func handle(w httpPkg.ResponseWriter, r *httpPkg.Request) {
	_, _ = os.ReadFile("config.json")
}
`)

	assertHasRule(t, findings, "quality.sync-io-in-request-path")
}

func TestGoPerformanceFindingsIgnoreSyncIOOutsideHTTPHandler(t *testing.T) {
	findings := performanceFindingsForSource(t, "worker.go", `package sample

import "os"

func load() {
	_, _ = os.ReadFile("config.json")
}
`)

	if len(findings) != 0 {
		t.Fatalf("expected no performance findings, got %#v", findings)
	}
}

func TestGoPerformanceFindingsDetectGoroutineInsideLoop(t *testing.T) {
	findings := performanceFindingsForSource(t, "worker.go", `package sample

func dispatch(items []int) {
	for _, item := range items {
		go func(value int) {
			_ = value
		}(item)
	}
}
`)

	assertHasRule(t, findings, "quality.unbounded-goroutines-in-loop")
}

func performanceFindingsForSource(t *testing.T, name string, source string) []core.Finding {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, name, source, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	env := support.Context{
		NewFinding: func(input support.FindingInput) core.Finding {
			return core.Finding{
				RuleID:  input.RuleID,
				Level:   input.Level,
				Message: input.Message,
				Path:    input.Path,
				Line:    input.Line,
				Column:  input.Column,
			}
		},
	}

	return goPerformanceFindings(env, name, fset, parsed)
}
