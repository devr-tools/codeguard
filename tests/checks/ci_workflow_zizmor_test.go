package checks_test

import (
	"strings"
	"testing"
)

// The fixtures below pin actions to a SHA with a version comment, the way
// pinning tools leave them, so the rules can read the major version.
const (
	pinSHA          = "8f3c2b1a4d5e6f7890abc1234567890abc123456"
	imageDigest     = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	workflowHeader  = "permissions:\n  contents: read\njobs:\n"
	releaseTagsOnly = "on:\n  push:\n    tags: [\"v*\"]\n"
)

func assertWorkflowFindingLevel(t *testing.T, views []workflowFindingView, rule string, line int, want string) {
	t.Helper()
	if got := findingFor(t, views, rule, line).level; got != want {
		t.Errorf("%s on line %d has level %q, want %q", rule, line, got, want)
	}
}

func TestWorkflowCachePoisoningOnReleaseTriggers(t *testing.T) {
	workflow := releaseTagsOnly + workflowHeader + `  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@` + pinSHA + ` # v5
        with:
          go-version-file: go.mod
      - uses: actions/setup-node@` + pinSHA + ` # v4
        with:
          cache: npm
      - uses: actions/cache@` + pinSHA + ` # v4
        with:
          path: ~/.cache
          key: deps
      - uses: actions/setup-python@` + pinSHA + ` # v5
      - uses: actions/setup-go@` + pinSHA + ` # v5
        with:
          cache: false
      - uses: actions/cache@` + pinSHA + ` # v4
        with:
          path: ~/.cache
          key: deps
          lookup-only: true
      - uses: actions/setup-node@` + pinSHA + ` # v4
        with:
          cache: ${{ startsWith(github.ref, 'refs/tags/') && '' || 'npm' }}
`
	path := ".github/workflows/release.yml"
	views := runWorkflowSecurityScan(t, map[string]string{path: workflow})
	assertWorkflowFindings(t, views, []string{
		"ci.workflow-cache-poisoning@" + path + ":10",
		"ci.workflow-cache-poisoning@" + path + ":13",
		"ci.workflow-cache-poisoning@" + path + ":16",
	})
	msg := findingFor(t, views, "ci.workflow-cache-poisoning", 10).message
	if !strings.Contains(msg, "runs only on tag pushes") || !strings.Contains(msg, "cache: false") {
		t.Errorf("cache poisoning message %q should name the trigger and the fix", msg)
	}
	assertWorkflowFindingLevel(t, views, "ci.workflow-cache-poisoning", 10, "warn")
}

// A publishing action makes a job a release job whatever the trigger; a
// plain CI job with caches, and a build-push that does not push, stay
// silent.
func TestWorkflowCachePoisoningPublisherJobsOnly(t *testing.T) {
	workflow := "on:\n  push:\n    branches: [main]\n  workflow_dispatch:\n" + workflowHeader + `  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-python@` + pinSHA + ` # v5
        with:
          cache: pip
      - uses: pypa/gh-action-pypi-publish@` + pinSHA + ` # v1
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@` + pinSHA + ` # v5
      - run: go test ./...
  image:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@` + pinSHA + ` # v5
      - uses: docker/build-push-action@` + pinSHA + ` # v6
        with:
          push: false
`
	// A CI workflow that also runs on tags (or on release-named branches)
	// tests rather than ships, so the trigger alone does not count.
	ciOnTags := "on:\n  push:\n    branches: [main, release/*]\n    tags: [\"v*\"]\n" + workflowHeader +
		"  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/setup-go@" + pinSHA + " # v5\n"
	path := ".github/workflows/publish.yml"
	views := runWorkflowSecurityScan(t, map[string]string{path: workflow, ".github/workflows/ci.yml": ciOnTags})
	assertWorkflowFindings(t, views, []string{"ci.workflow-cache-poisoning@" + path + ":11"})
	if msg := findingFor(t, views, "ci.workflow-cache-poisoning", 11).message; !strings.Contains(msg, "pypa/gh-action-pypi-publish") {
		t.Errorf("message %q should name the publishing action", msg)
	}
}

