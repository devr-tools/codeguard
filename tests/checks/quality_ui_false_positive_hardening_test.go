package checks_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualityAmbiguousNameAllowsConventionalUIParams(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/claims/claim-classification-fields.tsx"), strings.Join([]string{
		"export function ClaimClassificationFields() {",
		"  function onChange(value: string) {",
		"    return value.trim();",
		"  }",
		"  function renderItem(item: Item) {",
		"    return <span>{item.label}</span>;",
		"  }",
		"  return <Field onChange={onChange} renderItem={renderItem} />;",
		"}",
		"interface Item { label: string }",
		"declare function Field(props: unknown): unknown;",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ambiguous-name")
}

func TestFunctionCommandQueryMixAllowsReactAndNextBoundaries(t *testing.T) {
	cases := []struct {
		name   string
		file   string
		source []string
	}{
		{
			name: "react component",
			file: "apps/web/app/contracts/contract-detail-classification.tsx",
			source: []string{
				"export function ContractDetailClassification() {",
				"  async function onSave() {",
				"    await repo.save({ ok: true });",
				"  }",
				"  return <button onClick={onSave}>Save</button>;",
				"}",
			},
		},
		{
			name: "react hook",
			file: "apps/web/app/contracts/use-contract-classification.ts",
			source: []string{
				"export function useContractClassification(repo: Repository) {",
				"  async function saveClassification() {",
				"    await repo.save({ ok: true });",
				"  }",
				"  return { saveClassification };",
				"}",
				"interface Repository { save(input: unknown): Promise<void> }",
			},
		},
		{
			name: "next route",
			file: "apps/web/app/api/files/upload/route.ts",
			source: []string{
				"export async function POST(request: Request) {",
				"  await storage.upload(request);",
				"  return Response.json({ ok: true });",
				"}",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), strings.Join(tc.source, "\n"))

			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

			assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
		})
	}
}

func TestQualityDuplicatedKnowledgeSkipsDisplayStringsAndIncludesLiteral(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/claims/labels.tsx"), strings.Join([]string{
		"export function Labels() {",
		"  return <div className=\"status status\">Status</div>;",
		"}",
		"export const first = 'claim_status_code';",
		"export const second = 'claim_status_code';",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	finding := firstFindingForRule(t, report, "Code Quality", "quality.duplicated-knowledge")
	if !strings.Contains(finding.Message, "'claim_status_code'") {
		t.Fatalf("expected duplicated literal in message, got %q", finding.Message)
	}
}

func TestNamingCardinalityMismatchAllowsFrameworkConventions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/okrs/use-kr-drag.ts"), strings.Join([]string{
		"export function useKrDrag(args: DragArgs, props: Props, searchParams: URLSearchParams, next: string, out: Result) {",
		"  const ids = next;",
		"  const rows = props.row;",
		"  return { args, props, searchParams, ids, rows, out };",
		"}",
		"interface DragArgs { id: string }",
		"interface Props { row: string }",
		"interface Result { id: string }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "naming.cardinality-mismatch")
}

func TestQualityMutableGlobalStateIgnoresReactLocalBindings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/claims/claim-classification-fields.tsx"), strings.Join([]string{
		"export function ClaimClassificationFields() {",
		"  let value = '';",
		"  const data = new Map<string, string>();",
		"  data.set('classification_status', value);",
		"  return <div>{value}</div>;",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.mutable-global-state")
}

func TestNamingBooleanNotPredicateAllowsUIPropsAndHandlers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/contracts/contract-detail-classification.tsx"), strings.Join([]string{
		"export function ContractDetailClassification(open: boolean, loading: boolean, active: boolean, pending: boolean, onSave: () => void) {",
		"  if (open && active && !loading && !pending) {",
		"    onSave();",
		"  }",
		"  return <button onClick={onSave}>Save</button>;",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "naming.boolean-not-predicate")
}

func firstFindingForRule(t *testing.T, report codeguard.Report, sectionName string, ruleID string) codeguard.Finding {
	t.Helper()
	for _, section := range report.Sections {
		if section.Name != sectionName {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID == ruleID {
				return finding
			}
		}
	}
	t.Fatalf("rule %q not found in section %q", ruleID, sectionName)
	return codeguard.Finding{}
}
