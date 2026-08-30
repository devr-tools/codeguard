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

func TestFunctionHiddenMutationWarnsForObjectAssignIntoParameter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mutation.ts"), strings.Join([]string{
		"export function prepareUser(user: User, patch: Partial<User>): User {",
		"  Object.assign(user, patch);",
		"  return user;",
		"}",
		"interface User { name: string }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRulePresent(t, report, "Code Quality", "function.hidden-mutation")
	assertFindingRulePresent(t, report, "Code Quality", "function.command-query-mix")
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

func TestFunctionHiddenMutationReportsUnresolvedPersistenceInsideReactHook(t *testing.T) {
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

	assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
	assertUnresolvedDiagnosticCount(t, report, "typescript", "1")
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

func TestFunctionReturnContractAllowsStandardGoRepositoryResults(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "services", "postgres", "collections_repository.go"), strings.Join([]string{
		"package postgres",
		"",
		"import \"context\"",
		"",
		"type Collection struct{ ID string }",
		"type CollectionItem struct{ ID string }",
		"type Row interface{ Scan(...any) error }",
		"type Rows interface{ Next() bool; Scan(...any) error; Err() error; Close() error }",
		"type Querier interface{ QueryRow(context.Context, string, ...any) Row; Query(context.Context, string, ...any) (Rows, error); Exec(context.Context, string, ...any) (CommandTag, error) }",
		"type CommandTag interface{ RowsAffected() int64 }",
		"type CollectionsRepository struct{ db Querier }",
		"",
		"func (r *CollectionsRepository) FindCollection(ctx context.Context, id string) (*Collection, error) {",
		"\trow := r.db.QueryRow(ctx, \"select id from collections where id=$1\", id)",
		"\tcollection := &Collection{}",
		"\tif err := row.Scan(&collection.ID); err != nil {",
		"\t\treturn nil, err",
		"\t}",
		"\treturn collection, nil",
		"}",
		"",
		"func (r *CollectionsRepository) ListCollectionItems(ctx context.Context, id string) ([]*CollectionItem, error) {",
		"\trows, err := r.db.Query(ctx, \"select id from collection_items where collection_id=$1\", id)",
		"\tif err != nil {",
		"\t\treturn nil, err",
		"\t}",
		"\tdefer rows.Close()",
		"\titems := []*CollectionItem{}",
		"\tfor rows.Next() {",
		"\t\titem := &CollectionItem{}",
		"\t\tif err := rows.Scan(&item.ID); err != nil {",
		"\t\t\treturn nil, err",
		"\t\t}",
		"\t\titems = append(items, item)",
		"\t}",
		"\treturn items, rows.Err()",
		"}",
		"",
		"func (r *CollectionsRepository) CollectionExists(ctx context.Context, id string) (bool, error) {",
		"\trow := r.db.QueryRow(ctx, \"select exists(select 1 from collections where id=$1)\", id)",
		"\tvar exists bool",
		"\tif err := row.Scan(&exists); err != nil {",
		"\t\treturn false, err",
		"\t}",
		"\treturn exists, nil",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", "function.inconsistent-return-contract")
	assertFindingRuleAbsent(t, report, "Code Quality", "function.partial-result")
}

func TestBooleanPredicateNamingOnlyRunsOnBooleanReturns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "values.go"), strings.Join([]string{
		"package sample",
		"",
		"type Field struct{}",
		"type Row struct{}",
		"type Timing struct{}",
		"type Context struct{}",
		"type Collection struct{}",
		"",
		"func Get(limit int) *Collection {",
		"\treturn &Collection{}",
		"}",
		"",
		"func RawStringField(row Row, fallback string) string {",
		"\treturn \"value\"",
		"}",
		"",
		"func ValueOrNotFound(value string, fallback string) (string, error) {",
		"\treturn value, nil",
		"}",
		"",
		"func CollectOneRowOrNilAndClose(rows []Row, limit int) (*Row, error) {",
		"\treturn nil, nil",
		"}",
		"",
		"func TimingFromContext(ctx Context, fallback Timing) Timing {",
		"\treturn Timing{}",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", "naming.boolean-not-predicate")
}

func TestBooleanPredicateNamingAcceptsConventionalPredicateVocabulary(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "predicates.go"), strings.Join([]string{
		"package sample",
		"",
		"func ContainsNeedle(haystack string, needle string) bool { return true }",
		"func MatchesPattern(value string) bool { return true }",
		"func AllowsAccess(userID string) bool { return true }",
		"func ExistsInCache(key string) bool { return true }",
		"func SupportedFormat(format string) bool { return true }",
		"func ChangedSince(version int) bool { return true }",
		"func ReadableBy(userID string) bool { return true }",
		"func CompatibleWith(version string) bool { return true }",
		"func CompleteEnough(score int) bool { return true }",
		"func ForbiddenFor(role string) bool { return true }",
		"func IncludedInPlan(plan string) bool { return true }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", "naming.boolean-not-predicate")
}

func TestImperativeBooleanNamesAreAllowedOnlyForParameters(t *testing.T) {
	for _, tc := range []struct {
		name     string
		language string
		file     string
		content  string
	}{
		{name: "go", language: "go", file: "options.go", content: strings.Join([]string{
			"package sample",
			"func Load(includeInactive, requireCompleteMetadata, allowPartial, skipCache, enableTrace, disableRetry, forceRefresh, useIndex bool) {}",
		}, "\n")},
		{name: "cpp", language: "cpp", file: "options.cpp", content: "void load(bool includeInactive, bool requireCompleteMetadata, bool allowPartial, bool skipCache, bool forceRefresh, bool useIndex) {}\n"},
	} {
		t.Run(tc.name+" parameters", func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), tc.content)
			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, tc.language))
			assertFindingRuleAbsent(t, report, "Code Quality", "naming.boolean-not-predicate")
		})
	}

	t.Run("go non-imperative parameter", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "state.go"), "package sample\nfunc Load(metadata bool) {}\n")
		report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))
		assertFindingRulePresent(t, report, "Code Quality", "naming.boolean-not-predicate")
	})

	t.Run("cpp non-imperative parameter", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "state.cpp"), "void load(bool metadata) {}\n")
		report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "cpp"))
		assertFindingRulePresent(t, report, "Code Quality", "naming.boolean-not-predicate")
	})
}

