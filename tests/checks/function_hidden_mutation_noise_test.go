package checks_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFunctionHiddenMutationAllowsPureLocalMutationAcrossLanguages(t *testing.T) {
	cases := []struct {
		name     string
		language string
		file     string
		source   []string
	}{
		{
			name:     "typescript local map",
			language: "typescript",
			file:     "packages/api/src/lib/contract-summary/parse.ts",
			source: []string{
				"export function parseSummary(rows: Row[]): Map<string, number> {",
				"  const totals = new Map<string, number>();",
				"  for (const row of rows) {",
				"    totals.set(row.key, row.total);",
				"  }",
				"  return totals;",
				"}",
				"interface Row { key: string; total: number }",
			},
		},
		{
			name:     "javascript local set",
			language: "javascript",
			file:     "apps/web/lib/sla.js",
			source: []string{
				"export function buildScopes(events) {",
				"  const scopes = new Set();",
				"  for (const event of events) {",
				"    scopes.add(event.scope);",
				"  }",
				"  return Array.from(scopes);",
				"}",
			},
		},
		{
			name:     "typescript local date cursor",
			language: "typescript",
			file:     "apps/web/lib/calendar.ts",
			source: []string{
				"export function monthBuckets(start: number, end: number) {",
				"  const buckets: Array<{ start: number; label: string }> = [];",
				"  const cursor = new Date(start);",
				"  cursor.setDate(1);",
				"  cursor.setHours(0, 0, 0, 0);",
				"  while (cursor.getTime() < end) {",
				"    const next = new Date(cursor);",
				"    next.setMonth(next.getMonth() + 1);",
				"    buckets.push({ start: cursor.getTime(), label: cursor.toISOString() });",
				"    cursor.setTime(next.getTime());",
				"  }",
				"  return buckets;",
				"}",
			},
		},
		{
			name:     "typescript local date helper clone",
			language: "typescript",
			file:     "apps/web/lib/time-window.ts",
			source: []string{
				"export function range(now: Date) {",
				"  const startOfDay = (d: Date) => {",
				"    const x = new Date(d);",
				"    x.setHours(0, 0, 0, 0);",
				"    return x;",
				"  };",
				"  const startOfWeek = (d: Date) => {",
				"    const x = startOfDay(d);",
				"    x.setDate(x.getDate() - 1);",
				"    return x;",
				"  };",
				"  return startOfWeek(now);",
				"}",
			},
		},
		{
			name:     "typescript browser canvas builder",
			language: "typescript",
			file:     "apps/web/components/forms/attorney/avatar-image.ts",
			source: []string{
				"export async function fileToCompressedDataUrl(file: File): Promise<string> {",
				"  const img = new Image();",
				"  const canvas = document.createElement('canvas');",
				"  canvas.width = 10;",
				"  const ctx = canvas.getContext('2d');",
				"  if (!ctx) return '';",
				"  ctx.drawImage(img, 0, 0);",
				"  return canvas.toDataURL('image/jpeg', 0.85);",
				"}",
			},
		},
		{
			name:     "typescript url segment name",
			language: "typescript",
			file:     "apps/web/app/admin/settings/_components/governance/use-document-form.ts",
			source: []string{
				"export function nameFromUrl(target: string) {",
				"  try {",
				"    const u = new URL(target);",
				"    const slug = u.pathname.split('/').filter(Boolean).pop() ?? u.host;",
				"    return `${slug} (${new Date().toISOString().slice(0, 10)})`;",
				"  } catch {",
				"    return target.slice(0, 80);",
				"  }",
				"}",
			},
		},
		{
			name:     "typescript search predicate",
			language: "typescript",
			file:     "apps/web/app/admin/cases/_components/case-filters.ts",
			source: []string{
				"export function matchesSearch(m: MatterRow, q: string): boolean {",
				"  return m.title.toLowerCase().includes(q) ||",
				"    (m.tags ?? []).some((t) => t.toLowerCase().includes(q)) ||",
				"    m.assignee.name.toLowerCase().includes(q) ||",
				"    m.externalId.toLowerCase().includes(q);",
				"}",
				"interface MatterRow { title: string; tags?: string[]; assignee: { name: string }; externalId: string }",
			},
		},
		{
			name:     "tsx value cells formatter",
			language: "typescript",
			file:     "apps/web/app/admin/contracts/_components/contracts-cells.tsx",
			source: []string{
				"export function valueCells(c: ContractRow): Record<string, ReactNode> {",
				"  return {",
				"    value: c.value != null ? <span>{Number(c.value).toLocaleString()}</span> : <Dash />,",
				"    currency: <span>{c.currency}</span>,",
				"  };",
				"}",
				"interface ContractRow { value: number | null; currency: string }",
				"type ReactNode = unknown;",
				"declare function Dash(): ReactNode;",
			},
		},
		{
			name:     "python local list",
			language: "python",
			file:     "packages/auth/src/resolve.py",
			source: []string{
				"def data_derived_filters(rows):",
				"    filters = []",
				"    for row in rows:",
				"        filters.append(row.filter)",
				"    return filters",
			},
		},
		{
			name:     "go local builder",
			language: "go",
			file:     "report.go",
			source: []string{
				"package sample",
				"import \"strings\"",
				"func RenderReport(rows []string) string {",
				"\tvar builder strings.Builder",
				"\tfor _, row := range rows {",
				"\t\tbuilder.WriteString(row)",
				"\t}",
				"\treturn builder.String()",
				"}",
			},
		},
		{
			name:     "cpp local collection",
			language: "cpp",
			file:     "filters.cpp",
			source: []string{
				"#include <set>",
				"std::set<int> BuildFilters() {",
				"  std::set<int> filters = {};",
				"  filters.insert(1);",
				"  return filters;",
				"}",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), strings.Join(tc.source, "\n"))

			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, tc.language))

			assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
			assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
		})
	}
}

