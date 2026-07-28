package checks_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFunctionExcessiveParametersWarnsWithSpecificRule(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "params.go"), strings.Join([]string{
		"package sample",
		"",
		"func CreateUser(name string, email string, plan string, source string) string {",
		"\treturn name + email + plan + source",
		"}",
		"",
	}, "\n"))
	cfg := qualityPrecisionConfig(dir)
	cfg.Checks.QualityRules.MaxParameters = 2

	report := runQualityPrecisionScan(t, cfg)

	assertFindingRulePresent(t, report, "Code Quality", "function.excessive-parameters")
	assertFindingLevel(t, report, "Code Quality", "function.excessive-parameters", "warn")
}

func TestFunctionMixedAbstractionLevelWarnsForInfrastructureInsideOrchestration(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "checkout.go"), strings.Join([]string{
		"package sample",
		"",
		"func Checkout(order Order) error {",
		"\tvalidateOrder(order)",
		"\trows, err := db.Query(\"select 1\")",
		"\tif err != nil {",
		"\t\treturn err",
		"\t}",
		"\tdefer rows.Close()",
		"\treturn persistOrder(order)",
		"}",
		"",
		"type Order struct{}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRulePresent(t, report, "Code Quality", "function.mixed-abstraction-level")
	assertFindingLevel(t, report, "Code Quality", "function.mixed-abstraction-level", "warn")
}

func TestFunctionCommandQueryMixWarnsWhenQueryMutatesState(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "query.go"), strings.Join([]string{
		"package sample",
		"",
		"type Repository interface {",
		"\tFind(string) (User, error)",
		"\tSaveAudit(string) error",
		"}",
		"",
		"type User struct{}",
		"",
		"func GetUser(repo Repository, id string) (User, error) {",
		"\tif err := repo.SaveAudit(id); err != nil {",
		"\t\treturn User{}, err",
		"\t}",
		"\treturn repo.Find(id)",
		"}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRulePresent(t, report, "Code Quality", "function.command-query-mix")
	assertFindingLevel(t, report, "Code Quality", "function.command-query-mix", "warn")
}

func TestFunctionCommandQueryMixAllowsCommandsReturningUsefulResults(t *testing.T) {
	cases := []string{
		"cancelTextEdit",
		"createNewFile",
		"notify",
		"recordJobRun",
		"uploadVersion",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "command.ts"), strings.Join([]string{
				"export async function " + name + "(repo: Repository, input: Input) {",
				"  await repo.save(input);",
				"  return repo.find(input.id);",
				"}",
				"interface Repository { save(input: Input): Promise<void>; find(id: string): Promise<Input> }",
				"interface Input { id: string }",
			}, "\n"))

			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

			assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
			assertFindingRuleAbsent(t, report, "Code Quality", "quality.hidden-side-effect")
		})
	}
}

func TestFunctionHiddenMutationWarnsAcrossLanguages(t *testing.T) {
	cases := []struct {
		name     string
		language string
		file     string
		source   []string
	}{
		{
			name:     "go",
			language: "go",
			file:     "mutation.go",
			source: []string{
				"package sample",
				"type Audit interface { Save(string) error }",
				"func PrepareUser(audit Audit, id string) string {",
				"\taudit.Save(id)",
				"\treturn id",
				"}",
			},
		},
		{
			name:     "python",
			language: "python",
			file:     "mutation.py",
			source: []string{
				"def prepare_user(user):",
				"    user.name = user.name.strip()",
				"    return user",
			},
		},
		{
			name:     "typescript",
			language: "typescript",
			file:     "mutation.ts",
			source: []string{
				"export function prepareUser(user: User): User {",
				"  user.name = user.name.trim();",
				"  return user;",
				"}",
				"interface User { name: string }",
			},
		},
		{
			name:     "javascript",
			language: "javascript",
			file:     "mutation.js",
			source: []string{
				"export function prepareUser(user) {",
				"  user.name = user.name.trim();",
				"  return user;",
				"}",
			},
		},
		{
			name:     "cpp",
			language: "cpp",
			file:     "mutation.cpp",
			source: []string{
				"struct User { int score; };",
				"User PrepareUser(User& user) {",
				"  user.score = user.score + 1;",
				"  return user;",
				"}",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), strings.Join(tc.source, "\n"))

			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, tc.language))

			assertFindingRulePresent(t, report, "Code Quality", "function.hidden-mutation")
		})
	}
}