func TestWorkflowCredentialPersistenceOnlyWhenUploaded(t *testing.T) {
	workflow := "on: push\n" + workflowHeader + `  leak:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@` + pinSHA + ` # v4
      - uses: actions/upload-artifact@` + pinSHA + ` # v4
        with:
          name: src
          path: |
            ${{ github.workspace }}/
          include-hidden-files: true
  old-upload:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/upload-artifact@v3
        with:
          path: .git/config
  safe-flag:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@` + pinSHA + ` # v4
        with:
          persist-credentials: false
      - uses: actions/upload-artifact@` + pinSHA + ` # v4
        with:
          path: .
          include-hidden-files: true
  safe-v6:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@` + pinSHA + ` # v6.0.0
      - uses: actions/upload-artifact@` + pinSHA + ` # v4
        with:
          path: .
          include-hidden-files: true
  hidden-excluded:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@` + pinSHA + ` # v4
      - uses: actions/upload-artifact@` + pinSHA + ` # v4
        with:
          path: .
  build-output:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@` + pinSHA + ` # v4
      - uses: actions/upload-artifact@` + pinSHA + ` # v4
        with:
          path: dist/
          include-hidden-files: true
  pushes:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@` + pinSHA + ` # v4
      - run: git push origin HEAD
`
	path := ".github/workflows/bundle.yml"
	views := runWorkflowSecurityScan(t, map[string]string{path: workflow})
	assertWorkflowFindings(t, views, []string{
		"ci.workflow-credential-persistence@" + path + ":8",
		"ci.workflow-credential-persistence@" + path + ":18",
	})
	if msg := findingFor(t, views, "ci.workflow-credential-persistence", 8).message; !strings.Contains(msg, "line 9") || !strings.Contains(msg, "persist-credentials: false") {
		t.Errorf("message %q should name the upload line and the fix", msg)
	}
}

func TestWorkflowEnvFileInjection(t *testing.T) {
	workflow := `on:
  workflow_run:
    workflows: [ci]
    types: [completed]
env:
  WF_BRANCH: ${{ github.event.workflow_run.head_branch }}
` + workflowHeader + `  report:
    runs-on: ubuntu-latest
    env:
      TITLE: ${{ github.event.workflow_run.display_title }}
    steps:
      - run: echo "BRANCH=${{ github.event.workflow_run.head_branch }}" >> $GITHUB_ENV
      - env:
          MSG: ${{ github.event.workflow_run.head_commit.message }}
        run: |
          echo "start"
          echo "MSG=$MSG" >> "$GITHUB_ENV"
          echo "$WF_BRANCH" | tee -a "${GITHUB_PATH}"
          printf 'T=%s\n' "${TITLE}" >>$GITHUB_ENV
      - env:
          TITLE: static
          SHA: ${{ github.event.workflow_run.head_sha }}
        run: |
          echo "TITLE=$TITLE" >> "$GITHUB_ENV"
          echo "SHA=$SHA" >> "$GITHUB_ENV"
          echo "RUN=${{ github.run_id }}" >> "$GITHUB_ENV"
          echo "$MSG_UNSET" > "$GITHUB_OUTPUT"
`
	path := ".github/workflows/report.yml"
	views := runWorkflowSecurityScan(t, map[string]string{path: workflow})
	assertWorkflowFindings(t, views, []string{
		"ci.workflow-script-injection@" + path + ":15",
		"ci.workflow-env-file-injection@" + path + ":15",
		"ci.workflow-env-file-injection@" + path + ":20",
		"ci.workflow-env-file-injection@" + path + ":21",
		"ci.workflow-env-file-injection@" + path + ":22",
	})
	if msg := findingFor(t, views, "ci.workflow-env-file-injection", 20).message; !strings.Contains(msg, "$MSG (set from github.event.workflow_run.head_commit.message)") {
		t.Errorf("message %q should trace the variable to its context", msg)
	}
	if msg := findingFor(t, views, "ci.workflow-env-file-injection", 21).message; !strings.Contains(msg, "$GITHUB_PATH") {
		t.Errorf("message %q should name GITHUB_PATH", msg)
	}
}

