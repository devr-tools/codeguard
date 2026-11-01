package checks_test

import (
	"context"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// The *.javascript.* rules share detectors with their *.typescript.* twins and
// differ only in the rule id picked from the file extension. Until these tests
// they were only exercised through .ts fixtures, so a regression in the .js
// branch (wrong id, skipped extension) could ship unnoticed. Each test scans
// .js files and pins: the JavaScript id fires on the expected line, the
// TypeScript twin never fires for a .js file, and a clean .js file is silent.

// ruleLines returns sorted "rule@line" keys for findings whose rule id has
// one of the given prefixes.
func ruleLines(report codeguard.Report, prefixes ...string) []string {
	out := make([]string, 0)
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			for _, prefix := range prefixes {
				if strings.HasPrefix(finding.RuleID, prefix) {
					out = append(out, finding.RuleID+"@"+strconv.Itoa(finding.Line))
					break
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func javaScriptVariantConfig(name, dir string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "javascript"}}
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SecurityRules.GovulncheckMode = "off"
	return cfg
}

func runJavaScriptVariantScan(t *testing.T, cfg codeguard.Config) codeguard.Report {
	t.Helper()
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

// Flow-annotated JavaScript can carry `any`, `as` and `!` syntax. The
// quality.javascript.{explicit-any,double-assertion,non-null-assertion} rules
// were only ever tested through .ts files.
func TestJavaScriptQualityTypeEscapeVariants(t *testing.T) {
	disableTypeScriptSemanticEngine(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "loader.js"), strings.Join([]string{
		"// @flow",
		"export function load(input: any) {",
		"  const raw = input as unknown as Payload;",
		"  return raw.items!.length;",
		"}",
		"",
	}, "\n"))
	cfg := javaScriptVariantConfig("js-quality-variants", dir)
	cfg.Checks.Quality = true
	report := runJavaScriptVariantScan(t, cfg)

	want := []string{
		"quality.javascript.double-assertion@3",
		"quality.javascript.explicit-any@2",
		"quality.javascript.non-null-assertion@4",
	}
	got := ruleLines(report, "quality.javascript.explicit-any", "quality.javascript.double-assertion", "quality.javascript.non-null-assertion")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JavaScript type-escape findings = %q, want %q", got, want)
	}
	if twins := ruleLines(report, "quality.typescript."); len(twins) != 0 {
		t.Fatalf("a .js file produced TypeScript-variant findings: %q", twins)
	}
}

func TestJavaScriptQualityTypeEscapeVariantsSilentOnPlainJS(t *testing.T) {
	disableTypeScriptSemanticEngine(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "loader.js"), strings.Join([]string{
		"export function load(input) {",
		"  const ready = !input.pending;",
		"  return ready && input.items.length !== 0;",
		"}",
		"",
	}, "\n"))
	cfg := javaScriptVariantConfig("js-quality-variants-clean", dir)
	cfg.Checks.Quality = true
	report := runJavaScriptVariantScan(t, cfg)
	if got := ruleLines(report, "quality.javascript.explicit-any", "quality.javascript.double-assertion", "quality.javascript.non-null-assertion"); len(got) != 0 {
		t.Fatalf("plain JavaScript produced type-escape findings: %q", got)
	}
}

func TestJavaScriptPerformanceVariants(t *testing.T) {
	disableTypeScriptSemanticEngine(t)
	cases := []struct {
		rule   string
		file   string
		source string
		lines  []int
	}{
		{
			rule:   "performance.javascript.await-in-loop",
			file:   "serial.js",
			source: "export async function loadUsers(ids) {\n  const users = [];\n  for (const id of ids) {\n    const user = await loadUser(id);\n    users.push(user);\n  }\n  return users;\n}\n",
			lines:  []int{4},
		},
		{
			rule:   "performance.javascript.sync-io-in-handler",
			file:   "server.js",
			source: "import fs from \"fs\";\nimport express from \"express\";\n\nconst app = express();\n\napp.get(\"/report\", (req, res) => {\n  const data = fs.readFileSync(\"report.txt\", \"utf8\");\n  res.send(data);\n});\n",
			lines:  []int{7},
		},
		{
			rule:   "performance.javascript.unbounded-concurrency",
			file:   "warm.js",
			source: "export function warm(urls) {\n  const tasks = [];\n  for (const url of urls) {\n    tasks.push(fetch(url));\n  }\n  return Promise.all(tasks);\n}\n",
			lines:  []int{4},
		},
		{
			rule:   "performance.javascript.timer-listener-leak",
			file:   "poll.js",
			source: "export function start(items) {\n  setInterval(refresh, 1000);\n  for (const el of items) {\n    el.addEventListener(\"click\", onClick);\n  }\n}\n",
			// The uncleared setInterval and the listener added in a loop
			// are reported separately.
			lines: []int{2, 4},
		},
		{
			rule:   "performance.javascript.express-sync-middleware",
			file:   "middleware.js",
			source: "import express from \"express\";\nimport bcrypt from \"bcrypt\";\n\nconst app = express();\n\napp.use((req, res, next) => {\n  req.hash = bcrypt.hashSync(req.body.password, 10);\n  next();\n});\n",
			lines:  []int{7},
		},
		{
			rule:   "performance.javascript.react-expensive-render",
			file:   "ItemList.js",
			source: "import React from \"react\";\n\nexport function ItemList({ items }) {\n  const rows = items.filter((i) => i.active).map((i) => show(i));\n  return rows;\n}\n",
			lines:  []int{4},
		},
	}
	for _, tc := range cases {
		t.Run(tc.rule, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "src", tc.file), tc.source)
			report := runJavaScriptVariantScan(t, performanceConfig("js-"+tc.file, dir, "javascript"))

			got := ruleLines(report, tc.rule)
			want := make([]string, 0, len(tc.lines))
			for _, line := range tc.lines {
				want = append(want, tc.rule+"@"+strconv.Itoa(line))
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s findings = %q, want %q (all performance findings: %q)", tc.rule, got, want, ruleLines(report, "performance."))
			}
			if twins := ruleLines(report, "performance.typescript."); len(twins) != 0 {
				t.Fatalf("a .js file produced TypeScript-variant findings: %q", twins)
			}
		})
	}
}

