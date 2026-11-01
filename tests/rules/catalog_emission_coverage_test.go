// Package rules_test asserts catalog invariants that are otherwise unenforced.
package rules_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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
	// checks. is the runner's own namespace. It once carried a section-panic
	// finding built by hand with no catalog entry; that outcome is now a
	// core.Diagnostic, and any future runner-emitted finding must be catalogued.
	"checks.",
}

// emissionRoots are the trees whose source can construct findings: the check
// sections themselves and the runner that wraps them (panic recovery lives
// there, outside any section package).
var emissionRoots = []string{
	filepath.Join("..", "..", "internal", "codeguard", "checks"),
	filepath.Join("..", "..", "internal", "codeguard", "runner", "checks"),
}

// TestEveryEmittedRuleIDIsInTheCatalog is the invariant that was missing when
// five performance rules shipped emitting findings with no catalog entry: the
// scanner printed them, then `codeguard explain` answered "unknown rule" and
// they carried no metadata, fix template, OWASP mapping, or SARIF descriptor.
// Catalog-to-implementation was already enforced; this is the reverse.
func TestEveryEmittedRuleIDIsInTheCatalog(t *testing.T) {
	catalog := rules.Catalog()

	emitted := map[string][]string{}
	diagnostics := map[string]bool{}
	visit := func(path string, entry fs.DirEntry, walkErr error) error {
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
	}
	for _, root := range emissionRoots {
		if err := filepath.WalkDir(root, visit); err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
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

// emissionSiteRoot is every tree that can construct a finding. The rules
// package is excluded: it is the catalog itself, so an id appearing there is
// what this test is checking for, not evidence of an emitter.
var emissionSiteRoot = filepath.Join("..", "..", "internal", "codeguard")

var emissionSiteExcludedDirs = []string{
	filepath.Join(emissionSiteRoot, "rules"),
}

// TestEveryCatalogRuleHasAnEmissionSite is the reverse of
// TestEveryEmittedRuleIDIsInTheCatalog. A catalog entry whose id is never used
// to build a finding documents behaviour the scanner does not have: `codeguard
// rules` lists it, waivers and configs can target it, and nothing ever fires.
// quality.mixed-abstraction-levels lived that way beside the real
// function.mixed-abstraction-level, kept "alive" by an unused constant.
//
// A bare string literal is therefore not enough. The id must reach a finding
// constructor: a struct field named *RuleID (core.Finding, FindingInput,
// table-driven rule specs), an argument to a call whose name contains
// "Finding", or an argument bound to a parameter named *ruleID. Ids are
// resolved through string constants, local variables, `+` concatenation, and
// small id-building helpers (qualityRuleID, support.RuleIDForScript, ...), so
// language-specific ids composed at runtime still count.
func TestEveryCatalogRuleHasAnEmissionSite(t *testing.T) {
	sites := newEmissionAnalyzer(t).sites()
	for id := range embeddedScriptEmissionSites(t) {
		sites[id] = true
	}
	if len(sites) == 0 {
		t.Fatal("found no emission sites under internal/codeguard; the scan is not working")
	}
	missing := make([]string, 0)
	for id := range rules.Catalog() {
		if !sites[id] {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	for _, id := range missing {
		t.Errorf("catalog rule %q has no emission site (no *RuleID field, *Finding(...) argument, or ruleID parameter resolves to it); remove the dead rule or alias it with rules.RuleAliases", id)
	}
}

// scriptEmissionPatterns find emission sites in the embedded TypeScript
// semantic runner (checks/support/*.js), which builds findings in JavaScript:
// scriptRuleId(flavor, "<ts id>", "<js id>") and rule_id/ruleId fields.
var scriptEmissionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`scriptRuleId\(\s*\w+\s*,\s*"([^"]+)"\s*,\s*"([^"]+)"\s*\)`),
	regexp.MustCompile(`\brule_?[iI]d\s*:\s*"([^"]+)"`),
}

func embeddedScriptEmissionSites(t *testing.T) map[string]bool {
	t.Helper()
	sites := map[string]bool{}
	err := filepath.WalkDir(emissionSiteRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".js") {
			return nil
		}
		data, readErr := os.ReadFile(path) //nolint:gosec // test walks a fixed in-repo tree
		if readErr != nil {
			return readErr
		}
		for _, pattern := range scriptEmissionPatterns {
			for _, match := range pattern.FindAllStringSubmatch(string(data), -1) {
				for _, id := range match[1:] {
					sites[id] = true
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", emissionSiteRoot, err)
	}
	return sites
}

// TestRuleAliasesResolveToEmittedCatalogRules keeps retired ids honest: an
// alias must not shadow a live catalog id and must point at a rule that both
// exists and is emitted.
func TestRuleAliasesResolveToEmittedCatalogRules(t *testing.T) {
	catalog := rules.Catalog()
	sites := newEmissionAnalyzer(t).sites()
	for alias, target := range rules.RuleAliases() {
		if _, ok := catalog[alias]; ok {
			t.Errorf("alias %q is still a catalog rule; drop its catalog entry", alias)
		}
		if _, ok := catalog[target]; !ok {
			t.Errorf("alias %q resolves to %q, which is not in the catalog", alias, target)
		}
		if !sites[target] {
			t.Errorf("alias %q resolves to %q, which has no emission site", alias, target)
		}
		if got := rules.ResolveRuleAlias(alias); got != target {
			t.Errorf("ResolveRuleAlias(%q) = %q, want %q", alias, got, target)
		}
	}
}

const (
	maxEvalDepth  = 6
	maxEvalValues = 64
)

// funcInfo is a named function (declaration or `name := func(...)` literal)
// that the analyzer can evaluate symbolically.
type funcInfo struct {
	dir    string
	params []string
	body   *ast.BlockStmt
	locals map[string][]ast.Expr
}

type emissionAnalyzer struct {
	files  []parsedGoFile
	consts map[string]string
	funcs  map[string][]*funcInfo
	// declScopes records, per top-level declaration, the function it belongs
	// to so emission sites can resolve locals and constants in context.
	declScopes map[ast.Decl]*funcInfo
}

type parsedGoFile struct {
	dir  string
	file *ast.File
}

type evalScope struct {
	dir    string
	fn     *funcInfo
	params map[string][]string
}

func newEmissionAnalyzer(t *testing.T) *emissionAnalyzer {
	t.Helper()
	a := &emissionAnalyzer{
		files:      parseEmissionSiteFiles(t),
		funcs:      map[string][]*funcInfo{},
		declScopes: map[ast.Decl]*funcInfo{},
	}
	a.consts = collectStringConsts(a.files)
	for _, parsed := range a.files {
		for _, decl := range parsed.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			info := &funcInfo{dir: parsed.dir, params: paramNames(fn.Type), body: fn.Body, locals: collectLocals(fn.Body)}
			a.funcs[fn.Name.Name] = append(a.funcs[fn.Name.Name], info)
			a.declScopes[decl] = info
			a.registerFuncLiterals(parsed.dir, info.locals)
		}
	}
	return a
}

// registerFuncLiterals records `name := func(...) {...}` closures so calls to
// them can bind their ruleID parameters.
func (a *emissionAnalyzer) registerFuncLiterals(dir string, locals map[string][]ast.Expr) {
	for name, exprs := range locals {
		for _, expr := range exprs {
			if lit, ok := expr.(*ast.FuncLit); ok {
				a.funcs[name] = append(a.funcs[name], &funcInfo{dir: dir, params: paramNames(lit.Type), body: lit.Body, locals: locals})
			}
		}
	}
}

func (a *emissionAnalyzer) sites() map[string]bool {
	sites := map[string]bool{}
	credit := func(values []string) {
		for _, value := range values {
			sites[value] = true
		}
	}
	for _, parsed := range a.files {
		for _, decl := range parsed.file.Decls {
			scope := &evalScope{dir: parsed.dir, fn: a.declScopes[decl]}
			ast.Inspect(decl, func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.KeyValueExpr:
					if key, ok := n.Key.(*ast.Ident); ok && isRuleIDName(key.Name) {
						credit(a.values(n.Value, scope, 0))
					}
				case *ast.CallExpr:
					name := calleeName(n.Fun)
					findingCall := strings.Contains(strings.ToLower(name), "finding")
					for i, arg := range n.Args {
						if findingCall || a.ruleIDParamAt(name, i) {
							credit(a.values(arg, scope, 0))
						}
					}
				}
				return true
			})
		}
	}
	return sites
}

func (a *emissionAnalyzer) ruleIDParamAt(name string, index int) bool {
	for _, info := range a.funcs[name] {
		if index < len(info.params) && isRuleIDName(info.params[index]) {
			return true
		}
	}
	return false
}

// values returns the possible string values of expr, or nil when they cannot
// be determined statically.
func (a *emissionAnalyzer) values(expr ast.Expr, scope *evalScope, depth int) []string {
	if depth > maxEvalDepth || expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return nil
		}
		if value, err := strconv.Unquote(e.Value); err == nil {
			return []string{value}
		}
	case *ast.ParenExpr:
		return a.values(e.X, scope, depth+1)
	case *ast.Ident:
		return a.identValues(e.Name, scope, depth)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return nil
		}
		return concatValues(a.values(e.X, scope, depth+1), a.values(e.Y, scope, depth+1))
	case *ast.CallExpr:
		return a.callValues(e, scope, depth)
	}
	return nil
}

