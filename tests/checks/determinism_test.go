package checks_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// determinismRunsPerSetting is how many scans run at each GOMAXPROCS
// setting. Go's map iteration is biased for small maps, so the fixtures use
// many map entries and the tests also pin the deterministic choice.
const determinismRunsPerSetting = 4

// assertDeterministicFindings runs scan repeatedly at GOMAXPROCS=1 and at a
// high GOMAXPROCS and fails unless every run yields byte-identical findings.
// It returns the findings of the first run.
func assertDeterministicFindings(t *testing.T, scan func() codeguard.Report) []codeguard.Finding {
	t.Helper()
	previous := runtime.GOMAXPROCS(0)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })

	var first []codeguard.Finding
	var firstJSON []byte
	for _, procs := range []int{1, 32} {
		runtime.GOMAXPROCS(procs)
		for run := 0; run < determinismRunsPerSetting; run++ {
			findings := make([]codeguard.Finding, 0)
			for _, section := range scan().Sections {
				findings = append(findings, section.Findings...)
			}
			encoded, err := json.Marshal(findings)
			if err != nil {
				t.Fatalf("marshal findings: %v", err)
			}
			if firstJSON == nil {
				first, firstJSON = findings, encoded
				continue
			}
			if string(encoded) != string(firstJSON) {
				t.Fatalf("GOMAXPROCS=%d run %d produced different findings:\nfirst: %s\nthis:  %s", procs, run, firstJSON, encoded)
			}
		}
	}
	return first
}

func messagesFor(findings []codeguard.Finding, ruleID string) []string {
	out := make([]string, 0)
	for _, finding := range findings {
		if finding.RuleID == ruleID {
			out = append(out, finding.Message)
		}
	}
	return out
}

func requireMessages(t *testing.T, findings []codeguard.Finding, ruleID string, want ...string) {
	t.Helper()
	got := messagesFor(findings, ruleID)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("%s messages:\n got  %q\n want %q", ruleID, got, want)
	}
}

var determinismTypeNames = []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Golf", "Hotel", "India", "Juliet"}

func writeDeterminismGoFixture(t *testing.T, dir string) {
	t.Helper()
	// Go taint: many functions, and several callers feeding one helper sink
	// whose findings share a dedup key; the first-declared caller must win.
	var taint strings.Builder
	taint.WriteString("package app\n\nimport (\n\t\"os\"\n\t\"os/exec\"\n)\n\n")
	taint.WriteString("func runHelper(cmd string) {\n\t_ = exec.Command(\"sh\", \"-c\", cmd).Run()\n}\n")
	for _, name := range determinismTypeNames {
		fmt.Fprintf(&taint, "\nfunc caller%s() {\n\trunHelper(os.Getenv(%q))\n}\n", name, strings.ToUpper(name)+"_CMD")
	}
	writeFile(t, filepath.Join(dir, "taint.go"), taint.String())

	// Design max-methods and god objects: ten over-limit types.
	var types strings.Builder
	types.WriteString("package app\n")
	for _, name := range determinismTypeNames {
		fmt.Fprintf(&types, "\ntype %s struct {\n\ta, b, c, d, e int\n}\n", name)
		for i := 1; i <= 10; i++ {
			fmt.Fprintf(&types, "\nfunc (x *%s) Method%d() int {\n\treturn %d\n}\n", name, i, i)
		}
	}
	writeFile(t, filepath.Join(dir, "types.go"), types.String())

	// Middle man: four delegate targets tied at three methods each.
	var relay strings.Builder
	relay.WriteString("package app\n\ntype Store interface {\n\tGet(key string) int\n}\n\ntype Relay struct {\n")
	targets := []string{"north", "east", "south", "west"}
	for _, target := range targets {
		fmt.Fprintf(&relay, "\t%s Store\n", target)
	}
	relay.WriteString("}\n")
	for index, target := range targets {
		for i := 0; i < 3; i++ {
			fmt.Fprintf(&relay, "\nfunc (r *Relay) Get%d() int {\n\treturn r.%s.Get(%q)\n}\n", index*3+i, target, target)
		}
	}
	writeFile(t, filepath.Join(dir, "relay.go"), relay.String())

	// Glossary drift: several drifting concepts in one file; one is reported.
	writeFile(t, filepath.Join(dir, "vocab.go"), strings.Join([]string{
		"package app",
		"type RestaurantResponse struct{}",
		"type VenueEntity struct{}",
		"type OrderRecord struct{}",
		"type PurchaseRow struct{}",
		"type CustomerView struct{}",
		"type ClientModel struct{}",
		"type InvoiceData struct{}",
		"type BillingInfo struct{}",
		"type ShipmentNote struct{}",
		"type ParcelItem struct{}",
		"",
	}, "\n"))

	// Test idioms: a three-way framework tie.
	writeFile(t, filepath.Join(dir, "alpha_test.go"), "package app_test\n\nimport (\n\t. \"github.com/onsi/ginkgo/v2\"\n)\n\nvar _ = Describe(\"alpha\", func() {\n\tIt(\"works\", func() {})\n})\n")
	writeFile(t, filepath.Join(dir, "bravo_test.go"), "package app_test\n\nimport \"testing\"\n\nfunc TestBravo(t *testing.T) {\n\tt.Log(\"bravo\")\n}\n")
	writeFile(t, filepath.Join(dir, "charlie_test.go"), "package app_test\n\nimport (\n\t\"github.com/stretchr/testify/suite\"\n)\n\ntype CharlieSuite struct {\n\tsuite.Suite\n}\n")
}