// The same writes are not reported under an unprivileged trigger (the fork
// already runs its own code there) or in a composite action, whose inputs
// ci.workflow-script-injection already covers.
func TestWorkflowEnvFileInjectionNeedsPrivilegedTrigger(t *testing.T) {
	step := "      - run: echo \"T=${{ github.event.pull_request.title }}\" >> \"$GITHUB_ENV\"\n"
	action := "runs:\n  using: composite\n  steps:\n    - shell: bash\n      env:\n        V: ${{ inputs.v }}\n      run: echo \"V=$V\" >> \"$GITHUB_ENV\"\n"
	views := runWorkflowSecurityScan(t, map[string]string{
		".github/workflows/pr.yml":  "on: pull_request\n" + workflowHeader + "  x:\n    runs-on: ubuntu-latest\n    steps:\n" + step,
		".github/workflows/prt.yml": "on: pull_request_target\n" + workflowHeader + "  x:\n    runs-on: ubuntu-latest\n    steps:\n" + step,
		"action.yml":                action,
	})
	assertWorkflowFindings(t, views, []string{
		"ci.workflow-script-injection@.github/workflows/pr.yml:8",
		"ci.workflow-script-injection@.github/workflows/prt.yml:8",
		"ci.workflow-env-file-injection@.github/workflows/prt.yml:8",
	})
}

func TestWorkflowOverprovisionedSecrets(t *testing.T) {
	workflow := "on: push\n" + workflowHeader + `  deploy:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        target: [staging, prod]
    steps:
      - run: ./deploy.sh
        env:
          ALL: ${{ toJSON(secrets) }}
          PICKED: ${{ secrets[format('{0}_TOKEN', matrix.target)] }}
          NAMED: ${{ secrets.DEPLOY_TOKEN }}
          QUOTED: ${{ secrets['DEPLOY_TOKEN'] }}
          LITERAL: ${{ contains(github.event.head_commit.message, 'secrets') }}
          OTHER: ${{ env.secrets_file }}
      - uses: some/action@` + pinSHA + ` # v1
        with:
          secrets: ${{ toJson( secrets ) }}
`
	path := ".github/workflows/deploy.yml"
	views := runWorkflowSecurityScan(t, map[string]string{path: workflow})
	assertWorkflowFindings(t, views, []string{
		"ci.workflow-overprovisioned-secrets@" + path + ":13",
		"ci.workflow-overprovisioned-secrets@" + path + ":14",
		"ci.workflow-overprovisioned-secrets@" + path + ":21",
	})
	assertWorkflowFindingLevel(t, views, "ci.workflow-overprovisioned-secrets", 13, "fail")
	assertWorkflowFindingLevel(t, views, "ci.workflow-overprovisioned-secrets", 14, "warn")
}

func TestWorkflowUnpinnedImages(t *testing.T) {
	workflow := "on: push\n" + workflowHeader + `  short:
    runs-on: ubuntu-latest
    container: node:20
    services:
      db:
        image: postgres
      cache:
        image: redis@` + imageDigest + `
      registry:
        image: localhost:5000/team/tool:1.2
    steps:
      - uses: docker://alpine:3.20
      - uses: docker://ghcr.io/org/tool:latest
      - uses: docker://alpine@` + imageDigest + `
  long:
    runs-on: ubuntu-latest
    container:
      image: python:latest
    steps:
      - run: true
  dynamic:
    runs-on: ubuntu-latest
    container:
      image: ${{ matrix.image }}
    steps:
      - run: true
`
	path := ".github/workflows/images.yml"
	views := runWorkflowSecurityScan(t, map[string]string{path: workflow})
	// python:latest on an image: line (22) is ci.mutable-deployment-reference's,
	// as is docker://...:latest (17), so neither is reported twice.
	assertWorkflowFindings(t, views, []string{
		"ci.workflow-unpinned-image@" + path + ":7",
		"ci.workflow-unpinned-image@" + path + ":10",
		"ci.workflow-unpinned-image@" + path + ":14",
		"ci.workflow-unpinned-image@" + path + ":16",
	})
	assertWorkflowFindingLevel(t, views, "ci.workflow-unpinned-image", 7, "warn")
	assertWorkflowFindingLevel(t, views, "ci.workflow-unpinned-image", 10, "fail")
	assertWorkflowFindingLevel(t, views, "ci.workflow-unpinned-image", 14, "warn")
	if msg := findingFor(t, views, "ci.workflow-unpinned-image", 14).message; !strings.Contains(msg, `"1.2"`) || !strings.Contains(msg, "localhost:5000/team/tool@sha256") {
		t.Errorf("message %q should treat the registry port as part of the repository", msg)
	}
}