func (a *emissionAnalyzer) identValues(name string, scope *evalScope, depth int) []string {
	if bound, ok := scope.params[name]; ok {
		return bound
	}
	if value, ok := a.consts[scope.dir+"\x00"+name]; ok {
		return []string{value}
	}
	if scope.fn == nil {
		return nil
	}
	var out []string
	for _, rhs := range scope.fn.locals[name] {
		out = append(out, a.values(rhs, scope, depth+1)...)
	}
	return capValues(out)
}

func (a *emissionAnalyzer) callValues(call *ast.CallExpr, scope *evalScope, depth int) []string {
	var out []string
	for _, info := range a.funcs[calleeName(call.Fun)] {
		params := map[string][]string{}
		for i, param := range info.params {
			if i < len(call.Args) {
				params[param] = a.values(call.Args[i], scope, depth+1)
			} else {
				params[param] = nil
			}
		}
		inner := &evalScope{dir: info.dir, fn: info, params: params}
		for _, result := range returnedExprs(info.body) {
			out = append(out, a.values(result, inner, depth+1)...)
		}
	}
	return capValues(out)
}

func concatValues(left []string, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	out := make([]string, 0, len(left)*len(right))
	for _, l := range left {
		for _, r := range right {
			out = append(out, l+r)
		}
	}
	return capValues(out)
}

