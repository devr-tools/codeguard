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

func TestQualityAmbiguousNameAllowsReactNativeRenderParams(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/mobile/src/screens/ProfileScreen.tsx"), strings.Join([]string{
		"import { FlatList, Pressable, Text } from 'react-native';",
		"export function ProfileScreen({ users }: Props) {",
		"  function renderItem({ item }: { item: User }) {",
		"    return <Pressable onPress={() => item.onPress(item.id)}><Text>{item.name}</Text></Pressable>;",
		"  }",
		"  function keyExtractor(value: User) {",
		"    return value.id;",
		"  }",
		"  return <FlatList data={users} renderItem={renderItem} keyExtractor={keyExtractor} />;",
		"}",
		"interface User { id: string; name: string; onPress(id: string): void }",
		"interface Props { users: User[] }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ambiguous-name")
}

func TestFunctionMutationRulesAllowUICommandHelperNamesOnlyInUI(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/components/confirm-button.tsx"), strings.Join([]string{
		"export function ConfirmButton() {",
		"  let armed = false;",
		"  function disarm() {",
		"    armed = false;",
		"    return armed;",
		"  }",
		"  function confirmDelete() {",
		"    armed = true;",
		"    return armed;",
		"  }",
		"  return <button onClick={confirmDelete}>{String(disarm())}</button>;",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages/api/src/lib/select-user.ts"), strings.Join([]string{
		"export function selectUser(user: User): User {",
		"  user.selected = true;",
		"  return user;",
		"}",
		"interface User { selected: boolean }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertCodeQualityRuleAbsentForPath(t, report, "function.hidden-mutation", "confirm-button.tsx")
	assertCodeQualityRuleAbsentForPath(t, report, "function.command-query-mix", "confirm-button.tsx")
	assertCodeQualityRulePresentForPathWithMessage(t, report, "function.hidden-mutation", "select-user.ts", "mutates state")
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
		{
			name: "next patch route",
			file: "apps/web/app/api/users/[id]/route.ts",
			source: []string{
				"export async function PATCH(request: Request) {",
				"  await users.update(await request.json());",
				"  return Response.json({ ok: true });",
				"}",
			},
		},
		{
			name: "nest controller",
			file: "apps/api/src/users/users.controller.ts",
			source: []string{
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

func TestQualityAmbiguousNameAllowsNestJSPayloadNames(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/api/src/users/users.controller.ts"), strings.Join([]string{
		"import { Body, Controller, Patch, Query } from '@nestjs/common';",
		"@Controller('users')",
		"export class UsersController {",
		"  constructor(private readonly usersService: UsersService) {}",
		"  @Patch(':id')",
		"  async update(@Body() data: UpdateUserDto, @Query() value: QueryDto) {",
		"    await this.usersService.update(data);",
		"    return this.usersService.find(value);",
		"  }",
		"}",
		"interface UpdateUserDto { id: string }",
		"interface QueryDto { id: string }",
		"interface UsersService { update(input: UpdateUserDto): Promise<void>; find(input: QueryDto): Promise<unknown> }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ambiguous-name")
	assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
	assertFindingRuleAbsent(t, report, "Code Quality", "quality.hidden-side-effect")
}

func TestQualityDuplicatedKnowledgeSkipsDisplayStringsAndIncludesLiteral(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/claims/labels.tsx"), strings.Join([]string{
		"export function Labels() {",
		"  return <div className=\"status status\">Status</div>;",
		"}",
		"export const first = 'invoice_policy_code';",
		"export const second = 'invoice_policy_code';",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	finding := firstDuplicatedKnowledgeFinding(t, report)
	if !strings.Contains(finding.Message, "'invoice_policy_code'") {
		t.Fatalf("expected duplicated literal in message, got %q", finding.Message)
	}
}

func TestQualityDuplicatedKnowledgeSkipsTrivialRepeatedNumbers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/claims/_components/rows.tsx"), strings.Join([]string{
		"export const page = 0;",
		"export const start = 0;",
		"export const second = 2;",
		"export const columns = 2;",
		"export const policyA = 'invoice_policy_code';",
		"export const policyB = 'invoice_policy_code';",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	finding := firstDuplicatedKnowledgeFinding(t, report)
	if strings.Contains(finding.Message, " 0 ") || strings.Contains(finding.Message, " 2 ") {
		t.Fatalf("expected duplicated domain literal instead of trivial number, got %q", finding.Message)
	}
	if !strings.Contains(finding.Message, "'invoice_policy_code'") {
		t.Fatalf("expected duplicated domain literal in message, got %q", finding.Message)
	}
}

func TestQualityDuplicatedKnowledgeSkipsSmallNumbersAndEnumStatusStrings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/claims/statuses.ts"), strings.Join([]string{
		"export const retryAttempts = 3;",
		"export const visibleColumns = 3;",
		"export const pageSize = 25;",
		"export const defaultLimit = 25;",
		"export const statuses = [",
		"  { value: 'CLAIM_APPROVED', label: 'Approved' },",
		"  { value: 'CLAIM_APPROVED', label: 'Approved' },",
		"];",
		"export const premiumAmountCents = 1000;",
		"export const vipAmountCents = 1000;",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	finding := firstDuplicatedKnowledgeFinding(t, report)
	if strings.Contains(finding.Message, "CLAIM_APPROVED") || strings.Contains(finding.Message, "25") || strings.Contains(finding.Message, "3") {
		t.Fatalf("expected strong numeric domain duplicate, got %q", finding.Message)
	}
	if !strings.Contains(finding.Message, "1000") {
		t.Fatalf("expected duplicated money-like numeric literal, got %q", finding.Message)
	}
}