func TestFunctionHiddenMutationAllowsBuilderParserAccumulatorNames(t *testing.T) {
	cases := []struct {
		name   string
		file   string
		source []string
	}{
		{
			name: "build copy text with array push",
			file: "apps/web/lib/copy.ts",
			source: []string{
				"export function buildCopyText(claim: Claim) {",
				"  const parts: string[] = [];",
				"  parts.push(claim.id);",
				"  parts.push(claim.status);",
				"  return parts.join('\\n');",
				"}",
				"interface Claim { id: string; status: string }",
			},
		},
		{
			name: "primary cells object array",
			file: "apps/web/app/claims/_components/cells.ts",
			source: []string{
				"export function primaryCells(row: Row) {",
				"  const cells = [];",
				"  cells.push({ key: 'status', value: row.status });",
				"  cells.push({ key: 'owner', value: row.owner });",
				"  return cells;",
				"}",
				"interface Row { status: string; owner: string }",
			},
		},
		{
			name: "calendar buckets map",
			file: "apps/web/lib/calendar.ts",
			source: []string{
				"export function buildCalendarBuckets(events: Event[]) {",
				"  const buckets = new Map<string, Event[]>();",
				"  for (const event of events) {",
				"    const day = event.day;",
				"    if (!buckets.has(day)) buckets.set(day, []);",
				"    buckets.get(day)!.push(event);",
				"  }",
				"  return buckets;",
				"}",
				"interface Event { day: string }",
			},
		},
		{
			name: "local patch object assign",
			file: "packages/api/src/lib/portal-field-map.ts",
			source: []string{
				"export function answersToMatterPatch(answers: Record<string, string>) {",
				"  const patch: Record<string, unknown> = {};",
				"  for (const [key, value] of Object.entries(answers)) {",
				"    Object.assign(patch, coerce(key, value));",
				"  }",
				"  return patch;",
				"}",
				"declare function coerce(key: string, value: string): Record<string, unknown>;",
			},
		},
		{
			name: "local map entry alias",
			file: "apps/web/lib/filter-match.ts",
			source: []string{
				"export function passesActiveFilters(active: ActiveFilter[]) {",
				"  const byId = new Map<string, ActiveFilter[]>();",
				"  for (const filter of active) {",
				"    const group = byId.get(filter.id);",
				"    if (group) group.push(filter);",
				"    else byId.set(filter.id, [filter]);",
				"  }",
				"  return byId.size > 0;",
				"}",
				"interface ActiveFilter { id: string }",
			},
		},
		{
			name: "local url search params",
			file: "apps/web/lib/integration-oauth.ts",
			source: []string{
				"export function redirectToIntegrationProfile(origin: string, params: Record<string, string>) {",
				"  const url = new URL('/profile', origin);",
				"  url.hash = 'slack';",
				"  for (const [key, value] of Object.entries(params)) url.searchParams.set(key, value);",
				"  return url.toString();",
				"}",
			},
		},
		{
			name: "local crypto hmac builder",
			file: "apps/web/lib/oauth-state.ts",
			source: []string{
				"import { createHmac, timingSafeEqual } from 'node:crypto';",
				"export function verifySignedOAuthState(state: string, expected: string) {",
				"  const parts = state.split('.');",
				"  if (parts.length !== 3) return false;",
				"  const [nonce, userId, signature] = parts as [string, string, string];",
				"  const digest = createHmac('sha256', expected).update(`${nonce}.${userId}`).digest('hex');",
				"  return timingSafeEqual(Buffer.from(signature, 'hex'), Buffer.from(digest, 'hex'));",
				"}",
			},
		},
		{
			name: "multiline derived array sort",
			file: "packages/api/src/lib/contract-summary/source.ts",
			source: []string{
				"export async function findBestFile(prisma: PrismaClient, contractId: string) {",
				"  const links = await prisma.fileLink.findMany({ where: { entityId: contractId } });",
				"  const candidates = links",
				"    .map((l) => l.file)",
				"    .filter((f) => f.currentVersion?.storageKey);",
				"  return candidates.sort((a, b) => scoreFile(b) - scoreFile(a)).at(0);",
				"}",
				"declare function scoreFile(file: unknown): number;",
				"interface PrismaClient { fileLink: { findMany(input: unknown): Promise<Array<{ file: unknown }>> } }",
			},
		},
		{
			name: "local scalar score accumulator",
			file: "packages/api/src/lib/contract-summary/source.ts",
			source: []string{
				"export function scoreFile(file: FileRow): number {",
				"  let s = 0;",
				"  if (file.status === 'EXECUTED') s += 100;",
				"  if (/signed/i.test(file.title)) s += 10;",
				"  return s;",
				"}",
				"interface FileRow { status: string; title: string }",
			},
		},
		{
			name: "mapped result chain sort",
			file: "packages/api/src/routers/agent/tools/semantic-tools.ts",
			source: []string{
				"export async function rank(ctx: Context, vec: Float32Array) {",
				"  const all = await ctx.prisma.entityEmbedding.findMany();",
				"  return all",
				"    .map((e) => ({ id: e.id, score: cosine(vec, e.embedding) }))",
				"    .sort((a, b) => b.score - a.score)",
				"    .slice(0, 20);",
				"}",
				"declare function cosine(a: Float32Array, b: Float32Array): number;",
				"interface Context { prisma: { entityEmbedding: { findMany(): Promise<Array<{ id: string; embedding: Float32Array }>> } } }",
			},
		},
		{
			name: "parser local object",
			file: "packages/api/src/lib/contract-summary/parse.ts",
			source: []string{
				"export function parseContractSummary(raw: string) {",
				"  const result: Record<string, string> = {};",
				"  result.raw = raw;",
				"  return result;",
				"}",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), strings.Join(tc.source, "\n"))

			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

			assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
			assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
			assertFindingRuleAbsent(t, report, "Code Quality", "quality.hidden-side-effect")
		})
	}
}