func writeDeterminismRustFixture(t *testing.T, dir string) {
	t.Helper()
	var rust strings.Builder
	for _, name := range determinismTypeNames {
		fmt.Fprintf(&rust, "pub struct %s;\n\nimpl %s {\n", name, name)
		for i := 1; i <= 9; i++ {
			fmt.Fprintf(&rust, "    pub fn method%d(&self) -> i32 {\n        %d\n    }\n", i, i)
		}
		rust.WriteString("}\n\n")
	}
	writeFile(t, filepath.Join(dir, "lib.rs"), rust.String())
}

func writeDeterminismTypeScriptFixture(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "shell.ts"), strings.Join([]string{
		`import { exec, execSync } from "child_process";`,
		`import { runInNewContext, runInThisContext, compileFunction } from "vm";`,
		``,
		`export function runA(cmd: string) {`,
		`  execSync(cmd);`,
		`}`,
		``,
		`export function runB(cmd: string) {`,
		`  exec(cmd);`,
		`}`,
		``,
		`export function evalA(code: string) {`,
		`  runInThisContext(code);`,
		`}`,
		``,
		`export function evalB(code: string) {`,
		`  runInNewContext(code);`,
		`}`,
		``,
		`export function evalC(code: string) {`,
		`  compileFunction(code);`,
		`}`,
		``,
	}, "\n"))
}

