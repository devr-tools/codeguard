package rules_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/rules"
)

var docsDir = filepath.Join("..", "..", "docs")

// linkedDocPattern finds relative links from checks.md to sibling docs pages.
// A rule documented on a page that checks.md links to counts as documented,
// so a section can move its reference table out of checks.md without
// breaking this invariant.
var linkedDocPattern = regexp.MustCompile(`\]\(([a-z0-9_-]+\.md)(?:#[^)]*)?\)`)

// undocumentedConfigKeys lists yaml keys that are deliberately absent from the
// user-facing docs, each with the reason. It is empty today: every key a user
// can set is documented, including keys that are parsed but not yet read
// (docs/config-reference.md marks those "Reserved"). Add an entry only for a
// key that is genuinely internal plumbing, never to silence a missing doc.
var undocumentedConfigKeys = map[string]string{}

// ruleIndexRow matches a row of docs/rule-index.md: rule id, default level.
var ruleIndexRow = regexp.MustCompile("(?m)^\\| `([^`]+)` \\| ([a-z]+) \\|")

// TestEveryCatalogRuleIsDocumented keeps docs/checks.md (and the pages it
// links to) in step with rules.Catalog(). Before this test roughly half the
// catalog appeared in no doc, so `codeguard explain` was the only way to learn
// a rule existed.
func TestEveryCatalogRuleIsDocumented(t *testing.T) {
	text := readRuleDocs(t)
	missing := make([]string, 0)
	for id := range rules.Catalog() {
		if !containsToken(text, id, true) {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d catalog rule ids appear in neither docs/checks.md nor a page it links to; add a row to that section's rule table:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// TestRuleIndexLevelsMatchCatalog keeps the default-level column of
// docs/rule-index.md honest: a rule whose level changes in the catalog must
// have its row updated too, or the index silently misreports severity.
func TestRuleIndexLevelsMatchCatalog(t *testing.T) {
	catalog := rules.Catalog()
	rows := ruleIndexRow.FindAllStringSubmatch(readDoc(t, "rule-index.md"), -1)
	if len(rows) == 0 {
		t.Fatal("found no rule rows in docs/rule-index.md; the row pattern is broken")
	}
	seen := map[string]bool{}
	for _, row := range rows {
		id, level := row[1], row[2]
		if seen[id] {
			t.Errorf("docs/rule-index.md lists %s more than once", id)
		}
		seen[id] = true
		meta, ok := catalog[id]
		if !ok {
			t.Errorf("docs/rule-index.md lists %s, which is not in rules.Catalog(); remove the row", id)
			continue
		}
		if meta.DefaultLevel != level {
			t.Errorf("docs/rule-index.md says %s defaults to %q, catalog says %q", id, level, meta.DefaultLevel)
		}
	}
	for id := range catalog {
		if !seen[id] {
			t.Errorf("docs/rule-index.md has no row for catalog rule %s", id)
		}
	}
}

// TestEveryConfigKeyIsDocumented walks the config structs' yaml tags and
// requires every key to appear somewhere under docs/. Keys that a user cannot
// meaningfully set are listed, with a reason, in undocumentedConfigKeys.
func TestEveryConfigKeyIsDocumented(t *testing.T) {
	keys := map[string]bool{}
	collectYAMLKeys(reflect.TypeOf(core.Config{}), keys, map[reflect.Type]bool{})
	if len(keys) < 100 {
		t.Fatalf("collected only %d config keys; the struct walk is broken", len(keys))
	}
	text := readAllDocs(t)
	missing := make([]string, 0)
	for key := range keys {
		if _, ok := undocumentedConfigKeys[key]; ok {
			continue
		}
		if !containsToken(text, key, false) {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d config keys appear nowhere under docs/; document them (key, default, meaning) in the owning section:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
	for key := range undocumentedConfigKeys {
		if !keys[key] {
			t.Errorf("undocumentedConfigKeys lists %q, which is no longer a config key; remove it", key)
		}
	}
}

func collectYAMLKeys(typ reflect.Type, keys map[string]bool, seen map[reflect.Type]bool) {
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array || typ.Kind() == reflect.Map {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct || seen[typ] {
		return
	}
	seen[typ] = true
	for i := range typ.NumField() {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		if name == "-" {
			continue
		}
		if name != "" {
			keys[name] = true
		}
		collectYAMLKeys(field.Type, keys, seen)
	}
}

func readRuleDocs(t *testing.T) string {
	t.Helper()
	main := readDoc(t, "checks.md")
	var builder strings.Builder
	builder.WriteString(main)
	for _, match := range linkedDocPattern.FindAllStringSubmatch(main, -1) {
		builder.WriteString("\n")
		builder.WriteString(readDoc(t, match[1]))
	}
	return builder.String()
}

func readAllDocs(t *testing.T) string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(docsDir, "*.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob docs: %v (%d files)", err, len(paths))
	}
	var builder strings.Builder
	for _, path := range paths {
		builder.WriteString(readDoc(t, filepath.Base(path)))
		builder.WriteString("\n")
	}
	return builder.String()
}

func readDoc(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(docsDir, name)) //nolint:gosec // fixed in-repo docs tree
	if err != nil {
		t.Fatalf("read docs/%s: %v", name, err)
	}
	return string(data)
}

// containsToken reports whether needle occurs in text as a whole identifier,
// so `max_depth` is not satisfied by `typescript_taint_max_depth` and
// `security.secrets` is not satisfied by `security.secrets-config`.
//
// dottedSuffixIsPart controls whether a following ".segment" extends the
// token: rule ids nest by dots (`a.b` is not documented by `a.b.c`), while a
// config key is legitimately followed by a dotted child (`risk_scoring.x`).
func containsToken(text, needle string, dottedSuffixIsPart bool) bool {
	for start := 0; ; {
		idx := strings.Index(text[start:], needle)
		if idx < 0 {
			return false
		}
		idx += start
		end := idx + len(needle)
		if !isIdentByte(text, idx-1) && !isIdentByte(text, end) && (!dottedSuffixIsPart || !dottedContinuation(text, end)) {
			return true
		}
		start = idx + 1
	}
}

func isIdentByte(text string, idx int) bool {
	if idx < 0 || idx >= len(text) {
		return false
	}
	c := text[idx]
	return c == '_' || c == '-' || isAlnum(c)
}

func dottedContinuation(text string, idx int) bool {
	return idx+1 < len(text) && text[idx] == '.' && isAlnum(text[idx+1])
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