func TestFunctionHiddenMutationAllowsMoreLocalBuilderMutationIdioms(t *testing.T) {
	cases := []struct {
		name   string
		file   string
		source []string
	}{
		{
			name: "contracts filters set and sort",
			file: "apps/web/app/contracts/contracts-filters.ts",
			source: []string{
				"export function buildContractFilters(rows: Contract[]) {",
				"  const statuses = new Set<string>();",
				"  const defs: Array<{ value: string }> = [];",
				"  for (const row of rows) statuses.add(row.status);",
				"  for (const status of statuses) defs.push({ value: status });",
				"  defs.sort((a, b) => a.value.localeCompare(b.value));",
				"  return defs;",
				"}",
				"interface Contract { status: string }",
			},
		},
		{
			name: "split pop parser",
			file: "packages/api/src/parse.ts",
			source: []string{
				"export function parseFileExtension(name: string) {",
				"  return name.split('.').pop() ?? '';",
				"}",
			},
		},
		{
			name: "cheerio cleanup",
			file: "packages/api/src/clean-html.ts",
			source: []string{
				"import * as cheerio from 'cheerio';",
				"export function cleanHtml(html: string) {",
				"  const $ = cheerio.load(html);",
				"  $('script').remove();",
				"  $('[style]').removeAttr('style');",
				"  return $.html();",
				"}",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), strings.Join(tc.source, "\n"))

			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

			assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
			assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
		})
	}
}

