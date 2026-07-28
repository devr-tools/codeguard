package checks_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestQualityPrecisionAllowsDomainSideEffectAndAdapterOrchestrationNames(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/connectors/src/abuse.ts"), strings.Join([]string{
		"export async function loadAbuseConfig(repo: Repo) {",
		"  const config = await repo.load();",
		"  await repo.update(config);",
		"  return config;",
		"}",
		"export async function evaluateActionAbuse(evaluator: Evaluator, action: Action) {",
		"  const result = await evaluator.evaluate(action);",
		"  await evaluator.record(result);",
		"  return result;",
		"}",
		"export async function maybeAlert(alerts: Alerts, result: Result) {",
		"  if (result.highRisk) await alerts.send(result);",
		"  return result;",
		"}",
		"export async function saveAbuseConfig(repo: Repo, input: Input) {",
		"  const parsed = parseInput(input);",
		"  await repo.save(parsed);",
		"  await repo.audit(parsed);",
		"  return parsed;",
		"}",
		"export async function postBugReportToSlack(slack: Slack, report: Report) {",
		"  const body = formatReport(report);",
		"  await slack.post(body);",
		"  await slack.record(body);",
		"  return body;",
		"}",
		"interface Repo { load(): Promise<unknown>; update(input: unknown): Promise<void>; save(input: unknown): Promise<void>; audit(input: unknown): Promise<void> }",
		"interface Evaluator { evaluate(input: unknown): Promise<Result>; record(input: unknown): Promise<void> }",
		"interface Alerts { send(input: unknown): Promise<void> }",
		"interface Slack { post(input: unknown): Promise<void>; record(input: unknown): Promise<void> }",
		"interface Input { id: string }",
		"interface Action { id: string }",
		"interface Result { highRisk: boolean }",
		"interface Report { id: string }",
		"declare function parseInput(input: Input): unknown;",
		"declare function formatReport(report: Report): unknown;",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	for _, ruleID := range []string{
		"function.hidden-mutation",
		"function.multiple-responsibilities",
		"function.mixed-abstraction-level",
		"quality.mixed-abstraction-levels",
		"smell.feature-envy",
	} {
		assertFindingRuleAbsent(t, report, "Code Quality", ruleID)
	}
}

func TestQualityNamingAllowsCommonConnectorNames(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/connectors/src/normalizers.ts"), strings.Join([]string{
		"export function asRecord(value: unknown): Record<string, unknown> {",
		"  return typeof value === 'object' && value !== null ? value as Record<string, unknown> : {};",
		"}",
		"export function areStickerPlacementsEqual(source: Placement[], keys: string[], thresholds: Record<string, number>, cached: boolean) {",
		"  const value = source.length === keys.length;",
		"  return cached && value && Object.keys(thresholds).length > 0;",
		"}",
		"interface Placement { id: string }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "naming.boolean-not-predicate")
	assertFindingRuleAbsent(t, report, "Code Quality", "naming.cardinality-mismatch")
	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ambiguous-name")
}

func TestQualityDuplicatedKnowledgeSkipsTableAndEnumValueLiterals(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/connectors/src/constants.ts"), strings.Join([]string{
		"export const abuseTableName = 'lmp_abuse_config';",
		"export const auditTableName = 'lmp_abuse_config';",
		"export const volumeOptions = [{ value: 'high_volume' }, { value: 'high_volume' }];",
		"export const avatarOptions = [{ value: 'custom_avatar' }, { value: 'custom_avatar' }];",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.duplicated-knowledge")
}

func TestDefensiveRulesRecognizeValidationAndBoundedReadProofs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/connectors/src/boundary.ts"), strings.Join([]string{
		"export function handleToolAction(payload: Record<string, unknown>) {",
		"  const parsed = parseToolActionPayload(payload);",
		"  return parsed.action;",
		"}",
		"export async function POST(request: Request) {",
		"  const contentLength = Number(request.headers.get('content-length') ?? '0');",
		"  if (contentLength > MAX_UPLOAD_BYTES) throw new Error('upload too large');",
		"  const form = await request.formData();",
		"  return Response.json({ ok: true, form });",
		"}",
		"export async function readBounded(response: Response) {",
		"  const bytes = await response.arrayBuffer();",
		"  if (bytes.byteLength > MAX_RESPONSE_BYTES) throw new Error('response too large');",
		"  return bytes;",
		"}",
		"export function normalizeWebhookUrl(input: string) {",
		"  const url = new URL(input);",
		"  if (url.protocol !== 'https:') throw new Error('invalid protocol');",
		"  return url;",
		"}",
		"function parseToolActionPayload(value: Record<string, unknown>) {",
		"  const result = ToolActionSchema.safeParse(value);",
		"  if (!result.success) throw new Error('invalid payload');",
		"  return result.data;",
		"}",
		"declare const ToolActionSchema: { safeParse(value: unknown): { success: boolean; data: { action: string } } };",
		"declare const MAX_UPLOAD_BYTES: number;",
		"declare const MAX_RESPONSE_BYTES: number;",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.unvalidated-boundary-input")
	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.missing-schema-validation")
	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.missing-resource-limit")
	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.unchecked-external-response")
}