// Full-scan paths fixed for map-order nondeterminism: Go taint function
// order, design max-methods (Go and Rust), dominant-count ties, glossary
// concept choice, Go structural class order and TypeScript alias loops.
func TestFullScanFindingsAreDeterministicAcrossRunsAndGOMAXPROCS(t *testing.T) {
	libDir := t.TempDir()
	invalidLib := filepath.Join(libDir, "typescript.js")
	if err := os.WriteFile(invalidLib, []byte("invalid"), 0o600); err != nil {
		t.Fatalf("write invalid lib: %v", err)
	}
	t.Setenv("CODEGUARD_TYPESCRIPT_LIB_PATH", invalidLib)

	root := t.TempDir()
	goDir, rustDir, tsDir := filepath.Join(root, "go"), filepath.Join(root, "rust"), filepath.Join(root, "ts")
	writeDeterminismGoFixture(t, goDir)
	writeDeterminismRustFixture(t, rustDir)
	writeDeterminismTypeScriptFixture(t, tsDir)

	cfg := codeguard.ExampleConfig()
	cfg.Name = "determinism"
	cfg.Targets = []codeguard.TargetConfig{
		{Name: "go", Path: goDir, Language: "go"},
		{Name: "rust", Path: rustDir, Language: "rust"},
		{Name: "ts", Path: tsDir, Language: "typescript"},
	}
	cfg.Checks.Quality = true
	cfg.Checks.Design = true
	cfg.Checks.Security = true
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SecurityRules.GovulncheckMode = "off"
	off := false
	cfg.Checks.Context = &off
	cfg.Cache.Enabled = &off
	cfg.Checks.QualityRules.Naming.Glossary = map[string]codeguard.QualityNamingGlossaryEntry{
		"restaurant": {Avoid: []string{"venue"}},
		"order":      {Avoid: []string{"purchase"}},
		"customer":   {Avoid: []string{"client"}},
		"invoice":    {Avoid: []string{"billing"}},
		"shipment":   {Avoid: []string{"parcel"}},
	}

	findings := assertDeterministicFindings(t, func() codeguard.Report {
		report, err := codeguard.Run(context.Background(), cfg)
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return report
	})

	// Go taint: the helper sink is reported once, for the first caller.
	helper := make([]string, 0)
	for _, message := range messagesFor(findings, "security.taint.go") {
		if strings.Contains(message, "runHelper()") {
			helper = append(helper, message)
		}
	}
	if len(helper) != 1 || !strings.Contains(helper[0], "from os.Getenv (line 13)") {
		t.Fatalf("expected one helper flow from callerAlpha (line 13), got %q", helper)
	}

	// Files are scanned in path order (relay.go before types.go), types in
	// name order within a file.
	wantMethods := []string{"type Relay has 12 methods; max is 8"}
	wantRust := make([]string, 0, len(determinismTypeNames))
	for _, name := range determinismTypeNames {
		wantMethods = append(wantMethods, fmt.Sprintf("type %s has 10 methods; max is 8", name))
		wantRust = append(wantRust, fmt.Sprintf("type %s has 9 impl methods in this file; max is 8", name))
	}
	requireMessages(t, findings, "design.max-methods-per-type", wantMethods...)
	requireMessages(t, findings, "design.rust.max-methods-per-type", wantRust...)

	gods := messagesFor(findings, "smell.god-object")
	godNames := append([]string{"Relay"}, determinismTypeNames...)
	if len(gods) != len(godNames) {
		t.Fatalf("expected %d god objects, got %q", len(godNames), gods)
	}
	for index, name := range godNames {
		if !strings.HasPrefix(gods[index], "type "+name+" ") {
			t.Fatalf("god objects not in source order: %q", gods)
		}
	}

	requireMessages(t, findings, "smell.middle-man", "type Relay forwards 12 of 12 methods, mostly to east, without visible policy or translation")
	requireMessages(t, findings, "naming.domain-vocabulary-drift", `domain concept "customer" appears under multiple terms (client, customer); prefer one glossary vocabulary`)
	requireMessages(t, findings, "quality.ai.local-idiom-drift",
		"test uses testing while the repository primarily uses ginkgo",
		"test uses testify-suite while the repository primarily uses ginkgo")

	if got := messagesFor(findings, "security.typescript.shell-execution"); len(got) != 2 {
		t.Fatalf("expected two TypeScript shell-execution findings, got %q", got)
	}
	if got := messagesFor(findings, "security.typescript.vm-dynamic-code"); len(got) != 3 {
		t.Fatalf("expected three TypeScript vm-dynamic-code findings, got %q", got)
	}
}

// refactor.duplicate-implementation-left-behind reports one duplicate group;
// which group must not depend on map order.
func TestDiffDuplicateImplementationChoiceIsDeterministic(t *testing.T) {
	dir := initChangeRepo(t)
	body := "package service\n\nfunc %s(value int) int {\n\ttotal := value\n\tif total > %d {\n\t\ttotal = total - %d\n\t}\n\tif total < 0 {\n\t\ttotal = 0\n\t}\n\treturn total\n}\n"
	names := []string{"total", "fee", "tax", "tip", "fine", "rent"}
	for index, name := range names {
		writeFile(t, filepath.Join(dir, "service", "old_"+name+".go"), fmt.Sprintf(body, "Old"+name, 100+index*50, 10+index))
	}
	commitAll(t, dir, "base")
	added := make([]string, 0, len(names))
	for index, name := range names {
		rel := "service/new_" + name + ".go"
		writeFile(t, filepath.Join(dir, filepath.FromSlash(rel)), fmt.Sprintf(body, "New"+name, 100+index*50, 10+index))
		added = append(added, rel)
	}
	runGit(t, dir, append([]string{"add", "-N"}, added...)...)

	cfg := refactorTestConfig(t, dir, "go")
	findings := assertDeterministicFindings(t, func() codeguard.Report {
		return runChangeDiff(t, cfg)
	})
	requireMessages(t, findings, "refactor.duplicate-implementation-left-behind",
		"Refactor leaves duplicate implementation bodies active at service/new_fee.go:3, service/old_fee.go:3.")
}