func TestFunctionHiddenMutationStillWarnsForCollaboratorMutationWithLocalPayload(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mutation.ts"), strings.Join([]string{
		"export function prepareDigest(repo: Repository, message: Message) {",
		"  const payload = new Map<string, string>();",
		"  payload.set('subject', message.subject);",
		"  repo.save(payload);",
		"  return payload;",
		"}",
		"interface Repository { save(input: unknown): void }",
		"interface Message { subject: string }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRulePresent(t, report, "Code Quality", "function.hidden-mutation")
}

func TestFunctionHiddenMutationRetainsUnresolvedLocalCollaboratorConstruction(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mutation.ts"), strings.Join([]string{
		"export function prepareDigest(input: Input) {",
		"  const repo = getRepository();",
		"  repo.save(input);",
		"  return input;",
		"}",
		"function getRepository(): Repository { throw new Error('test'); }",
		"interface Repository { save(input: Input): void }",
		"interface Input { id: string }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
	assertUnresolvedDiagnosticCount(t, report, "typescript", "1")
}

func TestFunctionHiddenMutationAllowsNextRouteHandlerNames(t *testing.T) {
	cases := []struct {
		name   string
		file   string
		source []string
	}{
		{
			name: "get",
			file: "apps/web/app/api/files/[versionId]/download/route.ts",
			source: []string{
				"export async function GET(request: Request) {",
				"  await audit.save(request);",
				"  return Response.json({ ok: true });",
				"}",
			},
		},
		{
			name: "post",
			file: "apps/web/app/api/files/upload/route.ts",
			source: []string{
				"export async function POST(request: Request) {",
				"  await storage.upload(request);",
				"  await audit.record(request);",
				"  return Response.json({ ok: true });",
				"}",
			},
		},
		{
			name: "patch",
			file: "apps/web/app/api/users/[id]/route.ts",
			source: []string{
				"export async function PATCH(request: Request) {",
				"  await users.update(await request.json());",
				"  return Response.json({ ok: true });",
				"}",
			},
		},
		{
			name: "delete",
			file: "apps/web/app/api/users/[id]/route.ts",
			source: []string{
				"export async function DELETE(request: Request) {",
				"  await users.remove(request);",
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

			assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
		})
	}
}

func TestFunctionHiddenMutationAllowsNestJSControllerBoundaries(t *testing.T) {
	cases := []struct {
		name string
		file string
	}{
		{name: "controller", file: "apps/api/src/users/users.controller.ts"},
		{name: "resolver", file: "apps/api/src/users/users.resolver.ts"},
		{name: "gateway", file: "apps/api/src/events/events.gateway.ts"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), strings.Join([]string{
				"import { Body, Controller, Get, Post } from '@nestjs/common';",
				"@Controller('users')",
				"export class UsersController {",
				"  constructor(private readonly usersService: UsersService) {}",
				"  @Get(':id')",
				"  async getUser(data: RequestDto) {",
				"    await this.usersService.recordAccess(data);",
				"    return this.usersService.findOne(data.id);",
				"  }",
				"  @Post()",
				"  async submit(@Body() value: CreateUserDto) {",
				"    await this.usersService.create(value);",
				"    return { ok: true };",
				"  }",
				"}",
				"interface RequestDto { id: string }",
				"interface CreateUserDto { id: string }",
				"interface UsersService { recordAccess(input: RequestDto): Promise<void>; findOne(id: string): Promise<unknown>; create(input: CreateUserDto): Promise<void> }",
			}, "\n"))

			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

			assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
			assertFindingRuleAbsent(t, report, "Code Quality", "quality.hidden-side-effect")
			assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
		})
	}
}

func TestFunctionHiddenMutationDoesNotBubbleNestedUICallbackMutationToComponent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/claims/_components/claim-card.tsx"), strings.Join([]string{
		"export function ClaimCard(repo: Repository, claim: Claim) {",
		"  async function onSave() {",
		"    await repo.save(claim);",
		"  }",
		"  return <button onClick={onSave}>Save</button>;",
		"}",
		"interface Repository { save(input: unknown): Promise<void> }",
		"interface Claim { id: string }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
}

