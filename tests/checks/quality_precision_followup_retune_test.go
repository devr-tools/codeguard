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
	writeFile(t, filepath.Join(dir, "packages/integrations/src/embeddings/vector.ts"), strings.Join([]string{
		"export function cosine(a: Float32Array, b: Float32Array) {",
		"  if (a.length !== b.length || a.length === 0) return 0;",
		"  let dot = 0;",
		"  for (let i = 0; i < a.length; i++) dot += a[i] * b[i];",
		"  return dot;",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages/integrations/src/crypto/envelope.ts"), strings.Join([]string{
		"export function validateHex(ivHex: string, tagHex: string) {",
		"  return ivHex.length === IV_LEN * 2 && tagHex.length === TAG_LEN * 2;",
		"}",
		"declare const IV_LEN: number;",
		"declare const TAG_LEN: number;",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRulePresent(t, report, "Code Quality", "defensive.integer-overflow")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.integer-overflow", "external-id-retry.ts")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.integer-overflow", "seed-contracts.ts")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.integer-overflow", "date-buckets.ts")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.integer-overflow", "vector.ts")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.integer-overflow", "envelope.ts")
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

func TestDefensiveInvalidStateSkipsUIViewModels(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/settings/_components/users/user-types.ts"), strings.Join([]string{
		"export interface UserToolbarState {",
		"  loading: boolean;",
		"  open: boolean;",
		"  status: string;",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages/api/src/domain/order-state.ts"), strings.Join([]string{
		"export interface OrderState {",
		"  active: boolean;",
		"  deleted: boolean;",
		"  status: string;",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertCodeQualityRuleAbsentForPath(t, report, "defensive.invalid-state-representable", "apps/web/app/settings/_components/users/user-types.ts")
	assertCodeQualityRulePresentForPathWithMessage(t, report, "defensive.invalid-state-representable", "packages/api/src/domain/order-state.ts", "impossible combinations")
}

func TestErrorRetryableNotDistinguishedSkipsUIRetryControls(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/error.tsx"), strings.Join([]string{
		"export default function ErrorBoundary({ reset }: { reset(): void }) {",
		"  return <button onClick={() => reset()}>Try again</button>;",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages/api/src/lib/retry.ts"), strings.Join([]string{
		"export async function retryWrite(job: Job) {",
		"  try {",
		"    return await job.run();",
		"  } catch (err) {",
		"    return job.retry();",
		"  }",
		"}",
		"interface Job { run(): Promise<unknown>; retry(): Promise<unknown> }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertCodeQualityRuleAbsentForPath(t, report, "error.retryable-not-distinguished", "apps/web/app/error.tsx")
	assertCodeQualityRulePresentForPathWithMessage(t, report, "error.retryable-not-distinguished", "packages/api/src/lib/retry.ts", "retry path")
}

func TestCommandQueryMixSkipsScriptMainEntrypoints(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/db/prisma/seed-marketing-claims.ts"), strings.Join([]string{
		"export async function main() {",
		"  await db.claim.create({ data: { id: 'claim' } });",
		"  return { ok: true };",
		"}",
		"declare const db: { claim: { create(input: unknown): Promise<unknown> } };",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
}

func TestFunctionMutationRulesAllowDomainActionVerbBoundaries(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/api/src/routers/risk-actions.ts"), strings.Join([]string{
		"export async function captureSnapshotFor(repo: Repo, input: Input) {",
		"  await repo.create(input);",
		"  return input;",
		"}",
		"export async function transferRiskEntityLinks(repo: Repo, input: Input) {",
		"  await repo.update(input);",
		"  return input;",
		"}",
		"export async function linkRequestAttachments(repo: Repo, input: Input) {",
		"  await repo.create(input);",
		"  return input;",
		"}",
		"interface Repo { create(input: Input): Promise<void>; update(input: Input): Promise<void> }",
		"interface Input { id: string }",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "apps/web/scripts/backfill-counterparty-kinds.ts"), strings.Join([]string{
		"export async function classifyCounterparty(client: Client, value: Value) {",
		"  await client.create(value);",
		"  return value;",
		"}",
		"interface Client { create(input: Value): Promise<void> }",
		"interface Value { id: string }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
	assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
}

func TestBooleanRulesSkipObjectOptionsScriptsAndPredicateWords(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/api/src/lib/summaries.ts"), strings.Join([]string{
		"export function summarize(input: Input, opts: { includeDrafts: boolean }) {",
		"  return opts.includeDrafts ? input.title : input.id;",
		"}",
		"export function emailDomainAllowed(domain: string): boolean {",
		"  return domain.endsWith('.com');",
		"}",
		"export function sameGroups(a: string[], b: string[]): boolean {",
		"  return a.length === b.length;",
		"}",
		"export function looksImportant(value: string): boolean {",
		"  return value.includes('!');",
		"}",
		"interface Input { id: string; title: string }",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages/db/scripts/import-lawvu-xlsx.ts"), strings.Join([]string{
		"export function runImport(apply: boolean, dryRun: boolean) {",
		"  return apply && !dryRun;",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.boolean-argument")
	assertFindingRuleAbsent(t, report, "Code Quality", "naming.boolean-not-predicate")
}

func TestDefensiveNullAssumptionCreditsTypeScriptNarrowing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/api/src/lib/null-narrowing.ts"), strings.Join([]string{
		"export function fromNullableString(summary: string | null | undefined) {",
		"  const trimmed = typeof summary === 'string' ? summary.trim() : '';",
		"  return trimmed;",
		"}",
		"export function fromNullableDate(value: Date | null) {",
		"  if (!value) return null;",
		"  return value.toISOString();",
		"}",
		"export function fromNullableName(name: string | null | undefined) {",
		"  if (!name) {",
		"    return null;",
		"  }",
		"  const narrowed = name;",
		"  return narrowed.toLowerCase();",
		"}",
		"export function fromNullableElements(recipientIds: (string | null | undefined)[]) {",
		"  return recipientIds.filter((id): id is string => !!id).map((id) => id.toUpperCase());",
		"}",
		"export function fromNullableField(row: { status: string | null; title: string }) {",
		"  return row.status === 'ACTIVE' ? row.title : null;",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertCodeQualityRuleAbsentForPath(t, report, "defensive.null-assumption", "null-narrowing.ts:2")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.null-assumption", "null-narrowing.ts:7")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.null-assumption", "null-narrowing.ts:13")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.null-assumption", "null-narrowing.ts:16")
}

