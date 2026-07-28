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
	assertCodeQualityRulePresentForPathWithMessage(t, report, "defensive.sequence-collision-risk", "external-id-retry.ts", "architectural debt", "database sequence", "transactional allocator")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.sequence-collision-risk", "seed-contracts.ts")
}

func TestFunctionMultipleResponsibilitiesUsesHigherThresholdForUIBoundaries(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/claims/claim-detail.tsx"), strings.Join([]string{
		"export function ClaimDetail({ claim, repo, logger, canEdit }: Props) {",
		"  if (!claim.id) logger.warn('missing id');",
		"  const rows = claim.items.map((item) => ({ label: item.label }));",
		"  const owner = repo.find(claim.ownerId);",
		"  function onSave() { repo.update({ id: claim.id, rows }); }",
		"  return <button disabled={!canEdit} onClick={onSave}>{owner.name}{rows.length}</button>;",
		"}",
		"interface Props { claim: { id: string; ownerId: string; items: Array<{ label: string }> }; repo: Repo; logger: Logger; canEdit: boolean }",
		"interface Repo { find(id: string): { name: string }; update(input: unknown): void }",
		"interface Logger { warn(message: string): void }",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages/api/src/routers/claim-detail.ts"), strings.Join([]string{
		"export async function buildClaimDetail(input: Input, repo: Repo, logger: Logger, sender: Sender) {",
		"  validateInput(input);",
		"  if (!input.id) logger.warn('missing id');",
		"  const claim = await repo.find(input.id);",
		"  const rows = claim.items.map((item) => ({ label: item.label }));",
		"  await repo.update({ id: input.id, rows });",
		"  await sender.send(rows);",
		"  return { claim, rows };",
		"}",
		"interface Input { id: string }",
		"interface Claim { items: Array<{ label: string }> }",
		"interface Repo { find(id: string): Promise<Claim>; update(input: unknown): Promise<void> }",
		"interface Logger { warn(message: string): void }",
		"interface Sender { send(input: unknown): Promise<void> }",
		"declare function validateInput(input: Input): void;",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertCodeQualityRuleAbsentForPath(t, report, "function.multiple-responsibilities", "claim-detail.tsx")
	assertCodeQualityRulePresentForPathWithMessage(t, report, "function.multiple-responsibilities", "packages/api/src/routers/claim-detail.ts", "combines")
}

func TestAllocateExternalIDIsCommandStyleReturningValue(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/api/src/routers/entity-shared.ts"), strings.Join([]string{
		"export async function allocateExternalId(db: Db) {",
		"  return withExternalIdRetry(async () => {",
		"    const count = await db.entity.count();",
		"    const externalId = count + 1;",
		"    return db.entity.create({ data: { externalId } });",
		"  }, { code: 'P2002', field: 'externalId' });",
		"}",
		"declare function withExternalIdRetry<T>(fn: () => Promise<T>, opts: { code: 'P2002'; field: 'externalId' }): Promise<T>;",
		"interface Db { entity: { count(): Promise<number>; create(input: unknown): Promise<unknown> } }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
	assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
	assertCodeQualityRulePresentForPathWithMessage(t, report, "defensive.sequence-collision-risk", "entity-shared.ts", "architectural debt", "database sequence")
	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.integer-overflow")
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
		"export function modelConfig(config: Record<string, string>, modelKey: string) {",
		"  return config[modelKey] ?? 'default';",
		"}",
		"export async function promiseTuple(db: Db, ids: string[]) {",
		"  const [contracts, risks] = await Promise.all([",
		"    db.contract.findMany({ where: { id: { in: ids } }, take: 10 }),",
		"    db.risk.findMany({ where: { id: { in: ids } }, take: 10 }),",
		"  ]);",
		"  return { contracts, risks };",
		"}",
		"export function firstSegment(segments: string[]) {",
		"  return segments[0];",
		"}",
		"interface Db { contract: { findMany(input: unknown): Promise<unknown[]> }; risk: { findMany(input: unknown): Promise<unknown[]> } }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRulePresent(t, report, "Code Quality", "defensive.bounds-assumption")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.bounds-assumption", "field-map.ts:2")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.bounds-assumption", "field-map.ts:5")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.bounds-assumption", "field-map.ts:8")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.bounds-assumption", "field-map.ts:11")
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
		"export async function readRaw(file: File) {",
		"  return Buffer.from(await file.arrayBuffer());",
		"}",
		"export function uploadsRoot() {",
		"  const fromEnv = process.env.LEGAL_OS_UPLOADS_ROOT?.trim();",
		"  return fromEnv ?? '/tmp/uploads';",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRulePresent(t, report, "Code Quality", "defensive.missing-resource-limit")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.missing-resource-limit", "search-tools.ts")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.missing-resource-limit", "unbounded-upload.ts:8")
}

