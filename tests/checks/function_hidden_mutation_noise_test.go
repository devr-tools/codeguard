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

func TestFunctionHiddenMutationStillWarnsForLocalCollaboratorConstruction(t *testing.T) {
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

	assertFindingRulePresent(t, report, "Code Quality", "function.hidden-mutation")
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

func TestFunctionHiddenMutationStillWarnsForReactNativeCollaboratorMutation(t *testing.T) {
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

	assertFindingRulePresent(t, report, "Code Quality", "function.hidden-mutation")
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
