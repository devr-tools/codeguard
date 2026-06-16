package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualityCheckWarnsForSyncIOInRequestPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.go"), `package sample

import (
	"net/http"
	"os"
)

func handle(w http.ResponseWriter, r *http.Request) {
	_, _ = os.ReadFile("payload.json")
	_, _ = w.Write([]byte("ok"))
}
`)

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-sync-io-request-path"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
	assertFindingRulePresent(t, report, "Code Quality", "quality.sync-io-in-request-path")
}

func TestQualityCheckWarnsForGoroutinesInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "worker.go"), `package sample

func dispatch(items []int) {
	for _, item := range items {
		go func(value int) {
			_ = value
		}(item)
	}
}
`)

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-unbounded-goroutines"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
	assertFindingRulePresent(t, report, "Code Quality", "quality.unbounded-goroutines-in-loop")
}

func qualityPerfConfig(name string, dir string, language string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	return cfg
}

func TestQualityCheckWarnsForGoQueryInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "repo.go"),
		"package repo\n\nimport \"database/sql\"\n\nfunc UpdateAll(db *sql.DB, ids []int) error {\n\tfor _, id := range ids {\n\t\tif _, err := db.Exec(\"UPDATE items SET done = 1 WHERE id = ?\", id); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\treturn nil\n}\n")

	report, err := codeguard.Run(context.Background(), qualityPerfConfig("quality-go-nplusone", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.n-plus-one-query")
}

func TestQualityCheckSkipsGoQueryOutsideLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "repo.go"),
		"package repo\n\nimport \"database/sql\"\n\nfunc UpdateOne(db *sql.DB, id int) error {\n\t_, err := db.Exec(\"UPDATE items SET done = 1 WHERE id = ?\", id)\n\treturn err\n}\n")

	report, err := codeguard.Run(context.Background(), qualityPerfConfig("quality-go-nplusone-neg", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.n-plus-one-query")
}

func TestQualityCheckWarnsForGoAllocInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "report.go"),
		"package report\n\nimport \"fmt\"\n\nfunc Describe(items []string) string {\n\tout := \"\"\n\tfor _, item := range items {\n\t\tout += fmt.Sprintf(\"- %s\\n\", item)\n\t}\n\treturn out\n}\n\nfunc Gather(items []string) []string {\n\tvar values []string\n\tfor _, item := range items {\n\t\tvalues = append(values, item)\n\t}\n\treturn values\n}\n")

	on := true
	cfg := qualityPerfConfig("quality-go-alloc", dir, "go")
	cfg.Checks.QualityRules.DetectPreallocInLoop = &on

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.go.alloc-in-loop")
}

func TestQualityCheckWarnsForAppendWithoutPreallocWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "report.go"),
		"package report\n\nfunc Gather(items []string) []string {\n\tvar values []string\n\tfor _, item := range items {\n\t\tvalues = append(values, item)\n\t}\n\treturn values\n}\n")

	on := true
	cfg := qualityPerfConfig("quality-go-prealloc-on", dir, "go")
	cfg.Checks.QualityRules.DetectPreallocInLoop = &on

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.go.alloc-in-loop")
}

func TestQualityCheckPreallocInLoopDefaultOff(t *testing.T) {
	appendDir := t.TempDir()
	writeFile(t, filepath.Join(appendDir, "report.go"),
		"package report\n\nfunc Gather(items []string) []string {\n\tvar values []string\n\tfor _, item := range items {\n\t\tvalues = append(values, item)\n\t}\n\treturn values\n}\n")

	report, err := codeguard.Run(context.Background(), qualityPerfConfig("quality-go-prealloc-default", appendDir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Code Quality", "quality.go.alloc-in-loop")

	concatDir := t.TempDir()
	writeFile(t, filepath.Join(concatDir, "report.go"),
		"package report\n\nfunc Describe(items []string) string {\n\tout := \"\"\n\tfor _, item := range items {\n\t\tout += \"- \" + item\n\t}\n\treturn out\n}\n")

	report, err = codeguard.Run(context.Background(), qualityPerfConfig("quality-go-concat-default", concatDir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Code Quality", "quality.go.alloc-in-loop")
}

func TestQualityCheckSkipsPreallocatedAppendInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "report.go"),
		"package report\n\nfunc Gather(items []string) []string {\n\tvalues := make([]string, 0, len(items))\n\tfor _, item := range items {\n\t\tvalues = append(values, item)\n\t}\n\treturn values\n}\n")

	on := true
	cfg := qualityPerfConfig("quality-go-alloc-neg", dir, "go")
	cfg.Checks.QualityRules.DetectPreallocInLoop = &on

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.go.alloc-in-loop")
}

func TestQualityCheckAllocInLoopToggleOff(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "report.go"),
		"package report\n\nimport \"fmt\"\n\nfunc Describe(items []string) string {\n\tout := \"\"\n\tfor _, item := range items {\n\t\tout += fmt.Sprintf(\"- %s\\n\", item)\n\t}\n\treturn out\n}\n")

	off := false
	cfg := qualityPerfConfig("quality-go-alloc-off", dir, "go")
	cfg.Checks.QualityRules.DetectAllocInLoop = &off

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.go.alloc-in-loop")
}