func TestDefensiveBoundaryInputSkipsTypedInternalDTOs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/api/src/routers/internal-dtos.ts"), strings.Join([]string{
		"export function buildMyDayResult(inputs: MyDayInputs) {",
		"  return { count: inputs.myRequests.length };",
		"}",
		"export function createPrdAnalysis(request: PrdAnalysisRequest) {",
		"  return { title: request.title, body: request.prdContent };",
		"}",
		"export function consumeBoundaryInput(input: unknown) {",
		"  return JSON.stringify(input);",
		"}",
		"interface MyDayInputs { myRequests: unknown[] }",
		"interface PrdAnalysisRequest { title: string; prdContent: string }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertCodeQualityRuleAbsentForPath(t, report, "defensive.unvalidated-boundary-input", "internal-dtos.ts:1")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.unvalidated-boundary-input", "internal-dtos.ts:4")
	assertCodeQualityRulePresentForPathWithMessage(t, report, "defensive.unvalidated-boundary-input", "internal-dtos.ts:7", "boundary input")
}

func TestInvalidStateRepresentableSkipsImportDTOContainers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/db/prisma/import-types.ts"), strings.Join([]string{
		"export interface NotifyArgs {",
		"  severity?: 'info' | 'warn' | 'urgent';",
		"  body?: string | null;",
		"}",
		"interface ParsedRowPart1 {",
		"  state?: string | null;",
		"  title: string;",
		"}",
		"export interface RowContext {",
		"  dryRun: boolean;",
		"  verbose: boolean;",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages/api/src/lib/email-result.ts"), strings.Join([]string{
		"export interface SendEmailResult {",
		"  ok: boolean;",
		"  disabled?: boolean;",
		"  error?: string;",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertCodeQualityRuleAbsentForPath(t, report, "defensive.invalid-state-representable", "import-types.ts:1")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.invalid-state-representable", "import-types.ts:5")
	assertCodeQualityRuleAbsentForPath(t, report, "defensive.invalid-state-representable", "import-types.ts:9")
	assertCodeQualityRulePresentForPathWithMessage(t, report, "defensive.invalid-state-representable", "email-result.ts:1", "impossible combinations")
}

