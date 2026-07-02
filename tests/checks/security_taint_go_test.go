package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func securityOnlyConfig(name string, dir string, language string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "app", Path: dir, Language: language}}
	cfg.Checks.Security = true
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SecurityRules.GovulncheckMode = "off"
	return cfg
}

func taintMessages(t *testing.T, report codeguard.Report, ruleID string) []string {
	t.Helper()
	messages := make([]string, 0)
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == ruleID {
				messages = append(messages, finding.Message)
			}
		}
	}
	return messages
}

func assertChainMessage(t *testing.T, messages []string, wantParts ...string) {
	t.Helper()
	for _, message := range messages {
		matched := true
		for _, part := range wantParts {
			if !strings.Contains(message, part) {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	t.Fatalf("no taint message contains %v; got %v", wantParts, messages)
}

func TestGoTaintEnvToExecCommand(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), strings.Join([]string{
		"package main",
		"",
		"import (",
		"\t\"os\"",
		"\t\"os/exec\"",
		")",
		"",
		"func main() {",
		"\tuserCmd := os.Getenv(\"USER_CMD\")",
		"\talias := userCmd",
		"\t_ = exec.Command(\"sh\", \"-c\", alias)",
		"\t_ = os.Args",
		"}",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-go-exec", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "fail")
	messages := taintMessages(t, report, "security.taint.go")
	assertChainMessage(t, messages, "os.Getenv", "exec.Command", "userCmd -> alias")
	assertFindingConfidence(t, report, "Security", "security.taint.go", "high")
}

func TestGoTaintRequestToSQLViaSprintfAndHelper(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.go"), strings.Join([]string{
		"package web",
		"",
		"import (",
		"\t\"database/sql\"",
		"\t\"fmt\"",
		"\t\"net/http\"",
		")",
		"",
		"func userName(r *http.Request) string {",
		"\treturn r.FormValue(\"name\")",
		"}",
		"",
		"func handler(w http.ResponseWriter, r *http.Request, db *sql.DB) {",
		"\tname := userName(r)",
		"\tquery := fmt.Sprintf(\"SELECT * FROM users WHERE name = '%s'\", name)",
		"\trows, err := db.Query(query)",
		"\t_ = rows",
		"\t_ = err",
		"}",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-go-sql", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "fail")
	messages := taintMessages(t, report, "security.taint.go")
	assertChainMessage(t, messages, "r.FormValue", "db.Query", "userName()")
}

func TestGoTaintParamToSinkAcrossFunctions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "run.go"), strings.Join([]string{
		"package main",
		"",
		"import (",
		"\t\"os\"",
		"\t\"os/exec\"",
		")",
		"",
		"func runShell(command string) {",
		"\t_ = exec.Command(\"bash\", \"-c\", command)",
		"}",
		"",
		"func main() {",
		"\trunShell(os.Getenv(\"PAYLOAD\"))",
		"}",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-go-cross", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	messages := taintMessages(t, report, "security.taint.go")
	assertChainMessage(t, messages, "os.Getenv", "runShell()", "exec.Command")
}

func TestGoTaintStdinToOpenFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "files.go"), strings.Join([]string{
		"package main",
		"",
		"import (",
		"\t\"bufio\"",
		"\t\"os\"",
		")",
		"",
		"func main() {",
		"\treader := bufio.NewReader(os.Stdin)",
		"\tpath, _ := reader.ReadString('\\n')",
		"\tfile, err := os.OpenFile(path, os.O_RDONLY, 0)",
		"\t_ = file",
		"\t_ = err",
		"}",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-go-stdin", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	messages := taintMessages(t, report, "security.taint.go")
	assertChainMessage(t, messages, "stdin", "os.OpenFile", "path")
}

func TestGoTaintSanitizedAndParameterizedFlowsDoNotFlag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "safe.go"), strings.Join([]string{
		"package main",
		"",
		"import (",
		"\t\"database/sql\"",
		"\t\"fmt\"",
		"\t\"os\"",
		"\t\"os/exec\"",
		"\t\"strconv\"",
		")",
		"",
		"func main() {",
		"\tdb, _ := sql.Open(\"postgres\", \"dsn\")",
		"\tuserID := os.Getenv(\"USER_ID\")",
		"\trows, _ := db.Query(\"SELECT * FROM users WHERE id = $1\", userID)",
		"\t_ = rows",
		"\tcount, _ := strconv.Atoi(os.Getenv(\"COUNT\"))",
		"\t_ = exec.Command(\"echo\", fmt.Sprintf(\"%d\", count))",
		"\tstatic := \"uptime\"",
		"\t_ = exec.Command(\"sh\", \"-c\", static)",
		"}",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-go-safe", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if messages := taintMessages(t, report, "security.taint.go"); len(messages) != 0 {
		t.Fatalf("sanitized flows must not flag, got %v", messages)
	}
}

func TestGoTaintToggleDisables(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), strings.Join([]string{
		"package main",
		"",
		"import (",
		"\t\"os\"",
		"\t\"os/exec\"",
		")",
		"",
		"func main() {",
		"\t_ = exec.Command(\"sh\", \"-c\", os.Getenv(\"CMD\"))",
		"}",
		"",
	}, "\n"))

	cfg := securityOnlyConfig("taint-go-toggle", dir, "go")
	disabled := false
	cfg.Checks.SecurityRules.TaintGo = &disabled

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if messages := taintMessages(t, report, "security.taint.go"); len(messages) != 0 {
		t.Fatalf("taint_go=false must disable the rule, got %v", messages)
	}
}