func TestDefensiveRulesRecognizeUploadHelpersAndRouteValidationGuards(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/files/use-new-version-upload.ts"), strings.Join([]string{
		"import { INTERNAL_UPLOAD_MAX_BYTES, validateInternalUploadFile } from './upload-validation';",
		"export async function uploadVersion(file: File) {",
		"  validateInternalUploadFile(file, INTERNAL_UPLOAD_MAX_BYTES);",
		"  await uploadClient.upload(file);",
		"  return file.name;",
		"}",
		"declare const uploadClient: { upload(file: File): Promise<void> };",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "apps/web/app/api/files/route.ts"), strings.Join([]string{
		"import { NextResponse } from 'next/server';",
		"export async function POST(request: Request) {",
		"  const parsed = await parseRequestBody(request);",
		"  if (!parsed.ok) return NextResponse.json({ error: 'invalid' }, { status: 400 });",
		"  const url = new URL(parsed.value.callbackUrl);",
		"  if (url.protocol !== 'https:') return NextResponse.json({ error: 'invalid' }, { status: 400 });",
		"  return NextResponse.json({ ok: true });",
		"}",
		"export async function parseRequestBody(request: Request) {",
		"  const body = await request.json();",
		"  const result = z.safeParse(body);",
		"  if (!result.success) return { ok: false as const };",
		"  return { ok: true as const, value: result.data };",
		"}",
		"export function bearerTokenFrom(request: Request) {",
		"  const header = request.headers.get('authorization');",
		"  if (!header?.startsWith('Bearer ')) return null;",
		"  return header.slice('Bearer '.length);",
		"}",
		"declare const z: { safeParse(value: unknown): { success: boolean; data: { callbackUrl: string } } };",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.missing-resource-limit")
	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.unvalidated-boundary-input")
}

func TestQualityNamingAllowsUIBooleanAndDomainCollectionAliases(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/contracts/filters.tsx"), strings.Join([]string{
		"export function ContractFilters(showAdd: boolean, showDeprioritized: boolean, filtersActive: boolean, matchesFilter: boolean, krs: KeyResult[], docs: Document[], filtered: Contract[], all: Contract[]) {",
		"  return <div>{String(showAdd && showDeprioritized && filtersActive && matchesFilter)}{krs.length}{docs.length}{filtered.length}{all.length}</div>;",
		"}",
		"interface KeyResult { id: string }",
		"interface Contract { id: string }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "naming.boolean-not-predicate")
	assertFindingRuleAbsent(t, report, "Code Quality", "naming.cardinality-mismatch")
}

func TestSwitchOnTypeAllowsCentralizedEnumDisplayMaps(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/contracts/status-labels.ts"), strings.Join([]string{
		"export function labelForContractStatus(status: ContractStatus) {",
		"  switch (status.type) {",
		"    case 'draft': return 'Draft';",
		"    case 'review': return 'In review';",
		"    case 'signed': return 'Signed';",
		"    case 'archived': return 'Archived';",
		"    default: return 'Unknown';",
		"  }",
		"}",
		"export const statusOptions = {",
		"  draft: { label: 'Draft' },",
		"  review: { label: 'In review' },",
		"  signed: { label: 'Signed' },",
		"} as const;",
		"interface ContractStatus { type: string }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "smell.switch-on-type")
}

func TestDefensiveIntegerOverflowSkipsGuardedP2002SequenceRetry(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/api/src/external-id.ts"), strings.Join([]string{
		"export async function allocateExternalId(db: Db) {",
		"  for (let attempt = 0; attempt < 5; attempt++) {",
		"    const count = await db.file.count();",
		"    const externalId = count + 1;",
		"    try {",
		"      return await db.file.create({ data: { externalId } });",
		"    } catch (err: any) {",
		"      if (err.code !== 'P2002') throw err;",
		"    }",
		"  }",
		"  throw new Error('unique external ID collision retry exhausted');",
		"}",
		"interface Db { file: { count(): Promise<number>; create(input: unknown): Promise<unknown> } }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.integer-overflow")
}

func TestDefensiveIntegerArithmeticSplitsSequenceAndMetricContexts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/api/src/external-id.ts"), strings.Join([]string{
		"export async function allocateExternalId(db: Db) {",
		"  const count = await db.file.count();",
		"  const externalId = count + 1;",
		"  return db.file.create({ data: { externalId } });",
		"}",
		"export function recordMetric(stats: Stats, count: number, total: number) {",
		"  const nextTotal = total + count;",
		"  stats.histogram('documents_total').record(nextTotal);",
		"  return nextTotal;",
		"}",
		"interface Db { file: { count(): Promise<number>; create(input: unknown): Promise<unknown> } }",
		"interface Stats { histogram(name: string): { record(value: number): void } }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRulePresent(t, report, "Code Quality", "defensive.sequence-collision-risk")
	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.integer-overflow")
}

func TestDefensiveResourceLimitRecognizesSliceAndCountProofs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/api/src/uploads.ts"), strings.Join([]string{
		"export async function readChunk(request: Request, maxCount: number) {",
		"  const raw = await request.body?.getReader().read();",
		"  const count = raw?.value?.length ?? 0;",
		"  if (count > maxCount) throw new Error('too many bytes');",
		"  return raw?.value?.slice(0, maxCount);",
		"}",
		"export async function uploadPreview(file: File) {",
		"  const preview = file.slice(0, INTERNAL_UPLOAD_MAX_BYTES);",
		"  await uploadClient.upload(preview);",
		"  return preview;",
		"}",
		"declare const INTERNAL_UPLOAD_MAX_BYTES: number;",
		"declare const uploadClient: { upload(file: Blob): Promise<void> };",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.missing-resource-limit")
}