func TestJavaScriptPerformanceVariantsSilentOnCleanJS(t *testing.T) {
	disableTypeScriptSemanticEngine(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "clean.js"), strings.Join([]string{
		"import fs from \"fs\";",
		"import pLimit from \"p-limit\";",
		"",
		"const config = fs.readFileSync(\"config.json\", \"utf8\");",
		"const limit = pLimit(4);",
		"",
		"export async function drain(stream) {",
		"  for await (const chunk of stream) {",
		"    consume(chunk);",
		"  }",
		"}",
		"",
		"export function warm(urls) {",
		"  return Promise.all(urls.map((url) => limit(() => fetch(url))));",
		"}",
		"",
		"export function poll() {",
		"  const handle = setInterval(refresh, 1000);",
		"  return () => clearInterval(handle);",
		"}",
		"",
	}, "\n"))
	report := runJavaScriptVariantScan(t, performanceConfig("js-performance-clean", dir, "javascript"))
	if got := ruleLines(report, "performance.javascript.", "performance.typescript."); len(got) != 0 {
		t.Fatalf("clean JavaScript produced script performance findings: %q", got)
	}
}

func TestJavaScriptSecurityDynamicCodeVariants(t *testing.T) {
	disableTypeScriptSemanticEngine(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "plugin.js"), strings.Join([]string{
		"import vm from \"node:vm\";",
		"",
		"export function runPlugin(source, sandbox) {",
		"  const value = eval(source);",
		"  const fn = new Function(\"x\", source);",
		"  return vm.runInNewContext(source, sandbox) || fn(value);",
		"}",
		"",
	}, "\n"))
	cfg := javaScriptVariantConfig("js-security-variants", dir)
	cfg.Checks.Security = true
	report := runJavaScriptVariantScan(t, cfg)

	want := []string{
		"security.javascript.dynamic-code@4",
		"security.javascript.dynamic-code@5",
		"security.javascript.vm-dynamic-code@6",
	}
	got := ruleLines(report, "security.javascript.dynamic-code", "security.javascript.vm-dynamic-code")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JavaScript dynamic-code findings = %q, want %q", got, want)
	}
	if twins := ruleLines(report, "security.typescript."); len(twins) != 0 {
		t.Fatalf("a .js file produced TypeScript-variant findings: %q", twins)
	}
}

func TestJavaScriptSecurityDynamicCodeVariantsSilentOnCleanJS(t *testing.T) {
	disableTypeScriptSemanticEngine(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "plugin.js"), strings.Join([]string{
		"export function runPlugin(source) {",
		"  // eval(source) is deliberately not used here",
		"  const evaluate = JSON.parse(source);",
		"  return \"new Function(x)\" + evaluate.name;",
		"}",
		"",
	}, "\n"))
	cfg := javaScriptVariantConfig("js-security-variants-clean", dir)
	cfg.Checks.Security = true
	report := runJavaScriptVariantScan(t, cfg)
	if got := ruleLines(report, "security.javascript.dynamic-code", "security.javascript.vm-dynamic-code"); len(got) != 0 {
		t.Fatalf("clean JavaScript produced dynamic-code findings: %q", got)
	}
}

// security.javascript.taint-flow comes from the semantic engine, so it needs a
// Node TypeScript runtime; requireTypeScriptSemanticRuntime skips explicitly
// when there is none.
func TestJavaScriptSecurityTaintFlowVariant(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "db.js"),
		"import { Pool } from \"pg\";\n"+
			"const pool = new Pool();\n"+
			"export function runQuery(id) {\n"+
			"  return pool.query(\"SELECT * FROM users WHERE id = \" + id);\n"+
			"}\n")
	writeFile(t, filepath.Join(dir, "src", "routes.js"),
		"import { runQuery } from \"./db\";\n"+
			"export function handler(req) {\n"+
			"  runQuery(req.query.id);\n"+
			"}\n")
	writeFile(t, filepath.Join(dir, "src", "safe.js"),
		"import { runQuery } from \"./db\";\n"+
			"export function fixed() {\n"+
			"  runQuery(\"42\");\n"+
			"}\n")
	cfg := typeScriptTaintConfig(dir)
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "javascript"}}
	report := runTypeScriptTaintScan(t, cfg)

	// The finding sits at the sink; the message carries the whole path. The
	// constant call in safe.js reaches the same sink and must not add a flow.
	got := make([]string, 0)
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if !strings.HasSuffix(finding.RuleID, ".taint-flow") {
				continue
			}
			got = append(got, finding.RuleID+" "+finding.Path+":"+strconv.Itoa(finding.Line)+" "+finding.Message)
		}
	}
	want := []string{"security.javascript.taint-flow src/db.js:4 tainted data flow: request query (src/routes.js:3) \u2192 runQuery arg (src/routes.js:3) \u2192 pool.query sink (src/db.js:4)"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("taint findings = %q, want %q", got, want)
	}
}
