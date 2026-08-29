package checks_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestGoNestedCallbackReturnsDoNotLeakIntoParentReturnContract(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "repository", "save.go"), strings.Join([]string{
		"package repository",
		"",
		"import (",
		"\t\"context\"",
		"\t\"fmt\"",
		")",
		"",
		"type Pool interface{}",
		"type Tx interface{}",
		"",
		"func WithTxValue(ctx context.Context, pool Pool, fn func(Tx) (bool, error)) (bool, error) {",
		"\treturn fn(nil)",
		"}",
		"",
		"func SaveThing(ctx context.Context, pool Pool, failed bool, err error) error {",
		"\tvalue, err := WithTxValue(ctx, pool, func(tx Tx) (bool, error) {",
		"\t\tif failed {",
		"\t\t\treturn false, err",
		"\t\t}",
		"\t\treturn true, nil",
		"\t})",
		"\tif err != nil {",
		"\t\treturn fmt.Errorf(\"save thing: %w\", err)",
		"\t}",
		"\t_ = value",
		"\treturn nil",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", "function.inconsistent-return-contract")
	assertFindingRuleAbsent(t, report, "Code Quality", "function.partial-result")
}

func TestGoRepositoryCommandBoolErrorContractIsExplicitMutationBoundary(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "internal", "postgres", "repository", "delete.go"), strings.Join([]string{
		"package repository",
		"",
		"import (",
		"\t\"context\"",
		"\t\"fmt\"",
		")",
		"",
		"type Repository struct { queries Queries }",
		"type Queries interface { DeleteThing(context.Context, string) (int64, error) }",
		"",
		"func (r *Repository) DeleteThing(ctx context.Context, thingID string) (bool, error) {",
		"\taffected, err := r.queries.DeleteThing(ctx, thingID)",
		"\tif err != nil {",
		"\t\treturn false, fmt.Errorf(\"delete thing: %w\", err)",
		"\t}",
		"\treturn affected > 0, nil",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", "function.partial-result")
	assertFindingRuleAbsent(t, report, "Code Quality", "function.inconsistent-return-contract")
	assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
}

func TestGoPrimitiveObsessionSkipsInterfaceImplementationSignature(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "internal", "postgres", "repository", "collections.go"), strings.Join([]string{
		"package repository",
		"",
		"import \"context\"",
		"",
		"type CollectionRepository interface {",
		"\tDeleteCollectionItem(ctx context.Context, collectionID string, itemType string, itemID string) (bool, error)",
		"}",
		"",
		"type Repository struct{}",
		"",
		"func (r *Repository) DeleteCollectionItem(ctx context.Context, collectionID string, itemType string, itemID string) (bool, error) {",
		"\treturn false, nil",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.primitive-obsession")
}

func TestMaintainabilityHistoryDefaultsToAdvisoryArtifactsNotFindings(t *testing.T) {
	dir := initMaintainabilityHistoryRepo(t)

	report := runMaintainabilityDeltaScan(t, qualityPrecisionConfig(dir))

	for _, ruleID := range []string{
		"maintainability.hotspot",
		"maintainability.high-churn-hotspot",
		"maintainability.repeat-defect-area",
		"maintainability.unstable-interface",
		"maintainability.change-amplification",
		"smell.shotgun-surgery-history",
		"smell.divergent-change-history",
	} {
		assertFindingRuleAbsent(t, report, "Code Quality", ruleID)
	}
	if history := maintainabilityHistoryArtifact(report); history == nil || len(history.Hotspots) == 0 {
		t.Fatalf("expected maintainability history advisory artifact, got %#v", report.Artifacts)
	}
}

func TestBooleanNotPredicateRequiresStrongBooleanValueEvidence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages", "api", "src", "booleans.ts"), strings.Join([]string{
		"export function RequireCurrentUser(req: Request): User {",
		"  return req.user;",
		"}",
		"export function RawStringField(value: string): Field {",
		"  return { value };",
		"}",
		"export function fileExists(path: string): boolean {",
		"  return path.length > 0;",
		"}",
		"export function boolFromAny(value: unknown): boolean {",
		"  return Boolean(value);",
		"}",
		"export function ipInCIDRs(ip: string, cidrs: string[]): boolean {",
		"  return cidrs.includes(ip);",
		"}",
		"export function lookupUser(id: string): [User, boolean] {",
		"  return [{ id }, true];",
		"}",
		"interface Request { user: User }",
		"interface User { id: string }",
		"interface Field { value: string }",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "internal", "postgres", "repository", "commands.go"), strings.Join([]string{
		"package repository",
		"",
		"import \"context\"",
		"",
		"type Repository struct{}",
		"",
		"func (r *Repository) SaveThing(ctx context.Context, thingID string) (bool, error) {",
		"\treturn true, nil",
		"}",
	}, "\n"))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "boolean-retune"
	cfg.Targets = []codeguard.TargetConfig{
		{Name: "ts", Path: dir, Language: "typescript"},
		{Name: "go", Path: dir, Language: "go"},
	}
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	off := false
	cfg.Checks.Context = &off
	cfg.Cache.Enabled = &off

	report := runQualityPrecisionScan(t, cfg)

	assertFindingRuleAbsent(t, report, "Code Quality", "naming.boolean-not-predicate")
}

func maintainabilityHistoryArtifact(report codeguard.Report) *codeguard.PRHotspotsArtifact {
	for _, artifact := range report.Artifacts {
		if artifact.Kind == "maintainability_history" && artifact.PRHotspots != nil {
			return artifact.PRHotspots
		}
	}
	return nil
}
