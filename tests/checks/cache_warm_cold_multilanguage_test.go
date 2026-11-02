package checks_test

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// writeMultiLanguageCacheFixture lays out one source file per language that
// once shared a per-file cache section id ("quality", "performance",
// "reliability", "data", "security"), each written to trip at least one rule.
func writeMultiLanguageCacheFixture(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/fixture\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "service", "service.go"), strings.Join([]string{
		"package service",
		"",
		"import (",
		"\t\"database/sql\"",
		"\t\"net/http\"",
		"\t\"os\"",
		")",
		"",
		"// TODO: remove this",
		"type store interface{ Set(key string, value string) }",
		"",
		"// Fetch relies on exactly once delivery.",
		"func Fetch(db *sql.DB, cache store, ids []string) error {",
		"\tfor _, id := range ids {",
		"\t\tcache.Set(\"user\", id)",
		"\t\tresp, err := http.Get(\"https://example.com/\" + id)",
		"\t\tif err != nil {",
		"\t\t\tpanic(err)",
		"\t\t}",
		"\t\t_ = resp",
		"\t\trows, _ := db.Query(\"SELECT * FROM users WHERE id = \" + id)",
		"\t\t_ = rows",
		"\t}",
		"\t_ = os.Getenv(\"PASSWORD\")",
		"\treturn nil",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "app", "worker.py"), strings.Join([]string{
		"import requests",
		"import pickle",
		"",
		"def fetch(ids, cursor):",
		"    # TODO: remove this",
		"    for i in ids:",
		"        r = requests.get('https://example.com/' + i)",
		"        cursor.execute('SELECT * FROM users WHERE id = ' + i)",
		"        cache.set('user', i)  # exactly once delivery",
		"        try:",
		"            pickle.loads(r.content)",
		"        except:",
		"            pass",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "native", "engine.cpp"), strings.Join([]string{
		"#include <cstdlib>",
		"#include <cstring>",
		"",
		"// TODO: remove this",
		"int run(const char* input, int count) {",
		"  char buffer[16];",
		"  strcpy(buffer, input);",
		"  int* data = new int[count];",
		"  cache.set(\"user\", input);",
		"  for (int i = 0; i < count; i++) {",
		"    system(input);",
		"  }",
		"  return data[0];",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "rust", "lib.rs"), strings.Join([]string{
		"// TODO: remove this",
		"pub fn parse(input: &str) -> i32 {",
		"    let value: i32 = input.parse().unwrap();",
		"    let other = input.to_string().clone();",
		"    unsafe { std::ptr::null::<i32>().read() };",
		"    value + other.len() as i32",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "java", "Service.java"), strings.Join([]string{
		"public class Service {",
		"  // TODO: remove this",
		"  public void run(String input) {",
		"    try { Runtime.getRuntime().exec(input); } catch (Exception e) {}",
		"  }",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "csharp", "Service.cs"), strings.Join([]string{
		"public class Service {",
		"  // TODO: remove this",
		"  public void Run(string input) {",
		"    try { System.Diagnostics.Process.Start(input); } catch (System.Exception) {}",
		"  }",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "ruby", "service.rb"), strings.Join([]string{
		"# TODO: remove this",
		"def run(input)",
		"  eval(input)",
		"rescue",
		"  nil",
		"end",
	}, "\n"))
}

// multiLanguageCacheConfig scans the fixture once per language, every target
// rooted at the same directory, so the per-language passes of a section visit
// one target path and share everything in their cache key except the section
// id and the file.
func multiLanguageCacheConfig(dir string, cacheEnabled bool, cachePath string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = "multi-language-cache"
	cfg.Targets = nil
	for _, language := range []string{"go", "python", "cpp", "rust", "java", "csharp", "ruby"} {
		cfg.Targets = append(cfg.Targets, codeguard.TargetConfig{Name: language, Path: dir, Language: language})
	}
	on := true
	off := false
	cfg.Checks.Quality = true
	cfg.Checks.Design = true
	cfg.Checks.Security = true
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Performance = &on
	cfg.Checks.Reliability = &on
	cfg.Checks.Data = &on
	cfg.Checks.Context = &off
	// Low thresholds make every language's quality and design pass report
	// something, so a pass replaying another's entries cannot go unnoticed.
	cfg.Checks.QualityRules.MaxFileLines = 3
	cfg.Checks.QualityRules.MaxFunctionLines = 2
	cfg.Checks.QualityRules.MaxParameters = 1
	cfg.Checks.DesignRules.MaxDeclsPerFile = 1
	cfg.Cache.Enabled = &cacheEnabled
	cfg.Cache.Path = cachePath
	return cfg
}

func findingsBySection(report codeguard.Report) map[string][]codeguard.Finding {
	out := map[string][]codeguard.Finding{}
	for _, section := range report.Sections {
		out[section.Name] = section.Findings
	}
	return out
}

// TestWarmCacheMatchesColdScanAcrossLanguages guards the per-file findings
// cache against passes replaying each other's entries: a cold cached scan and
// a warm one (every file served from cache) must both report exactly what an
// uncached scan reports, across languages whose passes once shared an id.
func TestWarmCacheMatchesColdScanAcrossLanguages(t *testing.T) {
	dir := t.TempDir()
	writeMultiLanguageCacheFixture(t, dir)
	cachePath := filepath.Join(t.TempDir(), "scan-cache.json")

	run := func(cfg codeguard.Config) map[string][]codeguard.Finding {
		t.Helper()
		report, err := codeguard.Run(context.Background(), cfg)
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return findingsBySection(report)
	}
	uncached := run(multiLanguageCacheConfig(dir, false, ""))
	cold := run(multiLanguageCacheConfig(dir, true, cachePath))
	warm := run(multiLanguageCacheConfig(dir, true, cachePath))

	assertMultiLanguageCacheCoverage(t, uncached)
	if !reflect.DeepEqual(cold, uncached) {
		t.Fatalf("cold cached scan differs from uncached scan:\n cold: %+v\nuncached: %+v", cold, uncached)
	}
	if !reflect.DeepEqual(warm, uncached) {
		t.Fatalf("warm cached scan differs from uncached scan:\n warm: %+v\nuncached: %+v", warm, uncached)
	}
}

// multiLanguageCacheCoverage lists, per section, the file extensions whose
// language pass must report at least one finding on the fixture. Without a
// finding, a pass replaying another pass's (empty) entries would go unseen.
var multiLanguageCacheCoverage = map[string][]string{
	"Code Quality":     {".go", ".py", ".cpp", ".rs", ".java", ".cs", ".rb"},
	"Performance":      {".go", ".py"},
	"Reliability":      {".go", ".py"},
	"Data Correctness": {".go", ".py", ".cpp"},
	"Security":         {".py", ".cpp", ".java", ".cs", ".rb"},
	"Design Patterns":  {".go"},
}

func assertMultiLanguageCacheCoverage(t *testing.T, sections map[string][]codeguard.Finding) {
	t.Helper()
	for section, extensions := range multiLanguageCacheCoverage {
		seen := map[string]bool{}
		for _, finding := range sections[section] {
			seen[filepath.Ext(finding.Path)] = true
		}
		for _, ext := range extensions {
			if !seen[ext] {
				t.Errorf("fixture produced no %s finding for a %s file; it no longer exercises that language pass", section, ext)
			}
		}
	}
}
