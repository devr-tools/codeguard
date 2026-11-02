package support_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnerchecks "github.com/devr-tools/codeguard/internal/codeguard/runner/checks"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// clikeCacheSources exercise the parse features that feed findings: nested
// closures capturing outer bindings, mutation of captured and global state,
// long and branchy functions, many parameters, classes, and C++ members.
var clikeCacheSources = map[string]string{
	"src/service.ts": `import { Repository } from "./repo";

let requestCount = 0;
const cache: Record<string, number> = {};

export function handle(data: any, repo: Repository, user: string, flag: boolean, retries: number, mode: string) {
  let total = 0;
  const items = repo.list(user);
  for (const item of items) {
    if (flag && item.enabled) {
      total += item.value;
    } else if (mode === "strict") {
      total -= 1;
    } else {
      total = total * 2;
    }
  }
  const bump = () => {
    requestCount++;
    total = total + retries;
    cache[user] = total;
  };
  bump();
  return total;
}

export class Manager {
  private count = 0;
  process(input: string) {
    this.count++;
    return input.trim();
  }
}

export const run = async (value: number) => {
  if (value > 10) {
    return value;
  }
  return value + 1;
};
`,
	"src/view.tsx": `export function View(props: { items: string[] }) {
  const rows = props.items.map((item) => item.toUpperCase());
  return rows.join(",");
}
`,
	"native/counter.cpp": `#include <vector>
struct Counter {
  int value = 0;
  void bump(int amount) {
    value += amount;
    total += amount;
  }
};
int total = 0;
int compute(std::vector<int>& items, int a, int b, int c, int d, int e) {
  int sum = 0;
  for (int item : items) {
    if (item > a) { sum += item; } else if (item < b) { sum -= item; }
  }
  return sum;
}
`,
}