func TestDefensiveBroadeningSkipsUIBoundsAndInternalORMReads(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/components/relationship-tree.tsx"), strings.Join([]string{
		"export function RelationshipTree({ nodes, columns }: Props) {",
		"  const firstNode = nodes[0];",
		"  const selectedColumn = columns[activeIndex];",
		"  return <div>{firstNode?.id}{selectedColumn?.label}</div>;",
		"}",
		"interface Props { nodes: Array<{ id: string }>; columns: Array<{ label: string }>; activeIndex: number }",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "apps/web/components/use-filter-state.tsx"), strings.Join([]string{
		"export function useFilterState(input?: { query?: string }) {",
		"  const query = input?.query ?? '';",
		"  return { query };",
		"}",
		"export function SearchToolbar({ input }: { input?: { query?: string } }) {",
		"  return <button>{input?.query ?? 'Search'}</button>;",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages/api/src/lib/legal-roster.ts"), strings.Join([]string{
		"export async function getLegalRoster(db: Db) {",
		"  const rows = await db.user.findMany({ where: { active: true } });",
		"  return rows.map((row) => ({ id: row.id, name: row.name }));",
		"}",
		"interface Db { user: { findMany(input: unknown): Promise<Array<{ id: string; name: string }>> } }",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages/api/src/routers/search-tools.ts"), strings.Join([]string{
		"export async function searchTools(db: Db, query: string) {",
		"  return db.tool.findMany({ where: { name: { contains: query } } });",
		"}",
		"export async function searchToolsLimited(db: Db, query: string) {",
		"  return db.tool.findMany({ where: { name: { contains: query } }, take: TOOL_SEARCH_LIMIT });",
		"}",
		"declare const TOOL_SEARCH_LIMIT: number;",
		"interface Db { tool: { findMany(input: unknown): Promise<unknown[]> } }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertCodeQualityRuleAbsentForPath(t, report, "defensive.bounds-assumption", "relationship-tree.tsx")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.null-assumption", "use-filter-state.tsx")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.unvalidated-boundary-input", "use-filter-state.tsx")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.missing-resource-limit", "legal-roster.ts")
	assertCodeQualityRulePresentForPathWithMessage(t, report, "defensive.missing-resource-limit", "search-tools.ts", "resource limit")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.missing-resource-limit", "search-tools.ts:4")
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

func TestFunctionReturnContractAllowsExplicitNullableServiceContracts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/integrations/src/anthropic/summarize-content.ts"), strings.Join([]string{
		"export async function buildFileContentBlocks(prompt: string, storageKey?: string | null): Promise<Block[] | null> {",
		"  if (!storageKey) return null;",
		"  return [{ type: 'text', text: prompt }];",
		"}",
		"export async function fetchMatterContext(userId: string): Promise<MatterContext | null> {",
		"  if (!process.env.DAINTREE_MCP_URL) return null;",
		"  return { userId };",
		"}",
		"export async function autoRoute(pillar: string): Promise<string | null> {",
		"  if (!pillar) return null;",
		"  return 'user_1';",
		"}",
		"interface Block { type: string; text: string }",
		"interface MatterContext { userId: string }",
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

func assertCodeQualityRulePresentForPathWithMessage(t *testing.T, report codeguard.Report, ruleID string, pathFragment string, messageParts ...string) {
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
			if finding.RuleID != ruleID || !strings.Contains(location, pathFragment) {
				continue
			}
			loweredMessage := strings.ToLower(finding.Message)
			for _, part := range messageParts {
				if !strings.Contains(loweredMessage, strings.ToLower(part)) {
					t.Fatalf("section %q rule %q at %s message %q does not contain %q", "Code Quality", ruleID, location, finding.Message, part)
				}
			}
			return
		}
		t.Fatalf("section %q missing rule %q for path containing %q", "Code Quality", ruleID, pathFragment)
	}
	t.Fatalf("section %q not found", "Code Quality")
}
