package checks_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestDefensiveIntegerOverflowFollowupRetunes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/api/src/routers/external-id-retry.ts"), strings.Join([]string{
		"export async function createEntity(db: Db) {",
		"  return withExternalIdRetry(async () => {",
		"    const count = await db.entity.count();",
		"    const externalId = count + 1;",
		"    return db.entity.create({ data: { externalId } });",
		"  }, { code: 'P2002', field: 'externalId' });",
		"}",
		"declare function withExternalIdRetry<T>(fn: () => Promise<T>, opts: { code: 'P2002'; field: 'externalId' }): Promise<T>;",
		"interface Db { entity: { count(): Promise<number>; create(input: unknown): Promise<unknown> } }",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages/db/scripts/seed-contracts.ts"), strings.Join([]string{
		"export async function seedExternalIds(db: Db) {",
		"  const count = await db.contract.count();",
		"  const externalId = count + 1;",
		"  return db.contract.create({ data: { externalId } });",
		"}",
		"interface Db { contract: { count(): Promise<number>; create(input: unknown): Promise<unknown> } }",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "apps/web/lib/date-buckets.ts"), strings.Join([]string{
		"export function buildDateBuckets(dayCount: number) {",
		"  const nextBucket = dayCount + 1;",
		"  return `calendar bucket ${nextBucket}`;",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages/api/src/lib/unsafe-size.ts"), strings.Join([]string{
		"export function allocateBuffer(count: number, size: number) {",
		"  const totalBytes = count * size;",
		"  return new Uint8Array(totalBytes);",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRulePresent(t, report, "Code Quality", "defensive.integer-overflow")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.integer-overflow", "external-id-retry.ts")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.integer-overflow", "seed-contracts.ts")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.integer-overflow", "date-buckets.ts")
	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.sequence-collision-risk")
}

func TestDefensiveBoundsAssumptionDistinguishesDictionaryFromSequenceAccess(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/api/src/lib/field-map.ts"), strings.Join([]string{
		"export function portalFieldLabel(fieldMap: Record<string, string>, name: string) {",
		"  return fieldMap[name] ?? 'Unknown';",
		"}",
		"export function envValue(name: string) {",
		"  return process.env[name];",
		"}",
		"export function firstSegment(segments: string[]) {",
		"  return segments[0];",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRulePresent(t, report, "Code Quality", "defensive.bounds-assumption")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.bounds-assumption", "field-map.ts:2")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.bounds-assumption", "field-map.ts:5")
}

func TestDefensiveResourceLimitCreditsPrismaTakeAndContentLengthHelpers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/api/src/routers/search-tools.ts"), strings.Join([]string{
		"export async function searchTools(db: Db, query: string) {",
		"  return db.tool.findMany({ where: { name: { contains: query } }, take: TOOL_SEARCH_LIMIT });",
		"}",
		"export async function uploadDocument(request: Request) {",
		"  assertContentLengthWithinLimit(request, MAX_UPLOAD_BYTES);",
		"  const form = await request.formData();",
		"  return form;",
		"}",
		"declare const TOOL_SEARCH_LIMIT: number;",
		"declare const MAX_UPLOAD_BYTES: number;",
		"declare function assertContentLengthWithinLimit(request: Request, maxBytes: number): void;",
		"interface Db { tool: { findMany(input: unknown): Promise<unknown[]> } }",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages/api/src/routers/unbounded-upload.ts"), strings.Join([]string{
		"export async function uploadRaw(request: Request) {",
		"  const form = await request.formData();",
		"  return form;",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRulePresent(t, report, "Code Quality", "defensive.missing-resource-limit")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.missing-resource-limit", "search-tools.ts")
}

func TestFunctionReturnContractAllowsNullableParserLookupAndExistsHelpers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/integrations/src/slack/slack-client.ts"), strings.Join([]string{
		"export function parseSlackMessage(value: unknown): SlackMessage | null {",
		"  if (!isRecord(value)) return null;",
		"  return { id: String(value.id) };",
		"}",
		"export function lookupDigestMessage(messages: SlackMessage[], id: string): SlackMessage | null {",
		"  return messages.find((message) => message.id === id) ?? null;",
		"}",
		"export function exists(value: unknown) {",
		"  if (value == null) return false;",
		"  return true;",
		"}",
		"interface SlackMessage { id: string }",
		"declare function isRecord(value: unknown): value is Record<string, unknown>;",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "function.inconsistent-return-contract")
}

func TestErrorPartialFailureHiddenCreditsSurfacedDiagnostics(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/integrations/src/gmail/digest-fetch.ts"), strings.Join([]string{
		"export async function fetchDigest(client: Client, ids: string[]) {",
		"  const messages: Message[] = [];",
		"  const diagnostics: string[] = [];",
		"  for (const id of ids) {",
		"    try {",
		"      messages.push(await client.fetch(id));",
		"    } catch (error) {",
		"      diagnostics.push(`failed ${id}: ${String(error)}`);",
		"      continue;",
		"    }",
		"  }",
		"  return { messages, diagnostics };",
		"}",
		"export async function fetchDigestSilently(client: Client, ids: string[]) {",
		"  const messages: Message[] = [];",
		"  for (const id of ids) {",
		"    try {",
		"      messages.push(await client.fetch(id));",
		"    } catch (error) {",
		"      continue;",
		"    }",
		"  }",
		"  return { messages };",
		"}",
		"interface Client { fetch(id: string): Promise<Message> }",
		"interface Message { id: string }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRulePresent(t, report, "Code Quality", "error.partial-failure-hidden")
	assertCodeQualityRuleAbsentForPath(t, report, "error.partial-failure-hidden", "digest-fetch.ts:1")
}

func assertCodeQualityRuleAbsentForPath(t *testing.T, report codeguard.Report, ruleID string, pathFragment string) {
	t.Helper()
	for _, result := range report.Sections {
		if result.Name != "Code Quality" {
			continue
		}
		for _, finding := range result.Findings {
			location := finding.Path
			if finding.Line > 0 {
				location = fmt.Sprintf("%s:%d", location, finding.Line)
			}
			if finding.RuleID == ruleID && strings.Contains(location, pathFragment) {
				t.Fatalf("section %q unexpectedly contains rule %q at %s: %s", "Code Quality", ruleID, location, finding.Message)
			}
		}
		return
	}
	t.Fatalf("section %q not found", "Code Quality")
}
