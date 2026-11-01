package checks_test

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

const pinnedCheckout = "actions/checkout@8f3c2b1a4d5e6f7890abc1234567890abc123456"

// workflowSecurityRules are the rules under test; the other ci.* rules are
// filtered out so each case can assert the exact finding set.
var workflowSecurityRules = map[string]bool{
	"ci.workflow-script-injection":      true,
	"ci.workflow-dangerous-trigger":     true,
	"ci.workflow-write-all-permissions": true,
	"ci.workflow-missing-permissions":   true,
	"ci.workflow-remote-script-pipe":    true,
	"ci.workflow-fork-trigger-secrets":  true,
}

type workflowFindingView struct {
	rule    string
	path    string
	line    int
	column  int
	message string
}

func runWorkflowSecurityScan(t *testing.T, files map[string]string) []workflowFindingView {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		writeFile(t, filepath.Join(dir, filepath.FromSlash(rel)), content)
	}
	cfg := ciSafetyTestConfig(dir, "ci-workflow-security")
	cfg.Checks.CIRules.RequiredWorkflowFiles = []string{}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	views := make([]workflowFindingView, 0)
	for _, section := range report.Sections {
		if section.Name != "CI/CD" {
			continue
		}
		for _, finding := range section.Findings {
			if workflowSecurityRules[finding.RuleID] {
				views = append(views, workflowFindingView{rule: finding.RuleID, path: finding.Path, line: finding.Line, column: finding.Column, message: finding.Message})
			}
		}
	}
	return views
}

func findingKeys(views []workflowFindingView) []string {
	keys := make([]string, 0, len(views))
	for _, view := range views {
		keys = append(keys, fmt.Sprintf("%s@%s:%d", view.rule, view.path, view.line))
	}
	sort.Strings(keys)
	return keys
}

func assertWorkflowFindings(t *testing.T, views []workflowFindingView, want []string) {
	t.Helper()
	sort.Strings(want)
	if got := findingKeys(views); !reflect.DeepEqual(got, want) {
		t.Fatalf("workflow findings mismatch\n got: %v\nwant: %v", got, want)
	}
}

func findingFor(t *testing.T, views []workflowFindingView, rule string, line int) workflowFindingView {
	t.Helper()
	for _, view := range views {
		if view.rule == rule && view.line == line {
			return view
		}
	}
	t.Fatalf("no %s finding on line %d in %v", rule, line, findingKeys(views))
	return workflowFindingView{}
}

// The workflow from the original gap report: every one of these attacks used
// to pass with only the unpinned checkout flagged.
func TestWorkflowSecurityFlagsPullRequestTargetAttackChain(t *testing.T) {
	workflow := `name: pr
on: pull_request_target
permissions: write-all
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@main
        with:
          ref: ${{ github.event.pull_request.head.sha }}
      - run: echo "${{ github.event.pull_request.title }}"
      - run: curl -fsSL https://example.com/install.sh | bash
      - run: ./deploy.sh
        env:
          TOKEN: ${{ secrets.DEPLOY_TOKEN }}
`
	views := runWorkflowSecurityScan(t, map[string]string{".github/workflows/pr.yml": workflow})
	path := ".github/workflows/pr.yml"
	assertWorkflowFindings(t, views, []string{
		"ci.workflow-write-all-permissions@" + path + ":3",
		"ci.workflow-dangerous-trigger@" + path + ":10",
		"ci.workflow-script-injection@" + path + ":11",
		"ci.workflow-remote-script-pipe@" + path + ":12",
		"ci.workflow-fork-trigger-secrets@" + path + ":15",
	})
	if msg := findingFor(t, views, "ci.workflow-fork-trigger-secrets", 15).message; !strings.Contains(msg, "secrets.DEPLOY_TOKEN") {
		t.Errorf("fork secrets message %q does not name the secret", msg)
	}
	if msg := findingFor(t, views, "ci.workflow-dangerous-trigger", 10).message; !strings.Contains(msg, "github.event.pull_request.head.sha") {
		t.Errorf("dangerous trigger message %q does not name the ref", msg)
	}
}

