package checks_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestFunctionEffectsAllowLocalConstructionAndRepositoryHydration(t *testing.T) {
	cases := []struct{ name, language, file, source string }{
		{"go protobuf", "go", "mapper.go", `package sample
func MapUser(row Row) *User { out := &User{}; out.SetName(row.Name); return out }
type Row struct { Name string }; type User struct{}; func (*User) SetName(string) {}`},
		{"go local payload", "go", "payload.go", `package sample
func BuildPayload(name string) map[string]any { payload := make(map[string]any); payload["name"] = name; return payload }`},
		{"go constructor builder", "go", "buffer.go", `package sample
type Buffer struct{}; func NewBuffer() *Buffer { return &Buffer{} }; func (*Buffer) Write(string) {}
func Render() *Buffer { out := NewBuffer(); out.Write("ok"); return out }`},
		{"go sql mapper", "go", "repository.go", `package sample
func FindUser(rows Rows) (*User, error) { out := &User{}; if err := rows.Scan(&out.Name); err != nil { return nil, err }; return out, nil }
type User struct { Name string }; type Rows interface { Scan(...any) error }`},
		{"cpp local dto", "cpp", "mapper.cpp", `User BuildUser(const Row& row) { User out{}; out.set_name(row.name()); return out; }`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), tc.source)
			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, tc.language))
			assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
			assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
		})
	}
}

func TestFunctionEffectsDoNotPromoteUnknownFactoryResults(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "factory.go"), `package sample

type Item struct{ Field int }

var item *Item

func opaqueFactory() *Item { return nil }

func ReadValue() int {
	item := opaqueFactory()
	item.Field = 1
	return item.Field
}
`)

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))
	assertFindingRuleAbsent(t, report, "Code Quality", "function.hidden-mutation")
	assertFindingRuleAbsent(t, report, "Code Quality", "function.command-query-mix")
	assertUnresolvedDiagnosticCount(t, report, "go", "1")
}

func assertUnresolvedDiagnosticCount(t *testing.T, report codeguard.Report, language string, count string) {
	t.Helper()
	for _, section := range report.Sections {
		if section.Name != "Code Quality" {
			continue
		}
		for _, diagnostic := range section.Diagnostics {
			if diagnostic.ID == "quality.structural-unresolved-symbols" && diagnostic.Metadata["language"] == language && diagnostic.Metadata["count"] == count {
				return
			}
		}
	}
	t.Fatalf("expected unresolved-symbol diagnostic language=%q count=%q: %#v", language, count, report.Sections)
}

func TestFunctionEffectsReportOwnedAndObservableMutationEvidence(t *testing.T) {
	cases := []struct{ name, language, file, source, target, effect, origin string }{
		{"go argument alias", "go", "mutation.go", `package sample
type User struct { Name string }
func PrepareUser(user *User) *User { alias := user; alias.Name = "ready"; return user }`, "argument", "shared_state", "caller_owned"},
		{"go reassigned argument alias", "go", "reassignment.go", `package sample
type User struct { Name string }
func InspectUser(user *User) *User { var alias *User; alias = user; alias.Name = "seen"; return user }`, "argument", "shared_state", "caller_owned"},
		{"go receiver", "go", "receiver.go", `package sample
type Store struct { count int }
func (s *Store) Current() int { s.count++; return s.count }`, "receiver", "shared_state", "caller_owned"},
		{"cpp argument alias", "cpp", "mutation.cpp", `int Inspect(User& user) { User& alias = user; alias.score += 1; return alias.score; }`, "argument", "shared_state", "caller_owned"},
		{"go global", "go", "global.go", `package sample
var shared struct{ Count int }
func CurrentCount() int { shared.Count++; return shared.Count }`, "global", "shared_state", "shared"},
		{"go escaped local", "go", "escape.go", `package sample
type State struct{ Value int }; var sharedState *State
func CurrentState() *State { state := &State{}; sharedState = state; state.Value = 1; return state }`, "escaped", "shared_state", "shared"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), tc.source)
			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, tc.language))
			finding := findFinding(t, report, "Code Quality", "function.hidden-mutation")
			for key, want := range map[string]string{"mutation_target": tc.target, "effect_kind": tc.effect, "origin": tc.origin} {
				if got := finding.Metadata[key]; got != want {
					t.Fatalf("%s = %q, want %q; metadata=%v", key, got, want, finding.Metadata)
				}
			}
			if !strings.Contains(finding.Message, tc.target) {
				t.Fatalf("message lacks target evidence: %q", finding.Message)
			}
		})
	}
}