func TestQualityDuplicatedKnowledgeSkipsSentinelsStylesAndUnmarkedEnums(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/claims/constants.ts"), strings.Join([]string{
		"export const teamFilterA = '__team__';",
		"export const teamFilterB = '__team__';",
		"export const baseClass = 'rounded-md border-gray-200';",
		"export const activeClass = 'rounded-md border-gray-200';",
		"export const firstStatus = 'CLAIM_APPROVED';",
		"export const secondStatus = 'CLAIM_APPROVED';",
		"export const premiumAmountCents = 1000;",
		"export const vipAmountCents = 1000;",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	finding := firstDuplicatedKnowledgeFinding(t, report)
	if strings.Contains(finding.Message, "__team__") || strings.Contains(finding.Message, "rounded-md") || strings.Contains(finding.Message, "CLAIM_APPROVED") {
		t.Fatalf("expected only strong domain duplicate, got %q", finding.Message)
	}
	if !strings.Contains(finding.Message, "1000") {
		t.Fatalf("expected duplicated money-like numeric literal, got %q", finding.Message)
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

func TestNamingCardinalityMismatchAllowsCollectionSuffixesAndMapPairs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/okrs/use-kr-drag.ts"), strings.Join([]string{
		"export function buildKrLookup(krIds: string[], entries: Map<string, string>) {",
		"  entries.forEach((v: string, k: string) => {",
		"    console.log(k, v);",
		"  });",
		"  return krIds;",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "naming.cardinality-mismatch")
}

func TestNamingCardinalityMismatchAllowsCommonPluralDomainCollections(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/matters/list.ts"), strings.Join([]string{
		"export function renderSections(answers: Answer[], contracts: Contract[], matters: Matter[], risks: Risk[], sections: Section[], columns: Column[]) {",
		"  return { answers, contracts, matters, risks, sections, columns };",
		"}",
		"interface Answer { id: string }",
		"interface Contract { id: string }",
		"interface Matter { id: string }",
		"interface Risk { id: string }",
		"interface Section { id: string }",
		"interface Column { id: string }",
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

func TestNamingBooleanNotPredicateAllowsHandlersAndResourceIdentifiers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/contracts/actions.ts"), strings.Join([]string{
		"export function configure(onClick: () => void, onSave: () => Promise<void>, databaseSecretRoleArn: string) {",
		"  const selectedArn = databaseSecretRoleArn;",
		"  return { onClick, onSave, selectedArn };",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "naming.boolean-not-predicate")
}

func TestNamingBooleanNotPredicateAllowsConventionalNonBooleanNames(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/components/actions.tsx"), strings.Join([]string{
		"export function ActionButton(opts: boolean, message: boolean, className: boolean, Icon: boolean, submit: boolean, compare: boolean, parser: boolean, builder: boolean) {",
		"  return <button className={String(className)}>{String(message)}{String(Icon)}{String(opts)}{String(submit)}{String(compare)}{String(parser)}{String(builder)}</button>;",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "naming.boolean-not-predicate")
}

func TestFunctionCommandQueryMixAllowsLocalBuilderMutation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/lib/filters.ts"), strings.Join([]string{
		"export function buildAvailableFilters(rows: Row[]) {",
		"  const data = new Map<string, string>();",
		"  for (const row of rows) {",
		"    data.set(row.id, row.label);",
		"  }",
		"  return data;",
		"}",
		"interface Row { id: string; label: string }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
	assertFindingRuleAbsent(t, report, "Code Quality", "quality.hidden-side-effect")
}