func TestWorkflowScriptInjectionInMultiLineBlocksAndWhitespaceVariants(t *testing.T) {
	workflow := `on: issue_comment
permissions:
  contents: read
jobs:
  triage:
    runs-on: ubuntu-latest
    steps:
      - name: echo
        run: |
          echo "start"
          echo "${{github.event.comment.body}}"
          echo "${{ github.event.issue.number }}"
          echo "${{   github.event.issue.title   }}"
          echo "${{ github.repository }} ${{ github.sha }}"
          echo "${{ github.event.comment.body }}"
      - uses: actions/github-script@8f3c2b1a4d5e6f7890abc1234567890abc123456
        with:
          script: |
            console.log("${{ github.event['issue']['body'] }}")
`
	path := ".github/workflows/triage.yml"
	views := runWorkflowSecurityScan(t, map[string]string{path: workflow})
	assertWorkflowFindings(t, views, []string{
		"ci.workflow-script-injection@" + path + ":11",
		"ci.workflow-script-injection@" + path + ":13",
		"ci.workflow-script-injection@" + path + ":15",
		"ci.workflow-script-injection@" + path + ":19",
	})
	if column := findingFor(t, views, "ci.workflow-script-injection", 11).column; column != 17 {
		t.Errorf("column = %d, want 17 (the ${{ on line 11)", column)
	}
	if msg := findingFor(t, views, "ci.workflow-script-injection", 19).message; !strings.Contains(msg, "github-script") || !strings.Contains(msg, "github.event.issue.body") {
		t.Errorf("github-script message %q does not name the sink and the context", msg)
	}
}

// True negatives: env indirection, the plain pull_request trigger (where
// checking out the head is the normal thing to do), github.sha,
// github.repository, numeric PR fields, and a read-only permissions block.
func TestWorkflowSecurityAllowsSafePatterns(t *testing.T) {
	workflow := `name: ci
on:
  pull_request:
    branches: [main]
permissions:
  contents: read
jobs:
  test:
    runs-on: ubuntu-latest
    if: contains(github.event.pull_request.title, 'release')
    steps:
      - uses: ` + pinnedCheckout + `
        with:
          ref: ${{ github.event.pull_request.head.sha }}
      - name: safe
        env:
          PR_TITLE: ${{ github.event.pull_request.title }}
          HEAD_REF: ${{ github.head_ref }}
        run: |
          echo "$PR_TITLE on $HEAD_REF"
          echo "${{ github.sha }} ${{ github.repository }} ${{ github.event.pull_request.number }}"
          echo "${{ inputs.name }} ${{ github.event.pull_request.title == 'x' }}"
          curl -fsSLo tool.tar.gz https://example.com/tool.tar.gz
      - run: ./notify.sh "${{ secrets.SLACK_TOKEN }}"
`
	views := runWorkflowSecurityScan(t, map[string]string{".github/workflows/ci.yml": workflow})
	assertWorkflowFindings(t, views, []string{})
}

func TestWorkflowSecurityAllowsBaseRefPullRequestTarget(t *testing.T) {
	workflow := `on: pull_request_target
permissions:
  pull-requests: write
jobs:
  label:
    runs-on: ubuntu-latest
    steps:
      - uses: ` + pinnedCheckout + `
      - run: gh pr edit "$NUMBER" --add-label triage
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          NUMBER: ${{ github.event.pull_request.number }}
`
	views := runWorkflowSecurityScan(t, map[string]string{".github/workflows/label.yml": workflow})
	assertWorkflowFindings(t, views, []string{})
}

func TestWorkflowPermissionsMissingAndJobWriteAll(t *testing.T) {
	workflow := `on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: make
  release:
    permissions: write-all
    runs-on: ubuntu-latest
    steps:
      - run: make release
`
	path := ".github/workflows/build.yml"
	views := runWorkflowSecurityScan(t, map[string]string{path: workflow})
	assertWorkflowFindings(t, views, []string{
		"ci.workflow-missing-permissions@" + path + ":1",
		"ci.workflow-write-all-permissions@" + path + ":8",
	})
	msg := findingFor(t, views, "ci.workflow-missing-permissions", 1).message
	if !strings.Contains(msg, `"build"`) || strings.Contains(msg, `"release"`) {
		t.Errorf("missing permissions message %q should list only the job without a block", msg)
	}
	if !strings.Contains(msg, "GITHUB_TOKEN") || !strings.Contains(msg, "default") {
		t.Errorf("missing permissions message %q should explain the default token", msg)
	}
}