func TestCommandQueryMixUsesObservablePersistenceEvidence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "repository.go"), `package sample
type Repo interface { SaveAudit(string) error; Find(string) (User, error) }; type User struct{}
func GetUser(repo Repo, id string) (User, error) { if err := repo.SaveAudit(id); err != nil { return User{}, err }; return repo.Find(id) }`)
	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))
	finding := findFinding(t, report, "Code Quality", "function.command-query-mix")
	if finding.Metadata["effect_kind"] != "persistence" {
		t.Fatalf("metadata = %v", finding.Metadata)
	}
}

func TestCommandQueryMixUsesEventEvidence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "events.go"), `package sample
type Bus interface { Publish(string) error }
func GetStatus(bus Bus) (string, error) { if err := bus.Publish("read"); err != nil { return "", err }; return "ok", nil }`)
	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))
	finding := findFinding(t, report, "Code Quality", "function.command-query-mix")
	if finding.Metadata["effect_kind"] != "event" {
		t.Fatalf("metadata = %v", finding.Metadata)
	}
}

func TestOptionalReturnContractsAreDeliberate(t *testing.T) {
	cases := []string{
		`package sample
func FindUser(id string) (*User, error) { if id == "" { return nil, nil }; return &User{}, nil }; type User struct{}`,
		`package sample
func Lookup(id string) (User, bool, error) { if id == "" { return User{}, false, nil }; return User{}, true, nil }; type User struct{}`,
		`package sample
func Tags(ok bool) []string { if !ok { return nil }; return []string{} }`,
		`package sample
import "database/sql"
func Name(ok bool) sql.NullString { if !ok { return sql.NullString{} }; return sql.NullString{String: "Ada", Valid: true} }`,
	}
	for i, source := range cases {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "optional.go"), source)
		report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))
		assertFindingRuleAbsent(t, report, "Code Quality", "function.inconsistent-return-contract")
		_ = i
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "optional.cpp"), `std::optional<User> FindUser(bool found) { if (!found) return std::nullopt; return User{}; }`)
	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "cpp"))
	assertFindingRuleAbsent(t, report, "Code Quality", "function.inconsistent-return-contract")
}

func TestMessageChainsRequireIndependentCollaborators(t *testing.T) {
	cases := []struct {
		name, language, file, source string
	}{
		{"go protobuf accessors", "go", "mapper.go", `package sample
func Region(msg *Envelope) string { return msg.GetUser().GetProfile().GetAddress().GetRegion() }`},
		{"cpp fluent builder", "cpp", "builder.cpp", `Response Build() { ResponseBuilder builder; return builder.WithCode(200).WithBody("ok").WithHeader("x", "y").Build(); }`},
		{"typescript json traversal", "typescript", "parser.ts", `export function region(payload: any) { return payload.json.user.profile.address.region; }`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), tc.source)
			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, tc.language))
			assertFindingRuleAbsent(t, report, "Code Quality", "smell.message-chain")
		})
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "domain.go"), `package sample
func Country(order Order) string { return order.Customer().Account().Owner().Address().Country() }`)
	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))
	findFinding(t, report, "Code Quality", "smell.message-chain")
}

func TestMessageChainAllowsRepeatedCallsOnLocalFluentValue(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "buffer.go"), `package sample
func Render() string { buf := NewBuffer(); return buf.Append("a").Append("b").Append("c").Finish() }`)
	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))
	assertFindingRuleAbsent(t, report, "Code Quality", "smell.message-chain")
}
