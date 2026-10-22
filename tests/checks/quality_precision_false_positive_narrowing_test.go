package checks_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// goModuleFixture writes a go.mod so that quality.dependency-direction can map
// a file path back to its Go import path.
func goModuleFixture(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "go.mod"), strings.Join([]string{
		"module example.com/app",
		"",
		"go 1.22",
		"",
	}, "\n"))
}

func TestDependencyDirectionAllowsSameModuleInternalImportFromPublicPackage(t *testing.T) {
	dir := t.TempDir()
	goModuleFixture(t, dir)
	writeFile(t, filepath.Join(dir, "pkg", "app", "facade.go"), strings.Join([]string{
		"package app",
		"",
		"import \"example.com/app/internal/engine\"",
		"",
		"type Options = engine.Options",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.dependency-direction")
}

func TestDependencyDirectionAllowsSingularTestDirectoryExternalTestPackage(t *testing.T) {
	dir := t.TempDir()
	goModuleFixture(t, dir)
	// "test/" (singular), which the previous hardcoded "tests/" prefix missed.
	writeFile(t, filepath.Join(dir, "test", "cli", "config_test.go"), strings.Join([]string{
		"package cli_test",
		"",
		"import (",
		"\t\"testing\"",
		"",
		"\t\"example.com/app/internal/config\"",
		"\t\"example.com/app/internal/engine\"",
		")",
		"",
		"func TestLoad(t *testing.T) {",
		"\t_ = config.Default",
		"\t_ = engine.Default",
		"}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.dependency-direction")
}

func TestDependencyDirectionWarnsForInternalImportOutsideItsOwnTree(t *testing.T) {
	dir := t.TempDir()
	goModuleFixture(t, dir)
	// services/b/internal is scoped to services/b; services/a may not reach it.
	writeFile(t, filepath.Join(dir, "services", "a", "wire.go"), strings.Join([]string{
		"package a",
		"",
		"import \"example.com/app/services/b/internal/secretstore\"",
		"",
		"var Store = secretstore.Default",
		"",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "services", "b", "internal", "secretstore", "store.go"), strings.Join([]string{
		"package secretstore",
		"",
		"var Default = \"store\"",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRulePresent(t, report, "Code Quality", "quality.dependency-direction")
	assertFindingLevel(t, report, "Code Quality", "quality.dependency-direction", "warn")
}

func TestCardinalityMismatchAllowsSingularNamesThatAreNotElementTypes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "shapes.go"), strings.Join([]string{
		"package sample",
		"",
		"type Ref struct{ Name string }",
		"",
		"func WriteChunk(chunk []byte) int { return len(chunk) }",
		"",
		"func RunCommand(command []string) int { return len(command) }",
		"",
		"func Resolve(index map[string]Ref) int { return len(index) }",
		"",
		"func Compare(left, right []Ref) bool { return len(left) == len(right) }",
		"",
		"func Budget(maxLines int) int { return maxLines }",
		"",
		"func Render(lineNumbers bool) bool { return lineNumbers }",
		"",
		"func Grade(outputs1 []Ref, outputs2 []Ref) int { return len(outputs1) + len(outputs2) }",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", "naming.cardinality-mismatch")
}

func TestCardinalityMismatchWarnsWhenNameIsTheElementType(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "collections.go"), strings.Join([]string{
		"package sample",
		"",
		"type Invoice struct{ ID string }",
		"",
		"func Total(invoice []Invoice) int { return len(invoice) }",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRulePresent(t, report, "Code Quality", "naming.cardinality-mismatch")
	assertFindingMessageContains(t, report, "naming.cardinality-mismatch", "\"invoice\"")
}

func TestCardinalityMismatchWarnsForPluralNameOnStringScalar(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "scalars.go"), strings.Join([]string{
		"package sample",
		"",
		"func Notify(recipients string) int { return len(recipients) }",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRulePresent(t, report, "Code Quality", "naming.cardinality-mismatch")
	assertFindingMessageContains(t, report, "naming.cardinality-mismatch", "\"recipients\"")
}

func TestNullAssumptionAllowsGoPointerOutParametersAndAccumulators(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.go"), strings.Join([]string{
		"package sample",
		"",
		"import \"strings\"",
		"",
		"type Summary struct{ Commands int }",
		"",
		"func plistProgram(out *strings.Builder) {",
		"\tout.WriteString(\"<key>ProgramArguments</key>\")",
		"}",
		"",
		"func addArchivedCounters(summary *Summary, delta int) {",
		"\tsummary.Commands += delta",
		"}",
		"",
		"func handleRequestSummary(summary *Summary) int {",
		"\treturn summary.Commands",
		"}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.null-assumption")
}

func TestNullAssumptionWarnsForDeclaredOptionalBoundaryPayload(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "route.ts"), strings.Join([]string{
		"type Order = { id: string };",
		"",
		"export function handleOrderRoute(payload: Order | null) {",
		"  return payload.id;",
		"}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRulePresent(t, report, "Code Quality", "defensive.null-assumption")
	assertFindingConfidence(t, report, "Code Quality", "defensive.null-assumption", "low")
}

func TestNullAssumptionAllowsGuardedOptionalBoundaryPayload(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "route_guarded.ts"), strings.Join([]string{
		"type Order = { id: string };",
		"",
		"export function handleOrderRoute(payload: Order | null) {",
		"  if (payload === null) {",
		"    return \"\";",
		"  }",
		"  return payload.id;",
		"}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.null-assumption")
}

func TestFailOpenAuthorizationAllowsFailClosedErrorReturn(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "policy_load.go"), strings.Join([]string{
		"package sample",
		"",
		"import \"errors\"",
		"",
		"func loadPolicyPack(name string) (map[string]string, error) {",
		"\tpack, err := readPack(name)",
		"\tif err != nil {",
		"\t\treturn nil, err",
		"\t}",
		"\treturn pack, nil",
		"}",
		"",
		"func readPack(name string) (map[string]string, error) {",
		"\tif name == \"\" {",
		"\t\treturn nil, errors.New(\"empty\")",
		"\t}",
		"\treturn map[string]string{}, nil",
		"}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.fail-open-authorization")
}

func TestFailOpenAuthorizationWarnsWhenErrorPathReturnsAllow(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "authorize.go"), strings.Join([]string{
		"package sample",
		"",
		"import \"errors\"",
		"",
		"func authorizeRequest(subject string) bool {",
		"\tallowed, err := lookupPermission(subject)",
		"\tif err != nil {",
		"\t\treturn true",
		"\t}",
		"\treturn allowed",
		"}",
		"",
		"func lookupPermission(subject string) (bool, error) {",
		"\tif subject == \"\" {",
		"\t\treturn false, errors.New(\"empty\")",
		"\t}",
		"\treturn true, nil",
		"}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRulePresent(t, report, "Code Quality", "defensive.fail-open-authorization")
	assertFindingConfidence(t, report, "Code Quality", "defensive.fail-open-authorization", "medium")
}

func TestUnsafeDefaultAllowsOrdinaryConfigCallsWithStringArguments(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "rules.go"), strings.Join([]string{
		"package sample",
		"",
		"type Config struct{ Rules []string }",
		"",
		"func collect(rules []string, kind string, verdict string) []string { return rules }",
		"",
		"func evaluate(cfg Config) []string {",
		"\treturn collect(cfg.Rules, \"tool\", \"allow\")",
		"}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.unsafe-default")
}

func TestUnsafeDefaultWarnsForDefaultingSafetyControlToAllow(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "guard.go"), strings.Join([]string{
		"package sample",
		"",
		"import \"os\"",
		"",
		"func insecureAllowed() bool {",
		"\tskipVerify := os.Getenv(\"STRICT_TLS\") == \"\" || true",
		"\treturn skipVerify",
		"}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRulePresent(t, report, "Code Quality", "defensive.unsafe-default")
	assertFindingConfidence(t, report, "Code Quality", "defensive.unsafe-default", "low")
}

func TestDependencyDirectionHonoursConfiguredTestPaths(t *testing.T) {
	dir := t.TempDir()
	goModuleFixture(t, dir)
	// A test support helper with no test-naming marker, declared as test code
	// through ci_rules.allowed_test_paths.
	writeFile(t, filepath.Join(dir, "services", "a", "wire.go"), strings.Join([]string{
		"package a",
		"",
		"import \"example.com/app/services/b/internal/secretstore\"",
		"",
		"var Store = secretstore.Default",
		"",
	}, "\n"))

	cfg := qualityPrecisionConfig(dir)
	cfg.Checks.CIRules.AllowedTestPaths = []string{"services/a/**"}

	report := runQualityPrecisionScan(t, cfg)

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.dependency-direction")

	strict := qualityPrecisionConfig(dir)
	strict.Checks.CIRules.AllowedTestPaths = []string{"tests/**"}
	assertFindingRulePresent(t, runQualityPrecisionScan(t, strict), "Code Quality", "quality.dependency-direction")
}