func TestExceptionControlFlowSkipsValidationThrows(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/api/src/routers/validation-errors.ts"), strings.Join([]string{
		"export function validateSourceFile(file: File | null) {",
		"  if (!file) throw new TRPCError({ code: 'BAD_REQUEST', message: 'Source file not found' });",
		"  return file.id;",
		"}",
		"export function requireProfile(profile: Profile | null) {",
		"  if (!profile) throw new AppError('Attorney not found');",
		"  return profile.id;",
		"}",
		"export function parseTokenResponse(payload: unknown) {",
		"  if (!payload || typeof payload !== 'object') throw new Error('token exchange returned invalid JSON');",
		"  return payload;",
		"}",
		"export function findUser(userId: string | null) {",
		"  if (!userId) throw new Exception('not found');",
		"  return userId;",
		"}",
		"interface File { id: string }",
		"interface Profile { id: string }",
		"declare class TRPCError extends Error { constructor(input: unknown); }",
		"declare class AppError extends Error { constructor(message: string); }",
		"declare class Exception extends Error { constructor(message: string); }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertCodeQualityRuleAbsentForPath(t, report, "error.exception-used-for-control-flow", "validation-errors.ts:2")
	assertCodeQualityRuleAbsentForPath(t, report, "error.exception-used-for-control-flow", "validation-errors.ts:6")
	assertCodeQualityRuleAbsentForPath(t, report, "error.exception-used-for-control-flow", "validation-errors.ts:10")
	assertCodeQualityRulePresentForPathWithMessage(t, report, "error.exception-used-for-control-flow", "validation-errors.ts:14", "ordinary branch control")
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
	writeFile(t, filepath.Join(dir, "packages/db/scripts/row-import.ts"), strings.Join([]string{
		"export async function importRows(rows: Array<{ lawvuId?: string }>) {",
		"  let missingLawVuId = 0;",
		"  for (const row of rows) {",
		"    if (!row.lawvuId) {",
		"      missingLawVuId++;",
		"      continue;",
		"    }",
		"    await save(row.lawvuId);",
		"  }",
		"  console.error(`missing LawVu ID: ${missingLawVuId}`);",
		"}",
		"declare function save(id: string): Promise<void>;",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRulePresent(t, report, "Code Quality", "error.partial-failure-hidden")
	assertCodeQualityRuleAbsentForPath(t, report, "error.partial-failure-hidden", "digest-fetch.ts:1")
	assertCodeQualityRuleAbsentForPath(t, report, "error.partial-failure-hidden", "row-import.ts")
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
			if finding.RuleID == ruleID && codeQualityLocationMatches(location, pathFragment) {
				t.Fatalf("section %q unexpectedly contains rule %q at %s: %s", "Code Quality", ruleID, location, finding.Message)
			}
		}
		return
	}
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
			if finding.RuleID != ruleID || !codeQualityLocationMatches(location, pathFragment) {
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

func codeQualityLocationMatches(location string, fragment string) bool {
	if strings.Contains(fragment, ":") {
		return strings.HasSuffix(location, fragment)
	}
	return strings.Contains(location, fragment)
}