func TestFunctionHiddenMutationAllowsReactNativeLocalStateAndHandlers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/mobile/src/screens/ProfileScreen.tsx"), strings.Join([]string{
		"import { FlatList, Pressable, Text, View } from 'react-native';",
		"import { useState } from 'react';",
		"export function ProfileScreen({ users }: Props) {",
		"  const [selected, setSelected] = useState<string | null>(null);",
		"  const visibleIds = new Set<string>();",
		"  users.forEach((item) => visibleIds.add(item.id));",
		"  function handlePress(value: string) {",
		"    setSelected(value);",
		"  }",
		"  return <View><FlatList data={users} renderItem={({ item }) => <Pressable onPress={() => handlePress(item.id)}><Text>{item.name}</Text></Pressable>} /></View>;",
		"}",
		"interface Props { users: Array<{ id: string; name: string }> }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
}

func TestFunctionHiddenMutationAllowsReactComponentsAndHooksAsBoundaries(t *testing.T) {
	cases := []struct {
		name     string
		language string
		file     string
		source   []string
	}{
		{
			name:     "uppercase react component",
			language: "typescript",
			file:     "apps/web/app/claims/_components/ClaimEditDialog.tsx",
			source: []string{
				"export function ClaimEditDialog(repo: Repository, claim: Claim) {",
				"  repo.save(claim);",
				"  return <button>Save</button>;",
				"}",
				"interface Repository { save(input: Claim): void }",
				"interface Claim { id: string }",
			},
		},
		{
			name:     "react hook orchestrator",
			language: "typescript",
			file:     "apps/web/app/files/useNewVersionUpload.ts",
			source: []string{
				"export function useNewVersionUpload(uploader: Uploader, file: File) {",
				"  uploader.upload(file);",
				"  return { uploading: true };",
				"}",
				"interface Uploader { upload(input: File): void }",
				"interface File { id: string }",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), strings.Join(tc.source, "\n"))

			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, tc.language))

			assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
			assertFindingRuleAbsent(t, report, "Code Quality", "quality.hidden-side-effect")
		})
	}
}

func TestFunctionHiddenMutationReportsUnresolvedReactNativeCollaboratorCapture(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/mobile/src/screens/ProfileScreen.tsx"), strings.Join([]string{
		"import { Pressable, Text } from 'react-native';",
		"export function ProfileScreen(repo: Repository, user: User) {",
		"  function loadUser() {",
		"    repo.save(user);",
		"    return user;",
		"  }",
		"  return <Pressable onPress={loadUser}><Text>{user.name}</Text></Pressable>;",
		"}",
		"interface Repository { save(input: User): void }",
		"interface User { name: string }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
	assertUnresolvedDiagnosticCount(t, report, "typescript", "1")
}

func TestFunctionHiddenMutationAllowsConventionalCommandNames(t *testing.T) {
	cases := []struct {
		name string
	}{
		{name: "submit"},
		{name: "uploadOne"},
		{name: "downloadCsv"},
		{name: "notify"},
		{name: "fetchDigestMessages"},
		{name: "listWorkspaceUsers"},
		{name: "read"},
		{name: "exists"},
		{name: "seedContracts"},
		{name: "applyDiscrepancyMerge"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "commands.ts"), strings.Join([]string{
				"export async function " + tc.name + "(repo: Repository, input: Input) {",
				"  await repo.save(input);",
				"  return repo.find(input.id);",
				"}",
				"interface Repository { save(input: Input): Promise<void>; find(id: string): Promise<Input> }",
				"interface Input { id: string }",
			}, "\n"))

			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

			assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
		})
	}
}

func TestFunctionHiddenMutationAllowsScriptMainEntrypoints(t *testing.T) {
	cases := []struct {
		name string
		file string
	}{
		{name: "script directory", file: "apps/web/scripts/backfill-slack-profiles.ts"},
		{name: "cleanup file", file: "packages/db/prisma/cleanup-phantom-users.ts"},
		{name: "import file", file: "packages/db/prisma/import-drive-policies.ts"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), strings.Join([]string{
				"async function main() {",
				"  await repo.save({ ok: true });",
				"  await notifier.send({ ok: true });",
				"}",
				"main();",
			}, "\n"))

			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

			assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
		})
	}
}