func writeCLikeCacheSources(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range clikeCacheSources {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return dir
}

func clikeLanguageFor(rel string) checkSupport.CLikeLanguage {
	if strings.HasSuffix(rel, ".cpp") {
		return checkSupport.CLikeCPP
	}
	return checkSupport.CLikeTypeScript
}

// TestParseCLikeFileMemoizesByPathLanguageAndContent pins the cache key: the
// same path, language, and bytes share one tree, while a content change or a
// different dialect gets its own parse, and every result equals a fresh parse.
func TestParseCLikeFileMemoizesByPathLanguageAndContent(t *testing.T) {
	sc, err := runnersupport.NewContext(context.Background(), core.Config{}, core.ScanOptions{Mode: core.ScanModeFull})
	if err != nil {
		t.Fatalf("new context: %v", err)
	}
	data := []byte(clikeCacheSources["src/service.ts"])
	first := runnersupport.ParseCLikeFile(sc, "src/service.ts", data, checkSupport.CLikeTypeScript)
	second := runnersupport.ParseCLikeFile(sc, "src/service.ts", append([]byte(nil), data...), checkSupport.CLikeTypeScript)
	if first != second {
		t.Fatal("identical path, language, and content were parsed twice")
	}
	if fresh := checkSupport.ParseCLike(string(data), checkSupport.CLikeTypeScript); !reflect.DeepEqual(first, fresh) {
		t.Fatal("cached parse differs from a fresh ParseCLike")
	}

	patched := append([]byte("// patched\n"), data...)
	changed := runnersupport.ParseCLikeFile(sc, "src/service.ts", patched, checkSupport.CLikeTypeScript)
	if changed == first {
		t.Fatal("changed content was served the stale cached tree")
	}
	if fresh := checkSupport.ParseCLike(string(patched), checkSupport.CLikeTypeScript); !reflect.DeepEqual(changed, fresh) {
		t.Fatal("cached parse of changed content differs from a fresh ParseCLike")
	}

	other := runnersupport.ParseCLikeFile(sc, "src/service.ts", data, checkSupport.CLikeCPP)
	if other == first {
		t.Fatal("a different dialect was served the TypeScript tree")
	}
	if fresh := checkSupport.ParseCLike(string(data), checkSupport.CLikeCPP); !reflect.DeepEqual(other, fresh) {
		t.Fatal("cached C++ parse differs from a fresh ParseCLike")
	}

	bare := runnersupport.Context{}
	if got := runnersupport.ParseCLikeFile(bare, "src/service.ts", data, checkSupport.CLikeTypeScript); got == first || !reflect.DeepEqual(got, first) {
		t.Fatal("a corpus-less Context must parse afresh and still match")
	}
}

func clikeCacheScanConfig(dir string, language string) core.Config {
	cfg := config.ExampleConfig()
	cfg.Name = "clike-cache"
	cfg.Targets = []core.TargetConfig{{Name: "app", Path: dir, Language: language}}
	cfg.Checks.Quality = true
	cfg.Checks.Design = true
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.QualityRules.MaxFileLines = 10
	cfg.Checks.QualityRules.MaxFunctionLines = 5
	cfg.Checks.QualityRules.MaxCyclomaticComplexity = 2
	cfg.Checks.QualityRules.MaxParameters = 3
	return cfg
}

func sortedFindingKeys(results []core.SectionResult) []string {
	keys := make([]string, 0)
	for _, section := range results {
		for _, finding := range section.Findings {
			keys = append(keys, section.ID+"|"+finding.RuleID+"|"+finding.Path+"|"+finding.Message+"|"+finding.Fingerprint+"|"+strconv.Itoa(finding.Line))
		}
	}
	sort.Strings(keys)
	return keys
}

// corpuslessCopy returns sc without its per-scan corpus, so every read and
// parse (C-like included) is done afresh: the pre-memoization behavior.
func corpuslessCopy(sc runnersupport.Context) runnersupport.Context {
	return runnersupport.Context{
		Cfg:               sc.Cfg,
		Opts:              sc.Opts,
		Baseline:          sc.Baseline,
		Artifacts:         runnersupport.NewArtifactStore(),
		RuleStats:         runnersupport.NewRuleStatsCollector(),
		Suppressed:        &runnersupport.SuppressedFindingCollector{},
		Today:             sc.Today,
		RuleCatalog:       sc.RuleCatalog,
		CustomRules:       sc.CustomRules,
		NLRuntime:         sc.NLRuntime,
		ConfigHash:        sc.ConfigHash,
		SectionConfigHash: sc.SectionConfigHash,
		DiffCommand:       sc.DiffCommand,
	}
}

// TestMemoizedCLikeParsesProduceIdenticalFindings runs quality and design
// (which each parse every TypeScript and C++ file, concurrently) once with the
// shared corpus and once without it, and requires identical findings. It then
// checks that the trees the scan cached were not mutated by any check: each
// still equals a fresh parse. Run with -race to catch concurrent mutation.
func TestMemoizedCLikeParsesProduceIdenticalFindings(t *testing.T) {
	dir := writeCLikeCacheSources(t)
	for _, language := range []string{"typescript", "cpp"} {
		t.Run(language, func(t *testing.T) {
			sc, err := runnersupport.NewContext(context.Background(), clikeCacheScanConfig(dir, language), core.ScanOptions{Mode: core.ScanModeFull})
			if err != nil {
				t.Fatalf("new context: %v", err)
			}
			// Prime the cache under the keys the scan uses (target-relative
			// paths), so the scan's sections share these exact trees and the
			// post-scan comparison inspects what the checks actually read.
			primed := map[string]*checkSupport.ParsedFile{}
			for rel, content := range clikeCacheSources {
				lang := clikeLanguageFor(rel)
				if (language == "cpp") == (lang == checkSupport.CLikeCPP) {
					primed[rel] = runnersupport.ParseCLikeFile(sc, filepath.FromSlash(rel), []byte(content), lang)
				}
			}
			cached := runnerchecks.Build(context.Background(), sc)
			uncached := runnerchecks.Build(context.Background(), corpuslessCopy(sc))
			cachedKeys, uncachedKeys := sortedFindingKeys(cached), sortedFindingKeys(uncached)
			if len(cachedKeys) == 0 {
				t.Fatal("fixture produced no findings; the comparison would be vacuous")
			}
			if !reflect.DeepEqual(cachedKeys, uncachedKeys) {
				t.Fatalf("memoized findings differ from uncached findings:\ncached:   %v\nuncached: %v", cachedKeys, uncachedKeys)
			}

			for rel, tree := range primed {
				lang := clikeLanguageFor(rel)
				if again := runnersupport.ParseCLikeFile(sc, filepath.FromSlash(rel), []byte(clikeCacheSources[rel]), lang); again != tree {
					t.Fatalf("%s: tree not memoized", rel)
				}
				if fresh := checkSupport.ParseCLike(clikeCacheSources[rel], lang); !reflect.DeepEqual(tree, fresh) {
					t.Fatalf("%s: a check mutated the shared cached tree", rel)
				}
			}
		})
	}
}