func TestGoLocalRowAssignmentsDoNotLookLikeMutableGlobals(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "collections.go"), strings.Join([]string{
		"package sample",
		"",
		"type CollectionItem struct{ ID string }",
		"",
		"func BuildItems(ids []string) []CollectionItem {",
		"\titems := make([]CollectionItem, 0, len(ids))",
		"\tvar row CollectionItem",
		"\tfor _, id := range ids {",
		"\t\trow = CollectionItem{ID: id}",
		"\t\titems = append(items, row)",
		"\t}",
		"\treturn items",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.mutable-global-state")
}

func TestGoMutableGlobalExemptsImmutableConstructorUntilReassigned(t *testing.T) {
	t.Run("immutable compiled regex", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "patterns.go"), strings.Join([]string{
			"package sample",
			"import \"regexp\"",
			"var emailPattern = regexp.MustCompile(`@`)",
			"func Matches(value string) bool { return emailPattern.MatchString(value) }",
		}, "\n"))

		report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))
		assertFindingRuleAbsent(t, report, "Code Quality", "quality.mutable-global-state")
	})

	t.Run("reassigned compiled regex", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "patterns.go"), strings.Join([]string{
			"package sample",
			"import \"regexp\"",
			"var emailPattern = regexp.MustCompile(`@`)",
			"func Configure(pattern string) { emailPattern = regexp.MustCompile(pattern) }",
		}, "\n"))

		report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))
		assertFindingRulePresent(t, report, "Code Quality", "quality.mutable-global-state")
	})

	t.Run("mutable collection", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "cache.go"), "package sample\nvar cache = map[string]string{}\n")

		report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))
		assertFindingRulePresent(t, report, "Code Quality", "quality.mutable-global-state")
	})
}

func TestPostgresRepositoryAllowsPgxQueriesAndRowsAffectedResults(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "platform", "storage", "postgres", "collections_repository.go"), strings.Join([]string{
		"package postgres",
		"",
		"import \"context\"",
		"",
		"type Collection struct{ ID string }",
		"type Row interface{ Scan(...any) error }",
		"type CommandTag interface{ RowsAffected() int64 }",
		"type PgxPool interface{ QueryRow(context.Context, string, ...any) Row; Exec(context.Context, string, ...any) (CommandTag, error) }",
		"type CollectionsRepository struct{ pool PgxPool }",
		"",
		"func (r *CollectionsRepository) FindCollection(ctx context.Context, id string) (*Collection, error) {",
		"\tif err := validateCollectionID(id); err != nil {",
		"\t\treturn nil, err",
		"\t}",
		"\trow := r.pool.QueryRow(ctx, \"select id from collections where id=$1\", id)",
		"\treturn scanCollection(row)",
		"}",
		"",
		"func (r *CollectionsRepository) LikeCollection(ctx context.Context, userID string, collectionID string) (bool, error) {",
		"\tif err := validateCollectionID(collectionID); err != nil {",
		"\t\treturn false, err",
		"\t}",
		"\ttag, err := r.pool.Exec(ctx, \"insert into collection_likes(user_id, collection_id) values($1, $2) on conflict do nothing\", userID, collectionID)",
		"\tif err != nil {",
		"\t\treturn false, err",
		"\t}",
		"\treturn tag.RowsAffected() > 0, nil",
		"}",
		"",
		"func (r *CollectionsRepository) UnlikeCollection(ctx context.Context, userID string, collectionID string) (bool, error) {",
		"\ttag, err := r.pool.Exec(ctx, \"delete from collection_likes where user_id=$1 and collection_id=$2\", userID, collectionID)",
		"\tif err != nil {",
		"\t\treturn false, err",
		"\t}",
		"\treturn tag.RowsAffected() > 0, nil",
		"}",
		"",
		"func validateCollectionID(string) error { return nil }",
		"func scanCollection(Row) (*Collection, error) { return &Collection{}, nil }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", "function.mixed-abstraction-level")
	assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
	assertFindingRuleAbsent(t, report, "Code Quality", "function.partial-result")
}

func TestPostgresStorageAdapterAllowsPgxQueriesOutsideRepositoryFilename(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "platform", "storage", "postgres", "collection_store.go"), strings.Join([]string{
		"package postgres",
		"",
		"import \"context\"",
		"",
		"type Collection struct{ ID string }",
		"type Row interface{ Scan(...any) error }",
		"type PgxPool interface{ QueryRow(context.Context, string, ...any) Row }",
		"type Store struct{ pool PgxPool }",
		"",
		"func (s *Store) LoadCollection(ctx context.Context, id string) (*Collection, error) {",
		"\tif err := validateCollectionID(id); err != nil {",
		"\t\treturn nil, err",
		"\t}",
		"\trow := s.pool.QueryRow(ctx, \"select id from collections where id=$1\", id)",
		"\treturn scanCollection(row)",
		"}",
		"",
		"func validateCollectionID(string) error { return nil }",
		"func scanCollection(Row) (*Collection, error) { return &Collection{}, nil }",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", "function.mixed-abstraction-level")
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