func TestFunctionCommandQueryMixAllowsAPICommandReturningResult(t *testing.T) {
	cases := []struct {
		name   string
		source []string
	}{
		{
			name: "createNewFile",
			source: []string{
				"export async function createNewFile(repo: Repo, input: Input) {",
				"  const file = await repo.create(input);",
				"  await repo.save(file);",
				"  return file;",
				"}",
			},
		},
		{
			name: "upsertEmbedding",
			source: []string{
				"export async function upsertEmbedding(repo: Repo, input: Input) {",
				"  const embedding = await repo.upsert(input);",
				"  await repo.record(embedding);",
				"  return embedding;",
				"}",
			},
		},
		{
			name: "notify",
			source: []string{
				"export async function notify(channel: Channel, message: Message) {",
				"  const delivery = await channel.send(message);",
				"  await channel.record(delivery);",
				"  return delivery;",
				"}",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "packages/api/src/files/actions.ts"), strings.Join(append(tc.source, []string{
				"interface Repo { create(input: Input): Promise<unknown>; save(input: unknown): Promise<void>; upsert(input: Input): Promise<unknown>; record(input: unknown): Promise<void> }",
				"interface Channel { send(input: Message): Promise<unknown>; record(input: unknown): Promise<void> }",
				"interface Input { id: string }",
				"interface Message { id: string }",
			}...), "\n"))

			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

			assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
			assertFindingRuleAbsent(t, report, "Code Quality", "quality.hidden-side-effect")
		})
	}
}

func TestReactNativeScreenAllowsUIBooleanAndLocalCollections(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/mobile/src/screens/ClaimsScreen.tsx"), strings.Join([]string{
		"import { FlatList, Pressable, Text } from 'react-native';",
		"export function ClaimsScreen(open: boolean, loading: boolean, onPress: () => void, krIds: string[], rows: Row[]) {",
		"  const ids = new Set<string>();",
		"  rows.forEach((item) => ids.add(item.id));",
		"  const data = Array.from(ids);",
		"  if (open && !loading) {",
		"    onPress();",
		"  }",
		"  return <FlatList data={data} renderItem={({ item }) => <Pressable onPress={onPress}><Text>{item}</Text></Pressable>} />;",
		"}",
		"interface Row { id: string }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "naming.boolean-not-predicate")
	assertFindingRuleAbsent(t, report, "Code Quality", "naming.cardinality-mismatch")
	assertFindingRuleAbsent(t, report, "Code Quality", "quality.mutable-global-state")
	assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
}

func TestUISmellAndOverflowRulesSkipMappingHelpers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/app/claims/_components/claim-map.tsx"), strings.Join([]string{
		"export function renderClaimRows(props: Props) {",
		"  const width = props.table.columns.length * props.theme.spacing.size;",
		"  const label = props.claim.owner.profile.department.name.toUpperCase();",
		"  return <span>{label}{width}</span>;",
		"}",
		"interface Props { table: any; theme: any; claim: any }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "smell.message-chain")
	assertFindingRuleAbsent(t, report, "Code Quality", "smell.feature-envy")
	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.integer-overflow")
}

func TestUISmellAndOverflowRulesSkipReactNativeRenderingHelpers(t *testing.T) {
	cases := []struct {
		name string
		file string
		body []string
	}{
		{
			name: "screen mapping helper",
			file: "apps/mobile/src/screens/claims/claimRows.ts",
			body: []string{
				"export function collectClaimRows(props: Props) {",
				"  const width = props.route.params.claim.items.length * props.theme.spacing.medium;",
				"  const label = props.route.params.claim.owner.profile.department.name.toUpperCase();",
				"  return props.route.params.claim.items.map((item) => ({",
				"    id: item.id,",
				"    title: label,",
				"    width,",
				"  }));",
				"}",
				"interface Props { route: any; theme: any }",
			},
		},
		{
			name: "native style helper",
			file: "apps/mobile/src/components/ClaimCard.native.ts",
			body: []string{
				"export function buildClaimCardStyles(theme: Theme, props: Props) {",
				"  const width = props.layout.window.size.width * theme.spacing.medium;",
				"  const color = props.route.params.claim.owner.profile.department.color;",
				"  return { width, color, padding: theme.spacing.small };",
				"}",
				"interface Theme { spacing: any }",
				"interface Props { layout: any; route: any }",
			},
		},
		{
			name: "tsx react native component",
			file: "apps/mobile/src/screens/claims/ClaimScreen.tsx",
			body: []string{
				"import { View, Text } from 'react-native';",
				"export function ClaimScreen(props: Props) {",
				"  const width = props.route.params.claim.items.length * props.theme.spacing.medium;",
				"  const label = props.route.params.claim.owner.profile.department.name.toUpperCase();",
				"  return <View><Text>{label}{width}</Text></View>;",
				"}",
				"interface Props { route: any; theme: any }",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), strings.Join(tc.body, "\n"))

			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

			assertFindingRuleAbsent(t, report, "Code Quality", "smell.message-chain")
			assertFindingRuleAbsent(t, report, "Code Quality", "smell.feature-envy")
			assertFindingRuleAbsent(t, report, "Code Quality", "defensive.integer-overflow")
		})
	}
}