func TestFunctionHiddenMutationAllowsReactHookLocalStateBoundaries(t *testing.T) {
	cases := []struct {
		name     string
		language string
		file     string
		source   []string
	}{
		{
			name:     "typescript",
			language: "typescript",
			file:     "apps/web/src/hooks/use-filters.ts",
			source: []string{
				"import { useCallback, useReducer, useState } from 'react';",
				"export function useFilters() {",
				"  const [filters, setFilters] = useState({});",
				"  const [state, dispatch] = useReducer(reducer, {});",
				"  function clearFilters() {",
				"    setFilters({});",
				"  }",
				"  const resetFilters = useCallback(() => {",
				"    setFilters({});",
				"    dispatch({ type: 'reset' });",
				"  }, []);",
				"  return { clearFilters, resetFilters, state };",
				"}",
				"function reducer(state: unknown, event: { type: string }) { return state; }",
			},
		},
		{
			name:     "javascript",
			language: "javascript",
			file:     "apps/web/src/hooks/use-filters.js",
			source: []string{
				"import { useCallback, useState } from 'react';",
				"export function useFilters() {",
				"  const [filters, setFilters] = useState({});",
				"  const resetFilters = useCallback(() => {",
				"    setFilters({});",
				"  }, []);",
				"  return { resetFilters, filters };",
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

func TestFunctionHiddenMutationStillWarnsForHiddenPersistenceInsideReactHook(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps/web/src/hooks/use-user.ts"), strings.Join([]string{
		"export function useUser(repo: Repository, user: User) {",
		"  function loadUser() {",
		"    repo.save(user);",
		"    return repo.find(user.id);",
		"  }",
		"  return { loadUser };",
		"}",
		"interface Repository { save(user: User): void; find(id: string): User }",
		"interface User { id: string }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRulePresent(t, report, "Code Quality", "function.hidden-mutation")
}

func TestFunctionResponsibilityAndOrchestrationRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.ts"), strings.Join([]string{
		"export async function checkoutHandler(request: Request, response: Response) {",
		"  validateOrder(request.body);",
		"  const order = await repository.fetchOrder(request.body.id);",
		"  if (order.total > 100) {",
		"    await cache.set(order.id, order);",
		"  }",
		"  const payload = transformOrder(order);",
		"  await repository.saveOrder(payload);",
		"  await notifier.send(payload);",
		"  metrics.log(payload);",
		"  return response.json(payload);",
		"}",
		"interface Request { body: any }",
		"interface Response { json(input: unknown): unknown }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRulePresent(t, report, "Code Quality", "function.multiple-responsibilities")
	assertFindingRulePresent(t, report, "Code Quality", "function.orchestration-domain-mix")
}

func TestFunctionReturnContractRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "returns.go"), strings.Join([]string{
		"package sample",
		"type User struct{}",
		"func LoadUser(id string, missing bool) (*User, error) {",
		"\tuser, err := fetchUser(id)",
		"\tif err != nil {",
		"\t\treturn user, err",
		"\t}",
		"\tif missing {",
		"\t\treturn nil, nil",
		"\t}",
		"\treturn user, nil",
		"}",
		"func fetchUser(id string) (*User, error) { return &User{}, nil }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRulePresent(t, report, "Code Quality", "function.inconsistent-return-contract")
	assertFindingRulePresent(t, report, "Code Quality", "function.partial-result")
}

func TestFunctionPrecisionSkipsExplicitSingleResponsibility(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "clean.ts"), strings.Join([]string{
		"export function saveUser(user: User): User {",
		"  repository.save(user);",
		"  return user;",
		"}",
		"export function isUserReady(user: User): boolean {",
		"  return user.enabled === true;",
		"}",
		"interface User { enabled: boolean }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
	assertFindingRuleAbsent(t, report, "Code Quality", "function.multiple-responsibilities")
	assertFindingRuleAbsent(t, report, "Code Quality", "function.inconsistent-return-contract")
}