func capValues(values []string) []string {
	if len(values) > maxEvalValues {
		return values[:maxEvalValues]
	}
	return values
}

func isRuleIDName(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), "ruleid")
}

func paramNames(fnType *ast.FuncType) []string {
	var names []string
	if fnType.Params == nil {
		return names
	}
	for _, field := range fnType.Params.List {
		if len(field.Names) == 0 {
			names = append(names, "_")
			continue
		}
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}
	return names
}

// collectLocals maps each local variable in body to every expression assigned
// to it, including inside nested closures.
func collectLocals(body *ast.BlockStmt) map[string][]ast.Expr {
	locals := map[string][]ast.Expr{}
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.AssignStmt:
			if len(n.Lhs) != len(n.Rhs) {
				return true
			}
			for i, lhs := range n.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok {
					locals[ident.Name] = append(locals[ident.Name], n.Rhs[i])
				}
			}
		case *ast.ValueSpec:
			if len(n.Names) != len(n.Values) {
				return true
			}
			for i, name := range n.Names {
				locals[name.Name] = append(locals[name.Name], n.Values[i])
			}
		}
		return true
	})
	return locals
}

// returnedExprs lists the single-result return expressions of body, skipping
// returns that belong to nested closures.
func returnedExprs(body *ast.BlockStmt) []ast.Expr {
	var out []ast.Expr
	if body == nil {
		return out
	}
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(n.Results) == 1 {
				out = append(out, n.Results[0])
			}
		}
		return true
	})
	return out
}

func parseEmissionSiteFiles(t *testing.T) []parsedGoFile {
	t.Helper()
	fset := token.NewFileSet()
	files := make([]parsedGoFile, 0, 1024)
	err := filepath.WalkDir(emissionSiteRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			for _, excluded := range emissionSiteExcludedDirs {
				if path == excluded {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		files = append(files, parsedGoFile{dir: filepath.Dir(path), file: file})
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", emissionSiteRoot, err)
	}
	return files
}

// collectStringConsts indexes every string constant (package-level and
// function-local) by package directory and name, resolving constants defined
// in terms of other constants.
func collectStringConsts(files []parsedGoFile) map[string]string {
	raw := map[string]ast.Expr{}
	for _, parsed := range files {
		ast.Inspect(parsed.file, func(node ast.Node) bool {
			decl, ok := node.(*ast.GenDecl)
			if !ok || decl.Tok != token.CONST {
				return true
			}
			for _, spec := range decl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range valueSpec.Names {
					if i < len(valueSpec.Values) {
						raw[parsed.dir+"\x00"+name.Name] = valueSpec.Values[i]
					}
				}
			}
			return true
		})
	}
	resolved := map[string]string{}
	for pass := 0; pass < 4; pass++ {
		for key, expr := range raw {
			if _, done := resolved[key]; done {
				continue
			}
			dir := key[:strings.IndexByte(key, 0)]
			if value, ok := constStringValue(expr, dir, resolved); ok {
				resolved[key] = value
			}
		}
	}
	return resolved
}

func constStringValue(expr ast.Expr, dir string, consts map[string]string) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(e.Value)
		return value, err == nil
	case *ast.Ident:
		value, ok := consts[dir+"\x00"+e.Name]
		return value, ok
	case *ast.ParenExpr:
		return constStringValue(e.X, dir, consts)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		left, okLeft := constStringValue(e.X, dir, consts)
		right, okRight := constStringValue(e.Y, dir, consts)
		return left + right, okLeft && okRight
	}
	return "", false
}

func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	case *ast.IndexExpr:
		return calleeName(f.X)
	case *ast.IndexListExpr:
		return calleeName(f.X)
	}
	return ""
}