func TestWorkflowPermissionsSkipsReusableOnlyWorkflowsAndSubdirectories(t *testing.T) {
	reusable := `on:
  workflow_call:
    inputs:
      tag:
        type: string
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: make
`
	nested := "on: push\njobs:\n  x:\n    runs-on: ubuntu-latest\n    steps:\n      - run: curl https://example.com | sh\n"
	views := runWorkflowSecurityScan(t, map[string]string{
		".github/workflows/reusable.yml":      reusable,
		".github/workflows/templates/x.yml":   nested,
		".github/workflows/broken.yaml":       "on: [push\njobs: {",
		".github/workflows/not-a-mapping.yml": "- just\n- a list\n",
	})
	assertWorkflowFindings(t, views, []string{})
}

func TestWorkflowRunCheckoutAndContinuedPipe(t *testing.T) {
	workflow := `on:
  workflow_run:
    workflows: [ci]
    types: [completed]
permissions:
  contents: read
jobs:
  report:
    runs-on: ubuntu-latest
    steps:
      - run: |
          gh pr checkout "$PR"
          curl -fsSL \
            https://example.com/x.sh | sh
`
	path := ".github/workflows/report.yml"
	views := runWorkflowSecurityScan(t, map[string]string{path: workflow})
	assertWorkflowFindings(t, views, []string{
		"ci.workflow-dangerous-trigger@" + path + ":12",
		"ci.workflow-remote-script-pipe@" + path + ":13",
	})
}

func TestWorkflowSecurityCompositeAction(t *testing.T) {
	action := `name: tool
inputs:
  version:
    description: version to install
runs:
  using: composite
  steps:
    - shell: bash
      run: go install example.com/tool@${{ inputs.version }}
    - shell: bash
      env:
        VERSION: ${{ inputs.version }}
      run: echo "$VERSION"
    - shell: bash
      run: bash <(curl -fsSL https://example.com/i.sh)
    - uses: actions/setup-go@v5
`
	views := runWorkflowSecurityScan(t, map[string]string{"action.yml": action})
	assertWorkflowFindings(t, views, []string{
		"ci.workflow-script-injection@action.yml:9",
		"ci.workflow-remote-script-pipe@action.yml:15",
	})
	if msg := findingFor(t, views, "ci.workflow-script-injection", 9).message; !strings.Contains(msg, "inputs.version") {
		t.Errorf("composite injection message %q does not name the input", msg)
	}
}

// The unpinned-action rule is reused for composite actions rather than
// duplicated as a workflow-* rule.
func TestMutableActionReferenceCoversCompositeActions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "action.yml"), "runs:\n  using: composite\n  steps:\n    - uses: actions/setup-go@v5\n")
	cfg := ciSafetyTestConfig(dir, "ci-composite-mutable")
	cfg.Checks.CIRules.RequiredWorkflowFiles = []string{}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if messages := ciRuleMessages(report, "ci.mutable-deployment-reference"); len(messages) != 1 {
		t.Fatalf("mutable reference findings = %v, want exactly one for actions/setup-go@v5", messages)
	}
}

func TestWorkflowSecurityIgnoresNonCompositeActionAndPlainInputs(t *testing.T) {
	action := "name: docker\nruns:\n  using: docker\n  image: Dockerfile\n  args:\n    - ${{ inputs.value }}\n"
	views := runWorkflowSecurityScan(t, map[string]string{
		"tools/action.yaml":              action,
		".github/workflows/dispatch.yml": "on: workflow_dispatch\npermissions: {}\njobs:\n  run:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo \"${{ inputs.name }} ${{ github.event.inputs.name }}\"\n",
	})
	assertWorkflowFindings(t, views, []string{})
}