func TestStructuralSmellsSkipAPIConfigTraversalAndDTOBuilders(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/api/src/contracts/mappers.ts"), strings.Join([]string{
		"export function buildContractDto(row: Row) {",
		"  return {",
		"    id: row.contract.version.current.owner.profile.id,",
		"    ownerName: row.contract.version.current.owner.profile.name,",
		"    ownerEmail: row.contract.version.current.owner.profile.email,",
		"    status: row.contract.version.current.status.code,",
		"    updatedAt: row.contract.version.current.timestamps.updatedAt,",
		"  };",
		"}",
		"export function readApiResponse(response: ApiResponse) {",
		"  return response.data?.claim?.owner?.profile?.department?.name;",
		"}",
		"export function readConfig(config: AppConfig) {",
		"  return config.services.api.endpoints.claims.primary.url;",
		"}",
		"interface Row { contract: any }",
		"interface ApiResponse { data?: any }",
		"interface AppConfig { services: any }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "smell.feature-envy")
	assertFindingRuleAbsent(t, report, "Code Quality", "smell.message-chain")
}

func TestStructuralSmellsSkipSerializerPrismaPromptAndAdapterHelpers(t *testing.T) {
	cases := []struct {
		name   string
		file   string
		source []string
	}{
		{
			name: "url search params",
			file: "packages/api/src/search/params.ts",
			source: []string{
				"export function readSearchParams(request: Request) {",
				"  const params = new URLSearchParams(request.url);",
				"  return params.get('team')?.trim()?.toLowerCase()?.replaceAll(' ', '-');",
				"}",
			},
		},
		{
			name: "prisma include select",
			file: "packages/api/src/contracts/query.ts",
			source: []string{
				"export const contractQuery = {",
				"  include: { owner: { select: { profile: { select: { department: { select: { name: true } } } } } } },",
				"};",
			},
		},
		{
			name: "csv serializer",
			file: "packages/api/src/contracts/export.ts",
			source: []string{
				"export function serializeContractCsv(row: Row) {",
				"  return [row.contract.version.current.owner.profile.name, row.contract.version.current.status.code, row.contract.version.current.timestamps.updatedAt].join(',');",
				"}",
				"interface Row { contract: any }",
			},
		},
		{
			name: "prompt adapter",
			file: "packages/api/src/ai/prompt-adapter.ts",
			source: []string{
				"export function buildClaimPrompt(claim: Claim) {",
				"  return {",
				"    title: claim.case.owner.profile.name,",
				"    email: claim.case.owner.profile.email,",
				"    status: claim.case.status.current.code,",
				"    due: claim.case.timeline.current.dueAt,",
				"    team: claim.case.owner.team.name,",
				"  };",
				"}",
				"interface Claim { case: any }",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), strings.Join(tc.source, "\n"))

			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

			assertFindingRuleAbsent(t, report, "Code Quality", "smell.message-chain")
			assertFindingRuleAbsent(t, report, "Code Quality", "smell.feature-envy")
		})
	}
}

func TestSmellAndOverflowRulesStillFlagNonUIProductionCode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages/domain/src/account-risk.ts"), strings.Join([]string{
		"export function scoreCustomer(customer: Customer, count: number) {",
		"  const totalBytes = count * 4096;",
		"  const score = new Uint8Array(totalBytes).byteLength;",
		"  const code = customer.profile.address.country.region.zone.owner.name.toUpperCase();",
		"  return customer.profile.name + customer.profile.email + customer.account.region + customer.account.plan + customer.account.status + code + score;",
		"}",
		"interface Customer { profile: any; account: any }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertStructuralSmellPresent(t, report, "smell.message-chain")
	assertStructuralSmellPresent(t, report, "smell.feature-envy")
	assertStructuralSmellPresent(t, report, "defensive.integer-overflow")
}

func firstDuplicatedKnowledgeFinding(t *testing.T, report codeguard.Report) codeguard.Finding {
	t.Helper()
	for _, section := range report.Sections {
		if section.Name != "Code Quality" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID == "quality.duplicated-knowledge" {
				return finding
			}
		}
	}
	t.Fatalf("rule %q not found in section %q", "quality.duplicated-knowledge", "Code Quality")
	return codeguard.Finding{}
}