func TestWorkflowUnpinnedImageCompositeDockerStep(t *testing.T) {
	action := "runs:\n  using: composite\n  steps:\n    - uses: docker://alpine\n"
	views := runWorkflowSecurityScan(t, map[string]string{"action.yml": action})
	assertWorkflowFindings(t, views, []string{"ci.workflow-unpinned-image@action.yml:4"})
	assertWorkflowFindingLevel(t, views, "ci.workflow-unpinned-image", 4, "fail")
}

func TestWorkflowInsecureCommands(t *testing.T) {
	workflow := `on: push
env:
  ACTIONS_ALLOW_UNSECURE_COMMANDS: true
` + workflowHeader + `  build:
    runs-on: ubuntu-latest
    env:
      ACTIONS_ALLOW_UNSECURE_COMMANDS: "false"
    steps:
      - run: echo hi
        env:
          ACTIONS_ALLOW_UNSECURE_COMMANDS: ${{ inputs.legacy }}
      - run: echo hi
        env:
          OTHER: true
      - run: echo hi
        env:
          ACTIONS_ALLOW_UNSECURE_COMMANDS: yes
`
	path := ".github/workflows/legacy.yml"
	views := runWorkflowSecurityScan(t, map[string]string{path: workflow})
	assertWorkflowFindings(t, views, []string{
		"ci.workflow-insecure-commands@" + path + ":3",
		"ci.workflow-insecure-commands@" + path + ":14",
	})
	assertWorkflowFindingLevel(t, views, "ci.workflow-insecure-commands", 3, "fail")
	assertWorkflowFindingLevel(t, views, "ci.workflow-insecure-commands", 14, "warn")
}

func TestWorkflowBotConditions(t *testing.T) {
	jobs := `  automerge:
    runs-on: ubuntu-latest
    if: ${{ github.actor == 'dependabot[bot]' }}
    steps:
      - if: "'renovate[bot]' == github.triggering_actor"
        run: echo merge
      - if: github.event.pull_request.user.login == 'dependabot[bot]' && github.actor == 'dependabot[bot]'
        run: echo safe
      - if: github.actor != 'dependabot[bot]'
        run: echo not-a-gate
      - if: github['ACTOR_ID'] == 49699333
        run: echo by-id
      - if: github.actor == 'octocat'
        run: echo not-a-bot
`
	views := runWorkflowSecurityScan(t, map[string]string{
		".github/workflows/prt.yml": "on: pull_request_target\n" + workflowHeader + jobs,
		".github/workflows/pr.yml":  "on: pull_request\n" + workflowHeader + jobs,
	})
	assertWorkflowFindings(t, views, []string{
		"ci.workflow-bot-conditions@.github/workflows/prt.yml:7",
		"ci.workflow-bot-conditions@.github/workflows/prt.yml:9",
		"ci.workflow-bot-conditions@.github/workflows/prt.yml:15",
	})
}

// zizmor's github-path repro: the title reaches $GITHUB_PATH through a shell
// variable assigned in the script. Reassigning it from trusted data clears it.
func TestWorkflowEnvFileInjectionThroughScriptAssignments(t *testing.T) {
	workflow := "on: pull_request_target\n" + workflowHeader + `  x:
    runs-on: ubuntu-latest
    steps:
      - env:
          TITLE: ${{ github.event.pull_request.title }}
        run: |
          message=$(echo "$TITLE" | grep -oP '[a-z]+')
          echo "$message" >> $GITHUB_PATH
          message="fixed"
          echo "$message" >> $GITHUB_PATH
          printf "BUNDLE_WITH=avo\nRAILS_GROUPS=avo\n" >> "$GITHUB_ENV"
`
	path := ".github/workflows/path.yml"
	views := runWorkflowSecurityScan(t, map[string]string{path: workflow})
	assertWorkflowFindings(t, views, []string{"ci.workflow-env-file-injection@" + path + ":12"})
	if msg := findingFor(t, views, "ci.workflow-env-file-injection", 12).message; !strings.Contains(msg, "$message (set from github.event.pull_request.title)") {
		t.Errorf("message %q should trace the assignment back to the title", msg)
	}
}
