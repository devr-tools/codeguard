package support

import (
	"strings"
	"testing"

	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

func TestParseScriptEnforcesAggregateBudget(t *testing.T) {
	corpus := newFileCorpus()
	data := []byte("const value = 1;\n")
	corpus.scriptBytes = maxTreeSitterScanBytes - len(data)

	if _, err := corpus.parseScript("first.ts", data, checkSupport.ScriptLangTypeScript); err != nil {
		t.Fatalf("parse at budget boundary: %v", err)
	}
	if _, err := corpus.parseScript("second.ts", data, checkSupport.ScriptLangTypeScript); err == nil ||
		!strings.Contains(err.Error(), "scan budget") {
		t.Fatalf("parse beyond budget error = %v, want scan budget error", err)
	}
	if got := len(corpus.scripts); got != 1 {
		t.Fatalf("cached script count = %d, want 1", got)
	}
}

func TestParseScriptEnforcesFileCountBudget(t *testing.T) {
	corpus := newFileCorpus()
	corpus.scriptCount = maxTreeSitterScanFiles

	if _, err := corpus.parseScript("extra.ts", nil, checkSupport.ScriptLangTypeScript); err == nil ||
		!strings.Contains(err.Error(), "scan budget") {
		t.Fatalf("parse beyond file budget error = %v, want scan budget error", err)
	}
	if got := len(corpus.scripts); got != 0 {
		t.Fatalf("cached script count = %d, want 0", got)
	}
}
